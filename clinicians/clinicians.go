package clinicians

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrNotFound = fmt.Errorf("clinician %w", errors.NotFound)
var ErrDuplicate = fmt.Errorf("%w: clinician is already a member of the clinic", errors.Duplicate)

type Service interface {
	Get(ctx context.Context, clinicId string, id string) (*Clinician, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinician, error)
	Create(ctx context.Context, clinician *Clinician) (*Clinician, error)
	Update(ctx context.Context, clinicId string, id string, clinician *Clinician) (*Clinician, error)
	Delete(ctx context.Context, clinicId string, id string) error
}

type Clinician struct {
	Id          *primitive.ObjectID `bson:"_id,omitempty"`
	ClinicId    *primitive.ObjectID `bson:"clinicId,omitempty"`
	UserId      *string             `bson:"userId,omitempty"`
	Name        *string             `bson:"name"`
	Email       *string             `bson:"email"`
	InviteId    *string             `bson:"inviteId,omitempty"`
	Permissions *[]string           `bson:"permissions"`
}

type Filter struct {
	ClinicId string
	UserId   *string
	Search   *string
}
