package manager

import (
	"context"
	errs "errors"
	"fmt"

	"github.com/tidepool-org/clinic/deletions"

	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"

	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/store"
)

const (
	duplicateShareCodeRetryAttempts = 100
)

type CreateClinic struct {
	Clinic            clinics.Clinic
	CreatorUserId     string
	CreateDemoPatient bool
}

type Manager interface {
	CreateClinic(ctx context.Context, create *CreateClinic) (*clinics.Clinic, error)
	DeleteClinic(ctx context.Context, clinicId string, metadata deletions.Metadata) error
	GetClinicPatientCount(ctx context.Context, clinicId string) (*clinics.PatientCount, error)
	FinalizeMerge(ctx context.Context, sourceId, targetId string) error
	// CreateSite within a clinic.
	//
	// CreateSite is expected to return an error if a site with the given name already
	// exists, or if creating a new site would exceed the maximum number of sites.
	CreateSite(_ context.Context, clinicId string, name string) (*sites.Site, error)
	// DeleteSite within a clinic.
	//
	// The Site should be removed from the clinic and any patient records that include the
	// site.
	DeleteSite(_ context.Context, clinicId string, siteId string) error
	// MergeSite combines the patients from two sites into the target, deleting the source.
	MergeSite(_ context.Context, clinicId, sourceSiteId, targetSiteId string) (*sites.Site, error)
	// UpdateSite within a clinic.
	//
	// Sites are denormalized over the clinics and patients collections. This function
	// should handle maintaining that denormalization.
	UpdateSite(_ context.Context, clinicId, siteId string, site *sites.Site) (*sites.Site, error)
	// ConvertPatientTagToSite within a clinic.
	//
	// Useful after clinic merges for example. Or when clinics used a given tag to denote a
	// site, before the introduction of sites.
	ConvertPatientTagToSite(_ context.Context, clinicId, patientTagId string) (*sites.Site, error)
}

type manager struct {
	clinics              clinics.Service
	cliniciansRepository *clinicians.Repository
	config               *config.Config
	dbClient             *mongo.Client
	patientsRepository   patients.Repository
	patientsService      patients.Service
	shareCodeGenerator   clinics.ShareCodeGenerator
	userService          patients.UserService
}

type Params struct {
	fx.In

	Clinics              clinics.Service
	CliniciansRepository *clinicians.Repository
	Config               *config.Config
	DbClient             *mongo.Client
	PatientsRepository   patients.Repository
	PatientsService      patients.Service
	ShareCodeGenerator   clinics.ShareCodeGenerator
	UserService          patients.UserService
}

func NewManager(cp Params) (Manager, error) {
	return &manager{
		clinics:              cp.Clinics,
		cliniciansRepository: cp.CliniciansRepository,
		config:               cp.Config,
		dbClient:             cp.DbClient,
		patientsRepository:   cp.PatientsRepository,
		patientsService:      cp.PatientsService,
		shareCodeGenerator:   cp.ShareCodeGenerator,
		userService:          cp.UserService,
	}, nil
}

func (c *manager) CreateClinic(ctx context.Context, create *CreateClinic) (*clinics.Clinic, error) {
	user, err := c.userService.GetUser(create.CreatorUserId)
	if err != nil {
		return nil, err
	}
	if user == nil {
		return nil, fmt.Errorf("unable to find user with id %v", create.CreatorUserId)
	}

	profile, err := c.userService.GetUserProfile(ctx, create.CreatorUserId)
	if err != nil {
		return nil, fmt.Errorf("error fetching user profile of clinician %v", create.CreatorUserId)
	}

	var demoPatient *patients.Patient
	if create.CreateDemoPatient {
		demoPatient, err = c.getDemoPatient(ctx)
		if err != nil {
			return nil, err
		}
	}

	transaction := func(sessionCtx mongo.SessionContext) (any, error) {
		// Set initial admins
		create.Clinic.AddAdmin(create.CreatorUserId)

		// Add the clinic to the collection
		clinic, err := c.createClinicObject(sessionCtx, create)
		if err != nil {
			return nil, err
		}

		// Add the clinician to the collection
		clinician := &clinicians.Clinician{
			ClinicId: clinic.Id,
			UserId:   &create.CreatorUserId,
			Roles:    []string{clinicians.RoleClinicAdmin},
			Email:    &user.Emails[0],
		}
		if profile != nil {
			clinician.Name = profile.FullName
		}
		if _, err = c.cliniciansRepository.Create(sessionCtx, clinician); err != nil {
			return nil, err
		}

		// Add the demo patient account
		if demoPatient != nil {
			demoPatient.ClinicId = clinic.Id
			if _, err = c.patientsService.Create(sessionCtx, *demoPatient); err != nil {
				return nil, err
			}
		}

		return clinic, nil
	}

	result, err := store.WithTransaction(ctx, c.dbClient, transaction)
	if err != nil {
		return nil, err
	}

	return result.(*clinics.Clinic), nil
}

