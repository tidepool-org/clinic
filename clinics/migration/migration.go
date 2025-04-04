package migration

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/mongo"
	"time"

	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	internalErrs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/fx"
)

const (
	StatusPending = "PENDING"
)

var ErrAlreadyMigrated = fmt.Errorf("%w: clinic is already migrated", internalErrs.ConstraintViolation)
var ErrIncompleteClinicProfile = fmt.Errorf("%w: incomplete clinic profile", internalErrs.ConstraintViolation)

type Migration struct {
	ClinicId    *primitive.ObjectID `json:"clinicId" bson:"clinicId"`
	UserId      string              `json:"userId" bson:"userId"`
	Status      string              `json:"status" bson:"status"`
	CreatedTime time.Time           `json:"createdTime" bson:"createdTime"`
	UpdatedTime time.Time           `json:"updatedTime" bson:"updatedTime"`
}

func NewMigration(clinicId, userId string) *Migration {
	clinicObjId, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		panic(err)
	}

	return &Migration{
		UserId:      userId,
		ClinicId:    &clinicObjId,
		Status:      StatusPending,
		CreatedTime: time.Now(),
		UpdatedTime: time.Now(),
	}
}

type Migrator interface {
	CreateEmptyClinic(ctx context.Context, userId string) (*clinics.Clinic, error)
	GetMigration(ctx context.Context, clinicId string, userId string) (*Migration, error)
	ListMigrations(ctx context.Context, clinicId string) ([]*Migration, error)
	MigrateLegacyClinicianPatients(ctx context.Context, clinicId, userId string) (*Migration, error)
	TriggerInitialMigration(ctx context.Context, clinicId string) (*Migration, error)
	UpdateMigrationStatus(ctx context.Context, clinicId, userId, status string) (*Migration, error)
}

type Params struct {
	fx.In

	ClinicsCreator    manager.Manager
	ClinicsService    clinics.Service
	CliniciansService clinicians.Service
	MigrationRepo     Repository
	UserService       patients.UserService

	DBClient *mongo.Client
}

type migrator struct {
	dbClient *mongo.Client

	clinicsCreator    manager.Manager
	clinicsService    clinics.Service
	cliniciansService clinicians.Service
	migrationRepo     Repository
	userService       patients.UserService
}

func NewMigrator(p Params) (Migrator, error) {
	return &migrator{
		dbClient:          p.DBClient,
		clinicsCreator:    p.ClinicsCreator,
		clinicsService:    p.ClinicsService,
		cliniciansService: p.CliniciansService,
		migrationRepo:     p.MigrationRepo,
		userService:       p.UserService,
	}, nil
}

func (m *migrator) ListMigrations(ctx context.Context, clinicId string) ([]*Migration, error) {
	return m.migrationRepo.List(ctx, clinicId)
}

func (m *migrator) CreateEmptyClinic(ctx context.Context, userId string) (*clinics.Clinic, error) {
	if err := m.assertUserIsClinician(userId); err != nil {
		return nil, err
	}

	return m.clinicsCreator.CreateClinic(ctx, &manager.CreateClinic{
		Clinic:        *clinics.NewClinicWithDefaults(),
		CreatorUserId: userId,
	})
}

func (m *migrator) MigrateLegacyClinicianPatients(ctx context.Context, clinicId, userId string) (*Migration, error) {
	// Make sure the clinician is a member or admin of the clinic
	_, err := m.cliniciansService.Get(ctx, clinicId, userId)
	if err != nil {
		return nil, err
	}

	clinic, err := m.clinicsService.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	// Allow migrating legacy clinician patients only after the initial migration has been triggered
	if !clinic.IsMigrated {
		return nil, ErrAlreadyMigrated
	}

	migration := NewMigration(clinicId, userId)
	return m.migrationRepo.Create(ctx, migration)
}

func (m *migrator) TriggerInitialMigration(ctx context.Context, clinicId string) (*Migration, error) {
	clinic, err := m.clinicsService.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}
	if clinic.IsMigrated {
		return nil, ErrAlreadyMigrated
	}
	if !clinic.CanMigrate() {
		return nil, ErrIncompleteClinicProfile
	}

	userId, err := getClinicianForInitialMigration(clinic)
	if err != nil {
		return nil, err
	}
	if err = m.assertUserIsClinician(userId); err != nil {
		return nil, err
	}

	result, err := store.WithTransaction(ctx, m.dbClient, func(sessionContext mongo.SessionContext) (any, error) {
		migration := NewMigration(clinicId, userId)
		result, err := m.migrationRepo.Create(ctx, migration)
		if err != nil {
			return nil, err
		}

		clinic.IsMigrated = true
		_, err = m.clinicsService.Update(ctx, clinicId, clinic)
		if err != nil {
			return nil, err
		}

		return result, nil
	})

	return result.(*Migration), err
}

func (m *migrator) GetMigration(ctx context.Context, clinicId string, userId string) (*Migration, error) {
	return m.migrationRepo.Get(ctx, clinicId, userId)
}

func (m *migrator) UpdateMigrationStatus(ctx context.Context, clinicId, userId, status string) (*Migration, error) {
	return m.migrationRepo.UpdateStatus(ctx, clinicId, userId, status)
}

func (m *migrator) assertUserIsClinician(userId string) error {
	user, err := m.userService.GetUser(userId)
	if err != nil {
		return err
	}

	if !user.IsClinic() {
		return fmt.Errorf("%w: user %v is not clinician", internalErrs.ConstraintViolation, userId)
	}

	return nil
}

func getClinicianForInitialMigration(clinic *clinics.Clinic) (string, error) {
	if clinic.Admins == nil || len(*clinic.Admins) == 0 {
		return "", fmt.Errorf("clinic not found")
	} else if len(*clinic.Admins) > 1 {
		return "", fmt.Errorf("expected clinic with a signle admin")
	}

	return (*clinic.Admins)[0], nil
}
