package clinics

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store"
)

const (
	CollectionName             = "clinics"
	DefaultMrnIdType           = "MRN"
	WorkspaceIdTypeClinicId    = "clinicId"
	WorkspaceIdTypeEHRSourceId = "ehrSourceId"

	CountryCodeUS                            = "US"
	PatientCountSettingsHardLimitPlanDefault = 250
	DefaultCoefficientOfVariationUnits       = "UNIT_INTERVAL"
)

var (
	EHRProviderRedox  = "redox"
	EHRProviderXealth = "xealth"
)

var ErrNotFound = fmt.Errorf("clinic %w", errors.NotFound)
var ErrPatientTagNotFound = fmt.Errorf("patient tag %w", errors.NotFound)
var ErrDuplicatePatientTagName = fmt.Errorf("%w patient tag", errors.Duplicate)
var ErrDuplicateShareCode = fmt.Errorf("%w share code", errors.Duplicate)
var ErrAdminRequired = fmt.Errorf("%w: the clinic must have at least one admin", errors.ConstraintViolation)
var MaximumPatientTags = 50
var ErrMaximumPatientTagsExceeded = fmt.Errorf("%w: the clinic already has the maximum number of %v patient tags", errors.ConstraintViolation, MaximumPatientTags)
var ErrDuplicateSiteName = fmt.Errorf("%w site name", errors.Duplicate)
var ErrMaximumSitesExceeded = fmt.Errorf("%w: the clinic already has the maximum number of %d sites", errors.ConstraintViolation, sites.MaxSitesPerClinic)
var ErrSiteNotFound = fmt.Errorf("%w: the clinic has no site with that name", errors.ConstraintViolation)

//go:generate go tool mockgen -source=./clinics.go -destination=./test/mock_clinics.go -package test

type Service interface {
	Get(ctx context.Context, id string) (*Clinic, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinic, error)
	Create(ctx context.Context, clinic *Clinic) (*Clinic, error)
	Update(ctx context.Context, id string, clinic *Clinic) (*Clinic, error)
	Delete(ctx context.Context, id string, metadata deletions.Metadata) error
	UpsertAdmin(ctx context.Context, clinicId, clinicianId string) error
	RemoveAdmin(ctx context.Context, clinicId, clinicianId string, allowOrphaning bool) error
	UpdateTier(ctx context.Context, clinicId, tier string) error
	UpdateSuppressedNotifications(ctx context.Context, clinicId string, suppressedNotifications SuppressedNotifications) error
	CreatePatientTag(ctx context.Context, clinicId, tagName string) (*PatientTag, error)
	ListPatientTags(ctx context.Context, clinicId string) ([]PatientTag, error)
	UpdatePatientTag(ctx context.Context, clinicId, tagId, tagName string) (*PatientTag, error)
	DeletePatientTag(ctx context.Context, clinicId, tagId string) error
	ListMembershipRestrictions(ctx context.Context, clinicId string) ([]MembershipRestrictions, error)
	UpdateMembershipRestrictions(ctx context.Context, clinicId string, restrictions []MembershipRestrictions) error
	GetEHRSettings(ctx context.Context, clinicId string) (*EHRSettings, error)
	UpdateEHRSettings(ctx context.Context, clinicId string, settings *EHRSettings) error
	GetMRNSettings(ctx context.Context, clinicId string) (*MRNSettings, error)
	UpdateMRNSettings(ctx context.Context, clinicId string, settings *MRNSettings) error
	GetPatientCountSettings(ctx context.Context, clinicId string) (*PatientCountSettings, error)
	UpdatePatientCountSettings(ctx context.Context, clinicId string, settings *PatientCountSettings) error
	GetPatientCount(ctx context.Context, clinicId string) (*PatientCount, error)
	RefreshPatientCount(ctx context.Context, clinicId string) error
	AppendShareCodes(ctx context.Context, clinicId string, shareCodes []string) error
	CreateSite(ctx context.Context, clinicId string, site *sites.Site) (*sites.Site, error)
	CreateSiteIgnoringLimit(ctx context.Context, clinicId string, site *sites.Site) (*sites.Site, error)
	DeleteSite(ctx context.Context, clinicId, siteId string) error
	ListSites(ctx context.Context, clinicId string) ([]sites.Site, error)
	UpdateSite(ctx context.Context, clinicId, siteId string, site *sites.Site) (*sites.Site, error)
}