func (c *manager) DeleteClinic(ctx context.Context, clinicId string, metadata deletions.Metadata) error {
	transaction := func(sessionCtx mongo.SessionContext) (any, error) {
		return nil, c.deleteClinic(sessionCtx, clinicId, metadata)
	}

	_, err := store.WithTransaction(ctx, c.dbClient, transaction)
	return err
}

func (c *manager) FinalizeMerge(ctx context.Context, sourceId, targetId string) error {
	source, err := c.clinics.Get(ctx, sourceId)
	if err != nil {
		return err
	}

	// Delete clinics if allowed by patient list
	err = c.deleteClinic(ctx, sourceId, deletions.Metadata{})
	if err != nil {
		return err
	}

	// Append share codes of source clinic
	if source.ShareCodes != nil {
		err = c.clinics.AppendShareCodes(ctx, targetId, *source.ShareCodes)
		if err != nil {
			return err
		}
	}

	// Refresh patient count of target clinic
	return c.refreshPatientCount(ctx, targetId)
}

func (c *manager) GetClinicPatientCount(ctx context.Context, clinicId string) (*clinics.PatientCount, error) {
	patientCount, err := c.clinics.GetPatientCount(ctx, clinicId)
	if err != nil {
		return nil, err
	}

	if patientCount == nil {
		count, err := c.patientsService.Count(ctx, &patients.Filter{ClinicId: &clinicId, ExcludeDemo: true})
		if err != nil {
			return nil, err
		}

		patientCount = &clinics.PatientCount{PatientCount: count}
		if err := c.clinics.UpdatePatientCount(ctx, clinicId, patientCount); err != nil {
			return nil, err
		}
	}

	return patientCount, nil
}

func (c *manager) refreshPatientCount(ctx context.Context, clinicId string) error {
	count, err := c.patientsService.Count(ctx, &patients.Filter{ClinicId: &clinicId, ExcludeDemo: true})
	if err != nil {
		return err
	}

	patientCount := &clinics.PatientCount{PatientCount: count}
	return c.clinics.UpdatePatientCount(ctx, clinicId, patientCount)
}

// Creates a clinic document in mongo and retries if there is a violation of the unique share code constraint
func (c *manager) createClinicObject(sessionCtx mongo.SessionContext, create *CreateClinic) (clinic *clinics.Clinic, err error) {
retryLoop:
	for i := 0; i < duplicateShareCodeRetryAttempts; i++ {
		shareCode := c.shareCodeGenerator.Generate()
		shareCodes := []string{shareCode}
		create.Clinic.CanonicalShareCode = &shareCode
		create.Clinic.ShareCodes = &shareCodes

		clinic, err = c.clinics.Create(sessionCtx, &create.Clinic)
		if err == nil || !errs.Is(err, clinics.ErrDuplicateShareCode) {
			break retryLoop
		}
	}
	return clinic, err
}

func (c *manager) getDemoPatient(ctx context.Context) (*patients.Patient, error) {
	if c.config.ClinicDemoPatientUserId == "" {
		return nil, nil
	}

	perm := make(patients.Permission)
	patient := &patients.Patient{
		UserId:     &c.config.ClinicDemoPatientUserId,
		IsMigrated: true, // Do not send emails
		Permissions: &patients.Permissions{
			View: &perm,
		},
	}
	if err := c.userService.PopulatePatientDetailsFromExistingUser(ctx, patient); err != nil {
		return nil, err
	}
	return patient, nil
}

func (c *manager) deleteClinic(ctx context.Context, clinicId string, metadata deletions.Metadata) error {
	filter := patients.Filter{ClinicId: &clinicId}
	pagination := store.Pagination{Limit: 2}
	res, err := c.patientsService.List(ctx, &filter, pagination, nil)

	if err != nil {
		return err
	}
	if res == nil {
		return fmt.Errorf("patient list result not defined")
	}
	if !c.patientListAllowsClinicDeletion(res.Patients) {
		return fmt.Errorf("%w: deletion of non-empty clinics is not allowed", errors.BadRequest)
	}

	// Using the repository directly, because the service wraps deletes in transaction
	if err := c.patientsRepository.Remove(ctx, clinicId, c.config.ClinicDemoPatientUserId, metadata); err != nil && !errs.Is(err, errors.NotFound) {
		return err
	}
	if err := c.cliniciansRepository.DeleteAll(ctx, clinicId, metadata); err != nil {
		return err
	}

	return c.clinics.Delete(ctx, clinicId, metadata)
}

