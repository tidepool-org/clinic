package patients

import (
	"context"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/bson/bsoncodec"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/clinic/deletions"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store"
)

const (
	CollectionName                     = "patients"
	SubscriptionRedoxSummaryAndReports = "summaryAndReports"
	SubscriptionXealthReports          = "xealthReports"
)

var (
	ErrNotFound           = fmt.Errorf("patient %w", errors.NotFound)
	ErrSummaryNotFound    = fmt.Errorf("summary %w", errors.NoChange)
	ErrPermissionNotFound = fmt.Errorf("permission %w", errors.NotFound)
	ErrDuplicatePatient   = fmt.Errorf("%w: patient is already a member of the clinic", errors.Duplicate)
	ErrDuplicateEmail     = fmt.Errorf("%w: email address is already taken", errors.Duplicate)
	ErrReviewNotOwner     = fmt.Errorf("%w: cannot revert review from another clinician", errors.Conflict)

	ErrConnectionIssueNotFound = fmt.Errorf("connection issue %w", errors.NotFound)

	PendingDataSourceExpirationDuration = time.Hour * 24 * 30

	// StaleDuration is how long data or an invitation may go without activity before it
	// is considered stale.
	StaleDuration = time.Hour * 48

	// InvitationExpirationDuration is how long a device non-specific invitation stays
	// valid. It matches hydrophone's TypeSignUp confirmation timeout of 31 days. Reminders
	// do not extend it and new invitations are not issued, so it is measured from the
	// patient's creation.
	InvitationExpirationDuration = time.Hour * 24 * 31

	DexcomDataSourceProviderName = "dexcom"
	TwiistDataSourceProviderName = "twiist"
	AbbottDataSourceProviderName = "abbott"

	DataSourceStateConnected    = "connected"
	DataSourceStateDisconnected = "disconnected"
	DataSourceStateError        = "error"

	// connectionIssueProviderPriority breaks ties between data sources that enter the
	// connected state at the same time. Earlier entries win.
	connectionIssueProviderPriority = []string{
		TwiistDataSourceProviderName,
		DexcomDataSourceProviderName,
		AbbottDataSourceProviderName,
	}

	permission                  = make(Permission, 0)
	CustodialAccountPermissions = Permissions{
		Custodian: &permission,
		View:      &permission,
		Upload:    &permission,
		Note:      &permission,
	}
)

//go:generate go tool mockgen -source=./patients.go -destination=./test/mock_patients.go -package test
type Service interface {
	Get(ctx context.Context, clinicId string, userId string) (*Patient, error)
	Count(ctx context.Context, filter *Filter) (int, error)
	List(ctx context.Context, filter *Filter, pagination store.Pagination, sort []*store.Sort) (*ListResult, error)
	Create(ctx context.Context, patient Patient) (*Patient, error)
	Update(ctx context.Context, update PatientUpdate) (*Patient, error)
	AddReview(ctx context.Context, clinicId, userId string, review Review) ([]Review, error)
	DeleteReview(ctx context.Context, clinicId, clinicianId, userId string) ([]Review, error)
	UpdateEmail(ctx context.Context, userId string, email *string) error
	Remove(ctx context.Context, clinicId string, userId string, metadata deletions.Metadata) error
	UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *Permissions) (*Patient, error)
	DeletePermission(ctx context.Context, clinicId, userId, permission string) (*Patient, error)
	DeleteFromAllClinics(ctx context.Context, userId string, metadata deletions.Metadata) ([]string, error)
	DeleteNonCustodialPatientsOfClinic(ctx context.Context, clinicId string, metadata deletions.Metadata) error
	UpdateSummaryInAllClinics(ctx context.Context, userId string, summary *Summary) error
	DeleteSummaryInAllClinics(ctx context.Context, summaryId string) error
	UpdateLastUploadReminderTime(ctx context.Context, update *UploadReminderUpdate) (*Patient, error)
	UpdateLastInvitationSent(ctx context.Context, clinicId, userId string, sentTime time.Time) error
	AddProviderConnectionRequest(ctx context.Context, clinicId, userId string, request ConnectionRequest) error
	AssignPatientTagToClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error
	DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error
	ConvertPatientTagToSite(ctx context.Context, clinicId, patientTagId string, site *sites.Site) error
	UpdatePatientDataSources(ctx context.Context, userId string, dataSources *DataSources) error
	UpdateConnectionIssues(ctx context.Context) error
	SetConnectionIssueHidden(ctx context.Context, clinicId, userId string,
		hidden bool) (*Patient, error)
	TideReport(ctx context.Context, clinicId string, params TideReportParams) (*Tide, error)
	ListExportedPatients(ctx context.Context, params ExportParams) ([]ExportedPatient, error)
	UpdateEHRSubscription(ctx context.Context, clinicId, userId string, update SubscriptionUpdate) error
	RescheduleLastSubscriptionOrderForAllPatients(ctx context.Context, clinicId, subscription, ordersCollection, targetCollection string) error
	RescheduleLastSubscriptionOrderForPatient(ctx context.Context, clinicIds []string, userId, subscription, ordersCollection, targetCollection string) error
	DeleteSites(ctx context.Context, clinicId string, siteId string) error
	MergeSites(ctx context.Context, clinicId, sourceSiteId string, targetSite *sites.Site) error
	UpdateSites(ctx context.Context, clinicId string, siteId string, site *sites.Site) error
}

