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

	PendingDataSourceExpirationDuration = time.Hour * 24 * 30
	// PendingDataSourceStaleDuration is how long a provider connection request may go
	// unaccepted before the patient's primary issue is classified as a stale invitation.
	PendingDataSourceStaleDuration = time.Hour * 48
	// DataSourceStaleDataDuration is how long a connected data source may go without new
	// data before the patient's primary issue is classified as stale data.
	DataSourceStaleDataDuration = time.Hour * 48
	// DeviceNonSpecificInviteExpirationDuration is how long the invitation to claim a
	// custodial account stays valid. It mirrors hydrophone's signup confirmation timeout;
	// the automated resend after a week regenerates the token without extending it.
	DeviceNonSpecificInviteExpirationDuration = time.Hour * 24 * 31

	DexcomDataSourceProviderName = "dexcom"
	TwiistDataSourceProviderName = "twiist"
	AbbottDataSourceProviderName = "abbott"

	// DataSourceProviderNames lists the third-party providers a patient can connect.
	DataSourceProviderNames = []string{
		DexcomDataSourceProviderName,
		TwiistDataSourceProviderName,
		AbbottDataSourceProviderName,
	}

	DataSourceStateConnected    = "connected"
	DataSourceStateDisconnected = "disconnected"
	DataSourceStateError        = "error"

	// PrimaryIssueSourceDeviceNonSpecificInvite is the primary issue source recorded when
	// the patient's outstanding issue is the invitation to claim the account rather than a
	// specific device. The other sources are the provider names above.
	PrimaryIssueSourceDeviceNonSpecificInvite = "deviceNonSpecificInvite"

	// PrimaryIssueKind* classify a patient's primary issue. They are set by backend
	// services once the issue has been evaluated; an empty Kind means the issue has not
	// been classified yet.
	PrimaryIssueKindErroring      = "erroring"
	PrimaryIssueKindDisconnected  = "disconnected"
	PrimaryIssueKindExpiredInvite = "expiredInvite"
	PrimaryIssueKindStaleData     = "staleData"
	PrimaryIssueKindStaleInvite   = "staleInvite"

	// PrimaryIssueSourcePrecedence orders sources from lowest to highest priority. It only
	// breaks ties between events that share an effective time; a newer event wins regardless
	// of source, so a re-sent invitation replaces an older device event. On an exact tie the
	// device-non-specific invitation yields to any device.
	PrimaryIssueSourcePrecedence = []string{
		PrimaryIssueSourceDeviceNonSpecificInvite, // lowest priority
		AbbottDataSourceProviderName,
		DexcomDataSourceProviderName,
		TwiistDataSourceProviderName, // highest priority
	}

	permission                  = make(Permission, 0)
	CustodialAccountPermissions = Permissions{
		Custodian: &permission,
		View:      &permission,
		Upload:    &permission,
		Note:      &permission,
	}
)

// PrimaryIssueKindForDataSourceState returns the primary issue kind that describes a data
// source in the given state, and whether the state describes a failure at all.
func PrimaryIssueKindForDataSourceState(state string) (string, bool) {
	switch state {
	case DataSourceStateDisconnected:
		return PrimaryIssueKindDisconnected, true
	case DataSourceStateError:
		return PrimaryIssueKindErroring, true
	default:
		return "", false
	}
}

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
	AddProviderConnectionRequest(ctx context.Context, clinicId, userId string, request ConnectionRequest) error
	MarkInvitationResent(ctx context.Context, clinicId, userId string) error
	UpdateDeviceIssues(ctx context.Context) error
	AssignPatientTagToClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error
	DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error
	ConvertPatientTagToSite(ctx context.Context, clinicId, patientTagId string, site *sites.Site) error
	UpdatePatientDataSources(ctx context.Context, userId string, dataSources *DataSources) error
	TideReport(ctx context.Context, clinicId string, params TideReportParams) (*Tide, error)
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
	ProviderConnectionRequests ProviderConnectionRequests `bson:"providerConnectionRequests,omitempty"`
	RequireUniqueMrn           bool                       `bson:"requireUniqueMrn"`
	EHRSubscriptions           EHRSubscriptions           `bson:"ehrSubscriptions,omitempty"`
	Sites                      *[]sites.Site              `bson:"sites,omitempty"`
	GlycemicRanges             GlycemicRanges             `bson:"glycemicRanges,omitempty"`
	DiagnosisType              *DiagnosisType             `bson:"diagnosisType,omitempty"`
	// PrimaryIssue tracks the source of the most recent connection issue (if any).
	//
	// Only certain events trigger a change in the primary issue. Later issues that are
	// detected with a connection can be automatically suppressed, if they don't match the
	// primary issue. This prevents, for example, an old device having stale data from
	// becoming a connection issue when a newer device is present and connected.
	//
	// Its value should only be set by the backend and is read-only from the API.
	PrimaryIssue *PrimaryIssue `bson:"primaryIssue,omitempty"`

	// DEPRECATED: Remove when Tidepool Web starts using provider connection requests
	LastRequestedDexcomConnectTime time.Time `bson:"lastRequestedDexcomConnectTime,omitempty"`
}

// PrimaryIssue records what the patient's primary connection issue relates to and when
// that became the case.
type PrimaryIssue struct {
	// Source of the issue. It can be a provider name, or the device non-specific invitation
	// sent to custodial patients.
	Source string `bson:"source"`
	// Kind classifies the issue.
	//
	// It is set only by backend services, after they have evaluated the issue, and is empty
	// until then. Every event that replaces the primary issue starts over with an empty
	// Kind.
	Kind string `bson:"kind,omitempty"`
	// EffectiveTime is denormalized from the event that set the source: the createdTime of
	// the connection request, the modifiedTime of the data source at the time it was
	// connected, or the time the invitation was sent.
	EffectiveTime time.Time `bson:"effectiveTime"`
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
