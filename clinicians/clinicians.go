package clinicians

import (
	"context"
	"github.com/tidepool-org/clinic/store"
)

type Service interface {
	Get(ctx context.Context, clinicId string, userId string) (*Clinician, error)
	List(ctx context.Context, clinicId string, filter *Filter, pagination store.Pagination) ([]*Clinician, error)
	Create(ctx context.Context, clinicId string, clinician *Clinician) (*Clinician, error)
	Update(ctx context.Context, clinicId string, clinician *Clinician) (*Clinician, error)
	Delete(ctx context.Context, clinicId string, userId string) error
}

type Clinician struct {
	Email       *string  `bson:"email,omitempty"`
	UserId      *string  `bson:"userId,omitempty"`
	InviteId    *string  `bson:"inviteId,omitempty"`
	Name        *string  `bson:"name,omitempty"`
	Permissions []string `bson:"permissions,omitempty"`
}

type Filter struct {
	Search *string
}