type Repository interface {
	Get(ctx context.Context, id string) (*Clinic, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination) ([]*Clinic, error)
	Create(ctx context.Context, clinic *Clinic) (*Clinic, error)
	Update(ctx context.Context, id string, clinic *Clinic) (*Clinic, error)
	Delete(ctx context.Context, id string, metadata deletions.Metadata) error
	UpsertAdmin(ctx context.Context, clinicId, clinicianId string) error
	RemoveAdmin(ctx context.Context, clinicId, clinicianId string, allowOrphaning bool) error
	UpdateTier(ctx context.Context, clinicId, tier string) error
	UpdateSuppressedNotifications(ctx context.Context, clinicId string, suppressedNotifications SuppressedNotifications) error
	CreatePatientTag(ctx context.Context, clinicId, tagName string) (*PatientTag, error)
	UpdatePatientTag(ctx context.Context, clinicId, tagId, tagName string) (*PatientTag, error)
	DeletePatientTag(ctx context.Context, clinicId, tagId string) error
	ListPatientTags(ctx context.Context, clinicId string) ([]PatientTag, error)
	UpdateMembershipRestrictions(ctx context.Context, clinicId string, restrictions []MembershipRestrictions) error
	UpdateEHRSettings(ctx context.Context, clinicId string, settings *EHRSettings) error
	UpdateMRNSettings(ctx context.Context, clinicId string, settings *MRNSettings) error
	UpdatePatientCountSettings(ctx context.Context, clinicId string, settings *PatientCountSettings) error
	UpdatePatientCount(ctx context.Context, clinicId string, patientCount *PatientCount) error
	AppendShareCodes(ctx context.Context, clinicId string, shareCodes []string) error
	CreateSite(ctx context.Context, clinicId string, site *sites.Site) (*sites.Site, error)
	ListSites(ctx context.Context, clinicId string) ([]sites.Site, error)
	CreateSiteIgnoringLimit(ctx context.Context, clinicId string, site *sites.Site) (*sites.Site, error)
	DeleteSite(ctx context.Context, clinicId, siteId string) error
	UpdateSite(ctx context.Context, clinicId, siteId string, site *sites.Site) (*sites.Site, error)
}

type Filter struct {
	Ids                             []string
	Email                           *string
	ShareCodes                      []string
	CreatedTimeStart                *time.Time
	CreatedTimeEnd                  *time.Time
	EHRProvider                     *string
	EHRSourceId                     *string
	EHREnabled                      *bool
	ScheduledReportsOnUploadEnabled *bool
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
	SuppressedNotifications *SuppressedNotifications `bson:"suppressedNotifications,omitempty"`
	Timezone                *string                  `bson:"timezone"`
	MembershipRestrictions  []MembershipRestrictions `bson:"membershipRestrictions,omitempty"`
	EHRSettings             *EHRSettings             `bson:"ehrSettings,omitempty"`
	MRNSettings             *MRNSettings             `bson:"mrnSettings,omitempty"`
	PatientCountSettings    *PatientCountSettings    `bson:"patientCountSettings,omitempty"`
	PatientCount            *PatientCount            `bson:"patientCount,omitempty"`
	Sites                   []sites.Site             `bson:"sites,omitempty"`
}

func (c Clinic) IsOUS() bool {
	return c.Country != nil && *c.Country != CountryCodeUS
}

type EHRSettings struct {
	Enabled          bool               `bson:"enabled"`
	Provider         string             `bson:"provider"`
	DestinationIds   *EHRDestinationIds `bson:"destinationIds"`
	ProcedureCodes   EHRProcedureCodes  `bson:"procedureCodes"`
	SourceId         string             `bson:"sourceId"`
	MrnIdType        string             `bson:"mrnIdType"`
	ScheduledReports ScheduledReports   `bson:"scheduledReports"`
	Tags             TagsSettings       `bson:"tags"`
	Flowsheets       FlowsheetSettings  `bson:"flowsheets"`
}

func (e *EHRSettings) GetMrnIDType() string {
	if e.MrnIdType == "" {
		return DefaultMrnIdType
	}
	return e.MrnIdType
}

type EHRDestinationIds struct {
	Flowsheet string `bson:"flowsheet"`
	Notes     string `bson:"notes"`
	Results   string `bson:"results"`
}

type EHRProcedureCodes struct {
	EnableSummaryReports          *string `bson:"enableSummaryReports,omitempty"`
	DisableSummaryReports         *string `bson:"disableSummaryReports,omitempty"`
	CreateAccount                 *string `bson:"createAccount,omitempty"`
	CreateAccountAndEnableReports *string `bson:"createAccountAndEnableReports,omitempty"`
}

type ScheduledReports struct {
	Cadence               string  `bson:"cadence"`
	OnUploadEnabled       bool    `bson:"onUploadEnabled"`
	OnUploadNoteEventType *string `bson:"onUploadNoteEventType"`
}

type MRNSettings struct {
	Required bool `bson:"required"`
	Unique   bool `bson:"unique"`
}

type TagsSettings struct {
	Codes     []string `bson:"codes"`
	Separator *string  `bson:"separator"`
}

type FlowsheetSettings struct {
	Icode bool `bson:"icode,omitempty"`
}

type PatientProviderCount struct {
	States map[string]int `bson:"states,omitempty"`
	Total  int            `bson:"total"`
}

type PatientCount struct {
	Total     int                             `bson:"total"`
	Demo      int                             `bson:"demo"`
	Plan      int                             `bson:"plan"`
	Providers map[string]PatientProviderCount `bson:"providers,omitempty"`
}