func (c *manager) patientListAllowsClinicDeletion(list []*patients.Patient) bool {
	// No patients, OK to delete
	if len(list) == 0 {
		return true
	}
	// Only demo patients, OK to delete
	if len(list) == 1 && list[0] != nil && list[0].UserId != nil && *list[0].UserId == c.config.ClinicDemoPatientUserId {
		return true
	}
	return false
}

// CreateSite implements [Manager].
func (c *manager) CreateSite(ctx context.Context, clinicId, name string) (
	*sites.Site, error) {

	site, err := c.clinics.CreateSite(ctx, clinicId, sites.New(name))
	if err != nil {
		return nil, err
	}
	return site, nil
}

// DeleteSite implements [Manager].
func (c *manager) DeleteSite(ctx context.Context, clinicId, siteId string) error {
	tx := func(sessionCtx mongo.SessionContext) (any, error) {
		return nil, c.deleteSite(sessionCtx, clinicId, siteId)
	}
	_, err := store.WithTransaction(ctx, c.dbClient, tx)
	if err != nil {
		return err
	}
	return nil
}

// deleteSite and ripple the changes to a clinic's patients.
//
// This should be run in a transaction to prevent races.
func (c *manager) deleteSite(ctx context.Context, clinicId, siteId string) error {
	if err := c.patientsService.DeleteSites(ctx, clinicId, siteId); err != nil {
		return err
	}
	if err := c.clinics.DeleteSite(ctx, clinicId, siteId); err != nil {
		return err
	}
	return nil
}

func (c *manager) ConvertPatientTagToSite(ctx context.Context,
	clinicId, patientTagId string) (*sites.Site, error) {

	clinic, err := c.clinics.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}
	var tag *clinics.PatientTag
	for _, clinicTag := range clinic.PatientTags {
		if clinicTag.Id.Hex() == patientTagId {
			tag = &clinicTag
			break
		}
	}
	if tag == nil {
		return nil, fmt.Errorf("unable to find patient tag: %w", errors.NotFound)
	}

	siteName, err := sites.MaybeRenameSite(sites.Site{Name: tag.Name}, clinic.Sites)
	if err != nil {
		return nil, err
	}

	site, err := c.CreateSite(ctx, clinicId, siteName)
	if err != nil {
		if errs.Is(err, clinics.ErrMaximumSitesExceeded) {
			return nil, errors.Conflict
		}
		if errs.Is(err, clinics.ErrDuplicateSiteName) {
			return nil, errors.Conflict
		}
		return nil, err
	}

	err = c.patientsService.ConvertPatientTagToSite(ctx, clinicId, patientTagId, site)
	if err != nil {
		return nil, err
	}

	err = c.clinics.DeletePatientTag(ctx, clinicId, tag.Id.Hex())
	if err != nil {
		return nil, err
	}

	return site, nil
}

func (c *manager) MergeSite(ctx context.Context,
	clinicId, sourceSiteId, targetSiteId string) (*sites.Site, error) {

	if sourceSiteId == targetSiteId {
		return nil, fmt.Errorf("can't merge a site into itself: %w", errors.BadRequest)
	}

	clinic, err := c.clinics.Get(ctx, clinicId)
	if err != nil {
		return nil, err
	}
	var targetSite *sites.Site
	for _, site := range clinic.Sites {
		if site.Id.Hex() == targetSiteId {
			targetSite = &site
			break
		}
	}
	if targetSite == nil {
		return nil, errors.NotFound
	}
	err = c.patientsService.MergeSites(ctx, clinicId, sourceSiteId, targetSite)
	if err != nil {
		return nil, err
	}
	err = c.clinics.DeleteSite(ctx, clinicId, sourceSiteId)
	if err != nil {
		return nil, err
	}
	return targetSite, nil
}

// UpdateSite implements [Manager].
func (c *manager) UpdateSite(ctx context.Context,
	clinicId, siteId string, site *sites.Site) (*sites.Site, error) {

	tx := func(sessionCtx mongo.SessionContext) (any, error) {
		return c.updateSite(sessionCtx, clinicId, siteId, site)
	}
	updated, err := store.WithTransaction(ctx, c.dbClient, tx)
	if err != nil {
		return nil, err
	}
	site, ok := updated.(*sites.Site)
	if !ok {
		return nil, fmt.Errorf("expected a *sites.Site")
	}
	return site, nil
}

// updateSite and ripple the changes to a clinic's patients.
//
// This should be run in a transaction to prevent races.
func (c *manager) updateSite(ctx context.Context,
	clinicId, siteId string, site *sites.Site) (*sites.Site, error) {

	updated, err := c.clinics.UpdateSite(ctx, clinicId, siteId, site)
	if err != nil {
		return nil, err
	}
	if err := c.patientsService.UpdateSites(ctx, clinicId, siteId, site); err != nil {
		if errs.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.NotFound
		}
		return nil, err
	}
	return updated, nil
}
