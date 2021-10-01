package clinics

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"time"
)

var ErrNotFound = fmt.Errorf("clinic %w", errors.NotFound)
var ErrDuplicateShareCode = fmt.Errorf("%w share code", errors.Duplicate)
var ErrAdminRequired = fmt.Errorf("%w: the clinic must have at least one admin", errors.ConstraintViolation)

type Service interface {
	Get(ctx context.Context, id string) (*Clinic, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinic, error)
	Create(ctx context.Context, clinic *Clinic) (*Clinic, error)
	Update(ctx context.Context, id string, clinic *Clinic) (*Clinic, error)
	UpsertAdmin(ctx context.Context, clinicId, clinicianId string) error
	RemoveAdmin(ctx context.Context, clinicId, clinicianId string) error
}

type Filter struct {
	Ids        []string
	Email      *string
	ShareCodes []string
}

type Clinic struct {
	Id                 *primitive.ObjectID `bson:"_id,omitempty"`
	Address            *string             `bson:"address,omitempty"`
	City               *string             `bson:"city,omitempty"`
	ClinicType         *string             `bson:"clinicType,omitempty"`
	ClinicSize         *string             `bson:"clinicSize,omitempty"`
	Country            *string             `bson:"country,omitempty"`
	Name               *string             `bson:"name,omitempty"`
	PhoneNumbers       *[]PhoneNumber      `bson:"phoneNumbers,omitempty"`
	PostalCode         *string             `bson:"postalCode,omitempty"`
	State              *string             `bson:"state,omitempty"`
	CanonicalShareCode *string             `bson:"canonicalShareCode,omitempty"`
	Website            *string             `bson:"website,omitempty"`
	ShareCodes         *[]string           `bson:"shareCodes,omitempty"`
	Admins             *[]string           `bson:"admins,omitempty"`
	CreatedTime        time.Time           `bson:"createdTime"`
	UpdatedTime        time.Time           `bson:"updatedTime"`
	IsMigrated         bool                `bson:"isMigrated,omitempty"`
}

func (c *Clinic) HasAllRequiredFields() bool {
	return c.Id != nil &&
		isStringSet(c.Address) &&
		isStringSet(c.City) &&
		isStringSet(c.ClinicType) &&
		isStringSet(c.ClinicSize) &&
		isStringSet(c.Country) &&
		isStringSet(c.Name) &&
		isStringSet(c.PostalCode) &&
		isStringSet(c.State) &&
		c.Id != nil &&
		c.PhoneNumbers != nil &&
		hasValidPhoneNumber(*c.PhoneNumbers)

}

func (c *Clinic) AddAdmin(userId string) {
	admins := make([]string, 0)
	if c.Admins != nil {
		admins = *c.Admins
	}
	admins = append(admins, userId)
	c.Admins = &admins
}

func (c *Clinic) CanMigrate() bool {
	return !c.IsMigrated &&
		c.HasAllRequiredFields() &&
		c.Admins != nil && len(*c.Admins) > 0
}

type PhoneNumber struct {
	Type   *string `bson:"type,omitempty"`
	Number string  `bson:"number,omitempty"`
}

func (p *PhoneNumber) HasAllRequiredFields() bool {
	return isStringSet(&p.Number)
}

func hasValidPhoneNumber(phoneNumbers []PhoneNumber) bool {
	for _, p := range phoneNumbers {
		if p.HasAllRequiredFields() {
			return true
		}
	}
	return false
}

func isStringSet(s *string) bool {
	return s != nil && *s != ""
}