type Repository interface {
	Service

	ClinicIds(ctx context.Context, userId string) ([]string, error)
	Counts(ctx context.Context, clinicId string) (*Counts, error)
}

type ProviderCounts struct {
	States map[string]int `bson:"states,omitempty"`
	Total  int            `bson:"total"`
}

type Counts struct {
	Total     int                       `bson:"total"`
	Demo      int                       `bson:"demo"`
	Plan      int                       `bson:"plan"`
	Providers map[string]ProviderCounts `bson:"providers,omitempty"`
}

type Patient struct {
	Id                         *primitive.ObjectID        `bson:"_id,omitempty"`
	ClinicId                   *primitive.ObjectID        `bson:"clinicId,omitempty"`
	UserId                     *string                    `bson:"userId,omitempty"`
	BirthDate                  *string                    `bson:"birthDate"`
	Email                      *string                    `bson:"email"`
	FullName                   *string                    `bson:"fullName"`
	Mrn                        *string                    `bson:"mrn"`
	TargetDevices              *[]string                  `bson:"targetDevices"`
	Tags                       *[]primitive.ObjectID      `bson:"tags,omitempty"`
	DataSources                *[]DataSource              `bson:"dataSources,omitempty"`
	Permissions                *Permissions               `bson:"permissions,omitempty"`
	IsMigrated                 bool                       `bson:"isMigrated,omitempty"`
	LegacyClinicianIds         []string                   `bson:"legacyClinicianIds,omitempty"`
	CreatedTime                time.Time                  `bson:"createdTime,omitempty"`
	UpdatedTime                time.Time                  `bson:"updatedTime,omitempty"`
	InvitedBy                  *string                    `bson:"invitedBy,omitempty"`
	Summary                    *Summary                   `bson:"summary,omitempty"`
	Reviews                    []Review                   `bson:"reviews,omitempty"`
	LastUploadReminderTime     time.Time                  `bson:"lastUploadReminderTime,omitempty"`
	LastInvitationSent         time.Time                  `bson:"lastInvitationSent,omitempty"`
	ProviderConnectionRequests ProviderConnectionRequests `bson:"providerConnectionRequests,omitempty"`
	// ConnectionIssueSource is owned by the server, see the ConnectionIssueSource type.
	// omitempty keeps whole-document updates from clearing it.
	ConnectionIssueSource ConnectionIssueSource `bson:"connectionIssueSource,omitempty"`
	// ConnectionIssue is owned by the server, see DetectConnectionIssue. omitempty keeps
	// whole-document updates from clearing it.
	ConnectionIssue  *ConnectionIssue `bson:"connectionIssue,omitempty"`
	RequireUniqueMrn bool             `bson:"requireUniqueMrn"`
	EHRSubscriptions EHRSubscriptions `bson:"ehrSubscriptions,omitempty"`
	Sites            *[]sites.Site    `bson:"sites,omitempty"`
	GlycemicRanges   GlycemicRanges   `bson:"glycemicRanges,omitempty"`
	DiagnosisType    *DiagnosisType   `bson:"diagnosisType,omitempty"`

	// DEPRECATED: Remove when Tidepool Web starts using provider connection requests
	LastRequestedDexcomConnectTime time.Time `bson:"lastRequestedDexcomConnectTime,omitempty"`
}

