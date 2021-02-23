package clinicians

import (
	"context"
	"errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrNotFound = errors.New("clinician not found")
var ErrDuplicate = errors.New("clinician is already member of the clinic")

type Service interface {
	Get(ctx context.Context, clinicId string, userId string) (*Clinician, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinician, error)
	Create(ctx context.Context, clinician *Clinician) (*Clinician, error)
	Update(ctx context.Context, clinicId string, userId string, clinician *Clinician) (*Clinician, error)
	Delete(ctx context.Context, clinicId string, userId string) error

	GetByInviteId(ctx context.Context, clinicId string, inviteId string) (*Clinician, error)
	UpdateByInviteId(ctx context.Context, clinicId string, inviteId string, clinician *Clinician) (*Clinician, error)
	DeleteByInviteId(ctx context.Context, clinicId string, inviteId string) error
}

type Clinician struct {
	ClinicId    *primitive.ObjectID `bson:"clinicId,omitempty"`
	UserId      *string             `bson:"userId,omitempty"`
	Email       *string             `bson:"email,omitempty"`
	InviteId    *string             `bson:"inviteId,omitempty"`
	Name        *string             `bson:"name,omitempty"`
	Permissions *[]string           `bson:"permissions,omitempty"`
}

type Filter struct {
	ClinicId string
	UserId   *string
	Search   *string
}
