package xealth

import (
	"context"
	"fmt"
	"github.com/oapi-codegen/runtime/types"
	"github.com/tidepool-org/clinic/clinics"
	errs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"github.com/tidepool-org/clinic/xealth_client"
	"strings"
	"time"
)

var (
	NoMatchingPatients  = fmt.Errorf("%w: couldn't find matching patient", errs.NotFound)
	NoClinicsErr        = fmt.Errorf("%w: couldn't find matching clinic", errs.NotFound)
	MultipleClinicsErr  = fmt.Errorf("%w: found multiple matching clinics", errs.Duplicate)
	MultiplePatientsErr = fmt.Errorf("%w: multiple matching patients found", errs.ConstraintViolation)
)

type Response interface {
	*xealth_client.PreorderFormResponse |
		*xealth_client.GetProgramsResponse |
		*xealth_client.GetProgramUrlResponse |
		*xealth_client.EventNotificationResponse
}

type Matcher[R Response] struct {
	deploymentId string

	datasets        *xealth_client.GeneralDatasets
	order           *xealth_client.ReadOrderResponse
	patientIdentity xealth_client.PatientIdentity

	clinics  clinics.Service
	patients patients.Service

	noClinicsResp  R
	noClinicsErr   error
	noPatientsResp R
	noPatientsErr  error

	multipleClinicsResp  R
	multipleClinicsErr   error
	multiplePatientsResp R
	multiplePatientsErr  error
}

type MatchingResult[R Response] struct {
	Clinic   *clinics.Clinic
	Patient  *patients.Patient
	Criteria *PatientMatchingCriteria

	Response R
}

func NewMatcher[R Response](clinics clinics.Service, patients patients.Service) *Matcher[R] {
	return &Matcher[R]{
		clinics:  clinics,
		patients: patients,

		noClinicsErr:        NoClinicsErr,
		noPatientsErr:       NoMatchingPatients,
		multipleClinicsErr:  MultipleClinicsErr,
		multiplePatientsErr: MultiplePatientsErr,
	}
}

func (m *Matcher[R]) FromProgramsRequest(event xealth_client.GetProgramsRequest) *Matcher[R] {
	m.deploymentId = event.Deployment
	m.patientIdentity = event.PatientIdentity
	m.datasets = event.Datasets
	return m
}

func (m *Matcher[R]) FromProgramUrlRequest(event xealth_client.GetProgramUrlRequest) *Matcher[R] {
	m.deploymentId = event.Deployment
	m.patientIdentity = event.PatientIdentity
	m.datasets = event.Datasets
	return m
}

func (m *Matcher[R]) FromEventNotification(event xealth_client.EventNotification) *Matcher[R] {
	m.deploymentId = event.Deployment
	m.patientIdentity = xealth_client.PatientIdentity{}
	for _, identity := range event.PatientIdentity.Ids {
		AppendPatientId(&m.patientIdentity, identity.Id, string(identity.Origin), identity.Type)
	}
	if event.PatientIdentity.HistoricalIds != nil {
		for _, identity := range *event.PatientIdentity.HistoricalIds {
			AppendPatientHistoricalId(&m.patientIdentity, identity.Id, string(identity.Origin), identity.Type)
		}
	}
	return m
}

func (m *Matcher[R]) FromInitialPreorderForRequest(event xealth_client.PreorderFormRequest0) *Matcher[R] {
	m.deploymentId = event.Deployment
	m.patientIdentity = event.PatientIdentity
	m.datasets = event.Datasets
	return m
}

func (m *Matcher[R]) FromSubsequentPreorderForRequest(event xealth_client.PreorderFormRequest1) *Matcher[R] {
	m.deploymentId = event.Deployment
	m.patientIdentity = event.PatientIdentity
	m.datasets = event.Datasets
	return m
}

func (m *Matcher[R]) FromOrder(event OrderEvent) *Matcher[R] {
	m.deploymentId = event.OrderData.OrderInfo.Deployment
	m.order = &event.OrderData
	m.patientIdentity = xealth_client.PatientIdentity{}
	for _, identity := range event.OrderData.PatientIdentity.Ids {
		AppendPatientId(&m.patientIdentity, identity.Id, string(identity.Origin), identity.Type)
	}
	if event.OrderData.PatientIdentity.HistoricalIds != nil {
		for _, identity := range *event.OrderData.PatientIdentity.HistoricalIds {
			AppendPatientHistoricalId(&m.patientIdentity, identity.Id, string(identity.Origin), identity.Type)
		}
	}
	return m
}

func (m *Matcher[R]) OnNoMatchingClinicsRespondWith(response R) *Matcher[R] {
	m.noClinicsResp = response
	m.noClinicsErr = nil
	return m
}

func (m *Matcher[R]) OnMultipleMatchingClinicsRespondWith(response R) *Matcher[R] {
	m.multipleClinicsResp = response
	m.multipleClinicsErr = nil
	return m
}