type DiagnosisType string

func (d *DiagnosisType) IsZero() bool {
	// A value of nil is Zero, but the empty string is NOT.
	//
	// This means that clients that don't supply a value will not change an existing value,
	// while those that specify an empty string will clear out the value. This is needed as
	// some (faulty, but still relevant) clients won't/don't specify a value, but in those
	// cases, we must keep the existing value.
	return d == nil
}

type GlycemicRanges struct {
	Type GlycemicRangeType `json:"type"`

	// only one of the following should be present, based on Type
	Preset GlycemicRangesPreset `bson:",omitempty"`
	Custom GlycemicRangesCustom `bson:",omitempty"`
}

var _ bsoncodec.Zeroer = (*GlycemicRanges)(nil)

// IsZero implements bsoncodec.Zeroer
func (g GlycemicRanges) IsZero() bool {
	return g.Type == "" && g.Preset.IsZero() && g.Custom.IsZero()
}

type GlycemicRangesPreset string

var _ bsoncodec.Zeroer = (*GlycemicRangesPreset)(nil)

// IsZero implements bsoncodec.Zeroer
func (g GlycemicRangesPreset) IsZero() bool {
	return string(g) == ""
}

// String implements fmt.Stringer
func (g GlycemicRangesPreset) String() string {
	return string(g)
}

type GlycemicRangesCustom struct {
	Name       string                   `bson:"name"`
	Thresholds []GlycemicRangeThreshold `bson:"thresholds"`
}

// IsZero implements bsoncodec.Zeroer
func (g GlycemicRangesCustom) IsZero() bool {
	return g.Name == "" && len(g.Thresholds) == 0
}

var _ bsoncodec.Zeroer = (*GlycemicRangesCustom)(nil)

type GlycemicRangeThreshold struct {
	Name       string         `bson:"name"`
	UpperBound ValueWithUnits `bson:"upperBound"`
	Inclusive  bool           `bson:"inclusive"`
}

type ValueWithUnits struct {
	Value float32 `bson:"value"`
	Units string  `bson:"units"`
}

func (p Patient) IsCustodial() bool {
	return p.Permissions != nil && p.Permissions.Custodian != nil
}

func (p Patient) HasEmail() bool {
	return p.Email != nil && *p.Email != ""
}

type EHRSubscriptions map[string]EHRSubscription

type EHRSubscription struct {
	Active          bool             `bson:"active"`
	Provider        string           `bson:"provider"`
	MatchedMessages []MatchedMessage `bson:"matchedMessages,omitempty"`
	CreatedAt       time.Time        `bson:"createdAt"`
	UpdatedAt       time.Time        `bson:"updatedAt"`
}

type MatchedMessage struct {
	DocumentId primitive.ObjectID `bson:"id"`
	DataModel  string             `bson:"dataModel"`
	EventType  string             `bson:"eventType"`
}

type Review struct {
	ClinicianId string    `json:"clinicianId"`
	Time        time.Time `json:"time"`
}

type LastConnectionRequests map[string]time.Time

type ProviderConnectionRequests map[string]ConnectionRequests

type ConnectionRequests []ConnectionRequest

type ConnectionRequest struct {
	ProviderName   string    `bson:"providerName"`
	CreatedTime    time.Time `bson:"createdTime"`
	ExpirationTime time.Time `bson:"expirationTime,omitempty"`
}

// ExpiresAt is the request's expiration time. Requests that predate the field expire
// PendingDataSourceExpirationDuration after creation, which is the rule the service
// applies when it creates a request.
func (r ConnectionRequest) ExpiresAt() time.Time {
	if !r.ExpirationTime.IsZero() {
		return r.ExpirationTime
	}
	return r.CreatedTime.Add(PendingDataSourceExpirationDuration)
}

type SubscriptionUpdate struct {
	Name           string
	Provider       string
	Active         bool
	MatchedMessage MatchedMessage
}

type FilterPair struct {
	Cmp   string
	Value float64
}

type FilterDatePair struct {
	Min *time.Time
	Max *time.Time
}

type SummaryFilters map[string]FilterPair

type SummaryDateFilters map[string]FilterDatePair