func NewPatientCount() *PatientCount {
	return &PatientCount{}
}

type PatientCountSettings struct {
	HardLimit *PatientCountLimit `bson:"hardLimit,omitempty"`
	SoftLimit *PatientCountLimit `bson:"softLimit,omitempty"`
}

func (p PatientCountSettings) IsValid() bool {
	if p.HardLimit != nil && !p.HardLimit.IsValid() {
		return false
	}
	if p.SoftLimit != nil && !p.SoftLimit.IsValid() {
		return false
	}
	return true
}

func DefaultPatientCountSettings() *PatientCountSettings {
	return &PatientCountSettings{
		HardLimit: &PatientCountLimit{
			Plan: PatientCountSettingsHardLimitPlanDefault,
		},
	}
}

type PatientCountLimit struct {
	Plan      int        `bson:"plan"`
	StartDate *time.Time `bson:"startDate,omitempty"`
	EndDate   *time.Time `bson:"endDate,omitempty"`
}

func (p PatientCountLimit) IsValid() bool {
	if p.Plan < 0 {
		return false
	}
	if p.StartDate != nil && p.EndDate != nil && p.StartDate.After(*p.EndDate) {
		return false
	}
	return true
}

func NewClinicWithDefaults() *Clinic {
	c := NewClinic()
	c.PatientCount = NewPatientCount()
	c.PatientCountSettings = DefaultPatientCountSettings()
	return c
}

func NewClinic() *Clinic {
	return &Clinic{}
}

func (c *Clinic) UpdatePatientCountSettingsForCountry() bool {
	if isOUS := c.IsOUS(); isOUS && c.PatientCountSettings != nil {
		c.PatientCountSettings = nil
		return true
	} else if !isOUS && c.PatientCountSettings == nil {
		c.PatientCountSettings = DefaultPatientCountSettings()
		return true
	}
	return false
}

func (c *Clinic) HasAllRequiredFields() bool {
	return c.Id != nil &&
		isStringSet(c.Address) &&
		isStringSet(c.City) &&
		isStringSet(c.ClinicType) &&
		isStringSet(c.Country) &&
		isStringSet(c.Name) &&
		isStringSet(c.PostalCode) &&
		isStringSet(c.State) &&
		isStringSet(&c.PreferredBgUnits) &&
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

func (m MembershipRestrictions) String() string {
	return fmt.Sprintf("%s/%s", m.EmailDomain, m.RequiredIdp)
}

type PhoneNumber struct {
	Type   *string `bson:"type,omitempty"`
	Number string  `bson:"number,omitempty"`
}

func (p *PhoneNumber) HasAllRequiredFields() bool {
	return isStringSet(&p.Number)
}

type PatientTag struct {
	Id       *primitive.ObjectID `bson:"_id,omitempty"`
	Name     string              `bson:"name,omitempty"`
	Patients int                 `bson:"patients,omitempty"`
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

func FilterByWorkspaceId(clinics []*Clinic, workspaceId, workspaceIdType string) ([]*Clinic, error) {
	switch workspaceIdType {
	case WorkspaceIdTypeClinicId:
		return filterByClinicId(clinics, workspaceId)
	case WorkspaceIdTypeEHRSourceId:
		return filterByEHRSourceId(clinics, workspaceId)
	}
	return nil, fmt.Errorf("%w: unknown workspace identifier type", errors.BadRequest)
}

func filterByClinicId(clinics []*Clinic, clinicId string) ([]*Clinic, error) {
	var results []*Clinic
	for _, clinic := range clinics {
		clinic := clinic
		if clinic != nil && clinic.Id != nil && clinic.Id.Hex() == clinicId {
			results = append(results, clinic)
		}
	}

	return results, nil
}

func filterByEHRSourceId(clinics []*Clinic, sourceId string) ([]*Clinic, error) {
	var results []*Clinic
	for _, clinic := range clinics {
		clinic := clinic
		if clinic != nil && clinic.EHRSettings != nil && clinic.EHRSettings.SourceId == sourceId {
			results = append(results, clinic)
		}
	}

	return results, nil
}

func AssertCanAddPatientTag(clinic Clinic, tag PatientTag) error {
	if len(clinic.PatientTags) >= MaximumPatientTags {
		return ErrMaximumPatientTagsExceeded
	}

	if IsDuplicatePatientTag(clinic, tag) {
		return ErrDuplicatePatientTagName
	}

	return nil
}

func IsDuplicatePatientTag(clinic Clinic, tag PatientTag) bool {
	trimmedNewTagName := strings.ToLower(strings.ReplaceAll(tag.Name, " ", ""))

	for _, p := range clinic.PatientTags {
		// We only check for duplication against other tags
		if p.Id.Hex() != tag.Id.Hex() {
			trimmedExistingTagName := strings.ToLower(strings.ReplaceAll(p.Name, " ", ""))

			if trimmedExistingTagName == trimmedNewTagName {
				return true
			}
		}
	}

	return false
}