func (m *Matcher[R]) OnMultipleMatchingPatientsRespondWith(response R) *Matcher[R] {
	m.multiplePatientsResp = response
	m.multiplePatientsErr = nil
	return m
}

func (m *Matcher[R]) OnNoMatchingPatientsRespondWith(response R) *Matcher[R] {
	m.noPatientsResp = response
	m.noPatientsErr = nil
	return m
}

func (m *Matcher[R]) DisableErrorOnNoMatchingClinics() *Matcher[R] {
	m.noClinicsErr = nil
	return m
}

func (m *Matcher[R]) DisableErrorOnMultipleMatchingClinics() *Matcher[R] {
	m.multipleClinicsErr = nil
	return m
}

func (m *Matcher[R]) DisableErrorOnMultipleMatchingPatients() *Matcher[R] {
	m.multiplePatientsErr = nil
	return m
}

func (m *Matcher[R]) DisableErrorOnNoMatchingPatients() *Matcher[R] {
	m.noPatientsErr = nil
	return m
}

func (m *Matcher[R]) Match(ctx context.Context) (result MatchingResult[R], err error) {
	matchingClinics, err := m.matchClinics(ctx, m.deploymentId)
	if err != nil {
		return
	}

	clinicsCount := len(matchingClinics)
	if clinicsCount == 0 {
		result.Response = m.noClinicsResp
		err = m.noClinicsErr
		return
	} else if clinicsCount > 1 {
		result.Response = m.multipleClinicsResp
		err = m.multipleClinicsErr
		return
	} else {
		result.Clinic = matchingClinics[0]
	}

	if m.datasets != nil {
		result.Criteria, err = NewPatientMatchingCriteriaFromGeneralDatasets(m.datasets)
		if err != nil {
			return
		}
	} else if m.order != nil {
		result.Criteria, err = NewPatientMatchingCriteriaFromOrder(m.order)
		if err != nil {
			return
		}
	} else {
		return
	}

	if result.Criteria != nil {
		err = PopulateMRN(result.Criteria, m.patientIdentity, result.Clinic)
		if err != nil {
			return
		}

		err = result.Criteria.Validate()
		if err != nil {
			return
		}
	}

	matchingPatients, err := m.FindMatchingPatients(ctx, result.Criteria, result.Clinic)
	if err != nil {
		return
	}

	patientsCount := len(matchingPatients)
	if patientsCount == 0 {
		result.Response = m.noPatientsResp
		err = m.noPatientsErr
		return
	} else if patientsCount > 1 {
		result.Response = m.multiplePatientsResp
		err = m.multiplePatientsErr
		return
	} else {
		result.Patient = matchingPatients[0]
	}

	return
}

func (m *Matcher[R]) matchClinics(ctx context.Context, deployment string) ([]*clinics.Clinic, error) {
	enabled := true
	filter := &clinics.Filter{
		EHRProvider: &clinics.EHRProviderXealth,
		EHRSourceId: &deployment,
		EHREnabled:  &enabled,
	}
	page := store.Pagination{
		Offset: 0,
		Limit:  2,
	}

	return m.clinics.List(ctx, filter, page)
}

func (m *Matcher[R]) FindMatchingPatients(ctx context.Context, criteria *PatientMatchingCriteria, clinic *clinics.Clinic) ([]*patients.Patient, error) {
	clinicId := clinic.Id.Hex()
	page := store.Pagination{
		Offset: 0,
		Limit:  100,
	}

	filter := patients.Filter{
		ClinicId: &clinicId,
		Mrn:      &criteria.Mrn,
	}
	result, err := m.patients.List(ctx, &filter, page, nil)
	if err != nil {
		return nil, err
	}

	return result.Patients, nil
}

type PatientMatchingCriteria struct {
	FirstName   string
	LastName    string
	FullName    string
	Mrn         string
	DateOfBirth string
	Email       string
}

func (p *PatientMatchingCriteria) IsPatientUnder13() bool {
	dob, err := time.Parse(types.DateFormat, p.DateOfBirth)
	if err != nil {
		return false
	}
	return dob.AddDate(13, 0, 0).After(time.Now())
}

func (p *PatientMatchingCriteria) Validate() error {
	if p.Mrn == "" {
		return fmt.Errorf("%w: mrn is missing", errs.BadRequest)
	}
	if p.DateOfBirth == "" {
		return fmt.Errorf("%w: date of birth is missing", errs.BadRequest)
	}
	if p.FullName == "" {
		return fmt.Errorf("%w: full name is missing", errs.BadRequest)
	}
	return nil
}

func PopulateMRN(criteria *PatientMatchingCriteria, identity xealth_client.PatientIdentity, clinic *clinics.Clinic) error {
	if clinic == nil || clinic.EHRSettings == nil {
		return fmt.Errorf("%w: clinic has no EHR settings", errs.BadRequest)
	}
	mrnIdType := strings.ToLower(clinic.EHRSettings.GetMrnIDType())
	for _, identifier := range identity.Ids {
		if strings.ToLower(identifier.Type) == mrnIdType {
			criteria.Mrn = identifier.Id
			break
		}
	}
	return nil
}

