package clinics

import (
	"context"
	"fmt"
	"time"

	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/store"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrNotFound = fmt.Errorf("clinic %w", errors.NotFound)
var ErrPatientTagNotFound = fmt.Errorf("patient tag %w", errors.NotFound)
var ErrDuplicatePatientTagName = fmt.Errorf("%w patient tag", errors.Duplicate)
var ErrDuplicateShareCode = fmt.Errorf("%w share code", errors.Duplicate)
var ErrAdminRequired = fmt.Errorf("%w: the clinic must have at least one admin", errors.ConstraintViolation)
var MaximumPatientTags = 20
var ErrMaximumPatientTagsExceeded = fmt.Errorf("%w: the clinic already has the maximum number of %v patient tags", errors.ConstraintViolation, MaximumPatientTags)

//go:generate mockgen --build_flags=--mod=mod -source=./clinics.go -destination=./test/mock_service.go -package test MockRepository

type Service interface {
	Get(ctx context.Context, id string) (*Clinic, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinic, error)
	Create(ctx context.Context, clinic *Clinic) (*Clinic, error)
	Update(ctx context.Context, id string, clinic *Clinic) (*Clinic, error)
	Delete(ctx context.Context, id string) error
	UpsertAdmin(ctx context.Context, clinicId, clinicianId string) error
	RemoveAdmin(ctx context.Context, clinicId, clinicianId string, allowOrphaning bool) error
	UpdateTier(ctx context.Context, clinicId, tier string) error
	UpdateSuppressedNotifications(ctx context.Context, clinicId string, suppressedNotifications SuppressedNotifications) error
	CreatePatientTag(ctx context.Context, clinicId, tagName string) (*Clinic, error)
	UpdatePatientTag(ctx context.Context, clinicId, tagId, tagName string) (*Clinic, error)
	DeletePatientTag(ctx context.Context, clinicId, tagId string) (*Clinic, error)
	ListMembershipRestrictions(ctx context.Context, clinicId string) ([]MembershipRestrictions, error)
	UpdateMembershipRestrictions(ctx context.Context, clinicId string, restrictions []MembershipRestrictions) error
	GetEHRSettings(ctx context.Context, clinicId string) (*EHRSettings, error)
	UpdateEHRSettings(ctx context.Context, clinicId string, settings *EHRSettings) error
	GetMRNSettings(ctx context.Context, clinicId string) (*MRNSettings, error)
	UpdateMRNSettings(ctx context.Context, clinicId string, settings *MRNSettings) error
}

type Filter struct {
	Ids              []string
	Email            *string
	ShareCodes       []string
	CreatedTimeStart *time.Time
	CreatedTimeEnd   *time.Time
	EHRSourceId      *string
	EHRFacilityName  *string
	EHREnabled       *bool
}

type Clinic struct {
	Id                      *primitive.ObjectID      `bson:"_id,omitempty"`
	Address                 *string                  `bson:"address,omitempty"`
	City                    *string                  `bson:"city,omitempty"`
	ClinicType              *string                  `bson:"clinicType,omitempty"`
	ClinicSize              *string                  `bson:"clinicSize,omitempty"`
	Country                 *string                  `bson:"country,omitempty"`
	Name                    *string                  `bson:"name,omitempty"`
	PatientTags             []PatientTag             `bson:"patientTags,omitempty"`
	PhoneNumbers            *[]PhoneNumber           `bson:"phoneNumbers,omitempty"`
	PostalCode              *string                  `bson:"postalCode,omitempty"`
	State                   *string                  `bson:"state,omitempty"`
	CanonicalShareCode      *string                  `bson:"canonicalShareCode,omitempty"`
	Website                 *string                  `bson:"website,omitempty"`
	ShareCodes              *[]string                `bson:"shareCodes,omitempty"`
	Admins                  *[]string                `bson:"admins,omitempty"`
	CreatedTime             time.Time                `bson:"createdTime,omitempty"`
	UpdatedTime             time.Time                `bson:"updatedTime,omitempty"`
	IsMigrated              bool                     `bson:"isMigrated,omitempty"`
	Tier                    string                   `bson:"tier,omitempty"`
	PreferredBgUnits        string                   `bson:"PreferredBgUnits,omitempty"`
	SuppressedNotifications SuppressedNotifications  `bson:"suppressedNotifications"`
	MembershipRestrictions  []MembershipRestrictions `bson:"membershipRestrictions,omitempty"`
	EHRSettings             *EHRSettings             `bson:"ehrSettings,omitempty"`
	MRNSettings             *MRNSettings             `bson:"mrnSettings,omitempty"`
}

type EHRSettings struct {
	Enabled        bool              `bson:"enabled"`
	DestinationIds EHRDestinationIds `bson:"destinationIds"`
	Facility       *EHRFacility      `bson:"facility"`
	ProcedureCodes EHRProcedureCodes `bson:"procedureCodes"`
	SourceId       string            `bson:"sourceId"`
}

type EHRFacility struct {
	Name string `bson:"name"`
}

type EHRDestinationIds struct {
	Default   string  `bson:"default"`
	Flowsheet *string `bson:"flowsheet"`
	Notes     *string `bson:"notes"`
	Results   *string `bson:"results"`
}

type EHRProcedureCodes struct {
	SummaryReportsSubscription string `bson:"summaryReportsSubscription"`
}

type MRNSettings struct {
	Required bool `bson:"required"`
	Unique   bool `bson:"unique"`
}

func NewClinic() Clinic {
	return Clinic{
		CreatedTime: time.Now(),
		UpdatedTime: time.Now(),
	}
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
		isStringSet(&c.PreferredBgUnits) &&
		c.Id != nil &&
		c.PhoneNumbers != nil &&
		hasValidPhoneNumber(*c.PhoneNumbers) &&
		hasValidPatientTags(c.PatientTags)
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

type MembershipRestrictions struct {
	EmailDomain string `bson:"emailDomain,omitempty"`
	RequiredIdp string `bson:"requiredIdp,omitempty"`
}

type PhoneNumber struct {
	Type   *string `bson:"type,omitempty"`
	Number string  `bson:"number,omitempty"`
}

func (p *PhoneNumber) HasAllRequiredFields() bool {
	return isStringSet(&p.Number)
}

type PatientTag struct {
	Id   *primitive.ObjectID `bson:"_id,omitempty"`
	Name string              `bson:"name,omitempty"`
}

type SuppressedNotifications struct {
	PatientClinicInvitation *bool `bson:"patientClinicInvitation,omitempty"`
}

func (p *PatientTag) HasAllRequiredFields() bool {
	return p.Id != nil &&
		isStringSet(&p.Name)
}

func hasValidPhoneNumber(phoneNumbers []PhoneNumber) bool {
	for _, p := range phoneNumbers {
		if p.HasAllRequiredFields() {
			return true
		}
	}
	return false
}

func hasValidPatientTags(patientTags []PatientTag) bool {
	for _, p := range patientTags {
		if !p.HasAllRequiredFields() {
			return false
		}
	}
	return true
}

func isStringSet(s *string) bool {
	return s != nil && *s != ""
}
