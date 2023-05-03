package clinicians

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"sort"
	"time"
)

var (
	ErrNotFound  = fmt.Errorf("clinician %w", errors.NotFound)
	ErrDuplicate = fmt.Errorf("%w: clinician is already a member of the clinic", errors.Duplicate)

	ClinicAdmin = "CLINIC_ADMIN"
)

type Service interface {
	Get(ctx context.Context, clinicId string, clinicianId string) (*Clinician, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinician, error)
	Create(ctx context.Context, clinician *Clinician) (*Clinician, error)
	Update(ctx context.Context, update *ClinicianUpdate) (*Clinician, error)
	UpdateAll(ctx context.Context, update *CliniciansUpdate) error
	Delete(ctx context.Context, clinicId string, clinicianId string) error
	DeleteAll(ctx context.Context, clinicId string) error
	DeleteFromAllClinics(ctx context.Context, clinicianId string) error
	GetInvite(ctx context.Context, clinicId, inviteId string) (*Clinician, error)
	DeleteInvite(ctx context.Context, clinicId, inviteId string) error
	AssociateInvite(ctx context.Context, associate AssociateInvite) (*Clinician, error)
}

type AssociateInvite struct {
	ClinicId      string
	InviteId      string
	UserId        string
	ClinicianName *string
}

type ClinicianUpdate struct {
	UpdatedBy   string
	ClinicId    string
	ClinicianId string
	Clinician   Clinician
}

// Update multiple clinician records belonging to the same user
type CliniciansUpdate struct {
	UserId string
	Email  string
}

type Clinician struct {
	Id           *primitive.ObjectID `bson:"_id,omitempty"`
	InviteId     *string             `bson:"inviteId,omitempty"`
	ClinicId     *primitive.ObjectID `bson:"clinicId,omitempty"`
	UserId       *string             `bson:"userId,omitempty"`
	Email        *string             `bson:"email,omitempty"`
	Name         *string             `bson:"name"`
	Roles        []string            `bson:"roles"`
	RolesUpdates []RolesUpdate       `bson:"rolesUpdates,omitempty"`
	CreatedTime  time.Time           `bson:"createdTime"`
	UpdatedTime  time.Time           `bson:"updatedTime"`
}

type RolesUpdate struct {
	Roles     []string `bson:"roles"`
	UpdatedBy string   `bson:"updatedBy"`
}

func (c *Clinician) IsAdmin() bool {
	isAdmin := false
	for _, role := range c.Roles {
		if role == ClinicAdmin {
			isAdmin = true
			break
		}
	}
	return isAdmin
}

func (c *Clinician) RolesChanged(newRoles []string) bool {
	if len(c.Roles) != len(newRoles) {
		return true
	}

	sort.Strings(c.Roles)
	sort.Strings(newRoles)

	for i, role := range c.Roles {
		if newRoles[i] != role {
			return true
		}
	}

	return false
}

type Filter struct {
	ClinicId         *string
	UserId           *string
	Search           *string
	Email            *string
	Role             *string
	CreatedTimeStart *time.Time
	CreatedTimeEnd   *time.Time
}