func NewPatientMatchingCriteriaFromOrder(order *xealth_client.ReadOrderResponse) (*PatientMatchingCriteria, error) {
	if order.Datasets == nil {
		return nil, fmt.Errorf("%w: datasets is required", errs.BadRequest)
	}
	datasets := order.Datasets
	if datasets.DemographicsV1 == nil {
		return nil, fmt.Errorf("%w: demographics is required", errs.BadRequest)
	}

	criteria := &PatientMatchingCriteria{}

	if datasets.DemographicsV1.Name != nil {
		names := make([]string, 0, 2)
		if datasets.DemographicsV1.Name.Given != nil && len(*datasets.DemographicsV1.Name.Given) > 0 {
			criteria.FirstName = strings.Join(*datasets.DemographicsV1.Name.Given, " ")
			names = append(names, criteria.FirstName)
		}
		if datasets.DemographicsV1.Name.Family != nil && len(*datasets.DemographicsV1.Name.Family) > 0 {
			criteria.LastName = strings.Join(*datasets.DemographicsV1.Name.Family, " ")
			names = append(names, criteria.LastName)
		}
		if len(names) > 0 {
			criteria.FullName = strings.TrimSpace(strings.Join(names, " "))
		}
	}

	if datasets.DemographicsV1.BirthDate != nil {
		criteria.DateOfBirth = datasets.DemographicsV1.BirthDate.String()
	}

	if datasets.DemographicsV1.Telecom != nil {
		for _, v := range *datasets.DemographicsV1.Telecom {
			if v.System != nil && *v.System == xealth_client.ReadOrderResponseDatasetsDemographicsV1TelecomSystemEmail && v.Value != nil {
				criteria.Email = *v.Value
				break
			}
		}
	}

	return criteria, nil
}

func NewPatientMatchingCriteriaFromGeneralDatasets(datasets *xealth_client.GeneralDatasets) (*PatientMatchingCriteria, error) {
	if datasets == nil {
		return nil, fmt.Errorf("%w: datasets is required", errs.BadRequest)
	}
	if datasets.DemographicsV1 == nil {
		return nil, fmt.Errorf("%w: demographics is required", errs.BadRequest)
	}

	criteria := &PatientMatchingCriteria{}

	if datasets.DemographicsV1.Name != nil {
		names := make([]string, 0, 2)
		if datasets.DemographicsV1.Name.Given != nil && len(*datasets.DemographicsV1.Name.Given) > 0 {
			criteria.FirstName = strings.Join(*datasets.DemographicsV1.Name.Given, " ")
			names = append(names, criteria.FirstName)
		}
		if datasets.DemographicsV1.Name.Family != nil && len(*datasets.DemographicsV1.Name.Family) > 0 {
			criteria.LastName = strings.Join(*datasets.DemographicsV1.Name.Family, " ")
			names = append(names, criteria.LastName)
		}
		if len(names) > 0 {
			criteria.FullName = strings.TrimSpace(strings.Join(names, " "))
		}
	}

	if datasets.DemographicsV1.BirthDate != nil {
		criteria.DateOfBirth = datasets.DemographicsV1.BirthDate.String()
	}

	if datasets.DemographicsV1.Telecom != nil {
		for _, v := range *datasets.DemographicsV1.Telecom {
			if v.System != nil && *v.System == xealth_client.GeneralDatasetsDemographicsV1TelecomSystemEmail && v.Value != nil {
				criteria.Email = *v.Value
				break
			}
		}
	}

	return criteria, nil
}

func AppendPatientId(patientIdentity *xealth_client.PatientIdentity, id, origin, typ string) {
	patientIdentity.Ids = append(patientIdentity.Ids, struct {
		Id     string                                 `json:"id"`
		Origin xealth_client.PatientIdentityIdsOrigin `json:"origin"`
		Type   string                                 `json:"type"`
	}{
		Id:     id,
		Origin: xealth_client.PatientIdentityIdsOrigin(origin),
		Type:   typ,
	})
}

func AppendPatientHistoricalId(patientIdentity *xealth_client.PatientIdentity, id, origin, typ string) {
	ids := []struct {
		Id     string                                           `json:"id"`
		Origin xealth_client.PatientIdentityHistoricalIdsOrigin `json:"origin"`
		Type   string                                           `json:"type"`
	}{{
		Id:     id,
		Origin: xealth_client.PatientIdentityHistoricalIdsOrigin(origin),
		Type:   typ,
	}}
	if patientIdentity.HistoricalIds != nil {
		historicalIds := *patientIdentity.HistoricalIds
		historicalIds = append(historicalIds, ids[0])
		patientIdentity.HistoricalIds = &historicalIds
	} else {
		patientIdentity.HistoricalIds = &ids
	}
}
