package migration

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/creator"
	internalErrs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.uber.org/fx"
	"time"
)

const (
	StatusPending = "PENDING"
)

var ErrAlreadyMigrated = fmt.Errorf("%w: clinic is already migrated", internalErrs.ConstraintViolation)

type Migration struct {
	ClinicId    *primitive.ObjectID `json:"clinicId" bson:"clinicId"`
	UserId      string              `json:"userId" bson:"userId"`
	Status      string              `json:"status" bson:"status"`
	CreatedTime time.Time           `json:"createdTime" bson:"createdTime"`
	UpdatedTime time.Time           `json:"updatedTime" bson:"updatedTime"`
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

	ClinicsCreator    creator.Creator
	ClinicsService    clinics.Service
	CliniciansService clinicians.Service
	MigrationRepo     Repository
	UserService       patients.UserService
}

type migrator struct {
	clinicsCreator    creator.Creator
	clinicsService    clinics.Service
	cliniciansService clinicians.Service
	migrationRepo     Repository
	userService       patients.UserService
}

func NewMigrator(p Params) (Migrator, error) {
	return &migrator{
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

	return m.clinicsCreator.CreateClinic(ctx, &creator.CreateClinic{
		Clinic:        clinics.Clinic{},
		CreatorUserId: userId,
	})
}

func (m *migrator) MigrateLegacyClinicianPatients(ctx context.Context, clinicId, userId string) (*Migration, error) {
	// Make sure the clinician is a member or admin of the clinic
	_, err := m.cliniciansService.Get(ctx, clinicId, userId)
	if err != nil {
		return nil, err
	}

	clinicObjId, err := primitive.ObjectIDFromHex(clinicId)
	if err != nil {
		return nil, err
	}

	migration := &Migration{
		ClinicId:    &clinicObjId,
		UserId:      userId,
		CreatedTime: time.Now(),
	}

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

	userId, err := getClinicianForInitialMigration(clinic)
	if err != nil {
		return nil, err
	}
	if err := m.assertUserIsClinician(userId); err != nil {
		return nil, err
	}

	migration := &Migration{
		ClinicId:    clinic.Id,
		UserId:      userId,
		CreatedTime: time.Now(),
		UpdatedTime: time.Now(),
		Status:      StatusPending,
	}

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