type Filter struct {
	ClinicIds    []string
	ClinicId     *string
	UserId       *string
	Search       *string
	Tags         *[]string
	Mrn          *string
	BirthDate    *string
	FullName     *string
	LastReviewed *time.Time
	// Sites to which the patient must be assigned to be included.
	Sites *[]string

	HasSubscription *bool
	HasMRN          *bool
	HasEmail        *bool
	IsCustodial     *bool

	Period *string

	CGM SummaryFilters
	BGM SummaryFilters

	CGMTime SummaryDateFilters
	BGMTime SummaryDateFilters

	ExcludeDemo bool
	// ExcludeSummaryExceptFieldsInMergeReports along with its helper function
	// excludeSummaryExceptFieldsInMergeReports are used to reduce a patient's [Summary] to
	// the minimum content needed to generate clinic merge reports and perform clinic
	// merges.
	ExcludeSummaryExceptFieldsInMergeReports bool

	// OmitNonStandardRanges will exclude patients that aren't assigned the ADA standard
	// preset ranges.
	OmitNonStandardRanges bool

	// ConnectionIssueCauses, when set, only includes patients whose connection issue has
	// one of the causes and is not hidden. OnlyHiddenConnectionIssues inverts the hidden
	// condition, so that only hidden issues are included, and can be used on its own.
	ConnectionIssueCauses      []ConnectionIssueCause
	OnlyHiddenConnectionIssues bool
}

type Permission = map[string]interface{}
type Permissions struct {
	Custodian *Permission `bson:"custodian,omitempty"`
	View      *Permission `bson:"view,omitempty"`
	Upload    *Permission `bson:"upload,omitempty"`
	Note      *Permission `bson:"note,omitempty"`
}

func (p *Permissions) Empty() bool {
	return p.Custodian == nil &&
		p.View == nil &&
		p.Upload == nil &&
		p.Note == nil
}

func (p *Permissions) IsClaimed() bool {
	return p != nil && p.Custodian == nil
}

type ListResult struct {
	Patients      []*Patient `bson:"data"`
	MatchingCount int        `bson:"count"`
}

type PatientUpdate struct {
	ClinicId string
	UserId   string
	Patient  Patient
}

type UploadReminderUpdate struct {
	ClinicId  string
	UserId    string
	UpdatedBy string
	Time      time.Time
}

type Summary struct {
	CGM *PatientCGMStats `json:"cgmStats" bson:"cgmStats"`
	BGM *PatientBGMStats `json:"bgmStats" bson:"bgmStats"`
}

func (s *Summary) GetLastUploadDate() time.Time {
	last := time.Time{}
	if s.CGM != nil && s.CGM.GetLastUploadDate().After(last) {
		last = s.CGM.GetLastUploadDate()
	}
	if s.BGM != nil && s.BGM.GetLastUploadDate().After(last) {
		last = s.BGM.GetLastUploadDate()
	}
	return last
}

func (s *Summary) GetLastUpdatedDate() time.Time {
	last := time.Time{}
	if s.CGM != nil && s.CGM.GetLastUpdatedDate().After(last) {
		last = s.CGM.GetLastUpdatedDate()
	}
	if s.BGM != nil && s.BGM.GetLastUpdatedDate().After(last) {
		last = s.BGM.GetLastUpdatedDate()
	}
	return last
}

type DataSources []DataSource
type DataSource struct {
	DataSourceId   *primitive.ObjectID `bson:"dataSourceId,omitempty"`
	CreatedTime    *time.Time          `bson:"createdTime,omitempty"`
	ModifiedTime   *time.Time          `bson:"modifiedTime,omitempty"`
	ProviderName   string              `bson:"providerName"`
	State          string              `bson:"state"`
	LatestDataTime *time.Time          `bson:"latestDataTime,omitempty"`
	// ConnectedTime is when the source last entered the connected state. Sources that
	// have not connected since the platform started recording it do not have one.
	ConnectedTime *time.Time `bson:"connectedTime,omitempty"`
}

// NewlyConnected returns the incoming sources that enter the connected state relative
// to the existing (stored) sources, in incoming order. A source enters the connected
// state when it is connected and was not stored before, or was stored in another state.
// Sources are matched by provider name.
func (incoming DataSources) NewlyConnected(existing DataSources) DataSources {
	if incoming == nil {
		return nil
	}

	existingByProvider := existing.byProvider()
	result := DataSources{}
	for _, ds := range incoming {
		if entersConnected(ds, existingByProvider) {
			result = append(result, ds)
		}
	}

	return result
}

