package clinics

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrNotFound = fmt.Errorf("clinic %w", errors.NotFound)
var ErrDuplicateEmail = fmt.Errorf("%w email address", errors.Duplicate)

type Service interface {
	Get(ctx context.Context, id string) (*Clinic, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinic, error)
	Create(ctx context.Context, clinic *Clinic) (*Clinic, error)
	Update(ctx context.Context, id string, clinic *Clinic) (*Clinic, error)
}

type Filter struct {
	Ids   []string
	Email string
}

type Clinic struct {
	Id           *primitive.ObjectID `bson:"_id,omitempty"`
	Address      *string            `bson:"address,omitempty"`
	City         *string            `bson:"city,omitempty"`
	ClinicType   *string            `bson:"clinicType,omitempty"`
	Country      *string            `bson:"country,omitempty"`
	Email        *string            `bson:"Email,omitempty"`
	Name         *string            `bson:"name,omitempty"`
	PhoneNumbers *[]PhoneNumber     `bson:"phoneNumbers,omitempty"`
	PostalCode   *string            `bson:"postalCode,omitempty"`
	State        *string            `bson:"state,omitempty"`
}

type PhoneNumber struct {
	Type   *string `bson:"type,omitempty"`
	Number string `bson:"number,omitempty"`
}