// Newest returns the source most recently created, or nil when there are none. A nil
// CreatedTime sorts oldest. Ties are broken by connectionIssueProviderPriority and finally
// by position, earlier winning.
func (d DataSources) Newest() *DataSource {
	var newest *DataSource
	for i := range d {
		if newest == nil || isNewer(d[i], *newest) {
			newest = &d[i]
		}
	}
	return newest
}

// byProvider indexes the sources by provider name. The first source of a provider
// wins.
func (d DataSources) byProvider() map[string]DataSource {
	result := make(map[string]DataSource, len(d))
	for _, ds := range d {
		if _, ok := result[ds.ProviderName]; !ok {
			result[ds.ProviderName] = ds
		}
	}
	return result
}

// entersConnected reports whether ds is connected while the stored source of the same
// provider was either absent or not connected, or was connected earlier than ds. The
// latter catches a reconnection whose intermediate state was never stored. It requires
// both connected times, so a stored source gaining its first connected time, as it does
// when the platform backfills them, is not mistaken for a reconnection.
func entersConnected(ds DataSource, existingByProvider map[string]DataSource) bool {
	if ds.State != DataSourceStateConnected {
		return false
	}
	prev, wasStored := existingByProvider[ds.ProviderName]
	if !wasStored || prev.State != DataSourceStateConnected {
		return true
	}
	return ds.ConnectedTime != nil && prev.ConnectedTime != nil &&
		ds.ConnectedTime.After(*prev.ConnectedTime)
}

// isNewer reports whether a should replace b as the newest source. Equal candidates
// return false, so the earlier one is kept.
func isNewer(a, b DataSource) bool {
	switch {
	case a.CreatedTime == nil && b.CreatedTime == nil:
	// determine by provider priority below
	case a.CreatedTime == nil:
		return false
	case b.CreatedTime == nil:
		return true
	case !a.CreatedTime.Equal(*b.CreatedTime):
		return a.CreatedTime.After(*b.CreatedTime)
	}
	return providerPriority(a.ProviderName) < providerPriority(b.ProviderName)
}

// providerPriority ranks a provider by connectionIssueProviderPriority. Unknown
// providers rank last.
func providerPriority(providerName string) int {
	for i, name := range connectionIssueProviderPriority {
		if name == providerName {
			return i
		}
	}
	return len(connectionIssueProviderPriority)
}

// ConnectionIssueSource records the patient's most recently triggered connection or
// invitation flow. This flow is used to filter related issues, in turn causing the patient
// to be included or excluded from the connection issues dashboard. It is set by the server
// only, and is never accepted from API clients.
type ConnectionIssueSource string

const (
	ConnectionIssueSourceDexcom ConnectionIssueSource = "dexcom"
	ConnectionIssueSourceAbbott ConnectionIssueSource = "abbott"
	ConnectionIssueSourceTwiist ConnectionIssueSource = "twiist"
)

// ConnectionIssueSourceDeviceNonSpecificInvitation is set when a custodial patient is
// created with an email address, which triggers a device non-specific invitation, and
// when a reminder for that invitation is sent. A reminder never replaces a provider
// source that is already set.
const ConnectionIssueSourceDeviceNonSpecificInvitation ConnectionIssueSource = "deviceNonSpecificInvitation"

// ConnectionIssueSourceForProvider maps a data source or connection request provider
// name to its connection issue source. ok is false for unknown providers.
func ConnectionIssueSourceForProvider(providerName string) (
	source ConnectionIssueSource, ok bool) {

	switch providerName {
	case DexcomDataSourceProviderName:
		return ConnectionIssueSourceDexcom, true
	case AbbottDataSourceProviderName:
		return ConnectionIssueSourceAbbott, true
	case TwiistDataSourceProviderName:
		return ConnectionIssueSourceTwiist, true
	} // oura is not currently valid as a connection issue source.
	return "", false
}

type TideReportParams struct {
	Period         string
	Tags           []string
	LastDataCutoff time.Time
	Categories     []string
	ExcludeNoData  bool
}

type GlycemicRangeType string

const (
	GlycemicRangeTypePreset GlycemicRangeType = "preset"
	GlycemicRangeTypeCustom GlycemicRangeType = "custom"
)
