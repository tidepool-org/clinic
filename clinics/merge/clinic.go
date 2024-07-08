package merge

import (
	"context"
	"fmt"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/store"
	"slices"
	"time"
)

type ClinicMergePlan struct {
	Source clinics.Clinic
	Target clinics.Clinic

	MembershipRestrictionsMergePlan MembershipRestrictionsMergePlan
	SourcePatientClusters           PatientClusters
	TargetPatientClusters           PatientClusters
	SettingsPlan                    SettingsPlans
	TagsPlan                        TagPlans
	CliniciansPlan                  ClinicianPlans
	PatientsPlan                    PatientPlans

	CreatedTime time.Time
}

func (c ClinicMergePlan) PreventsMerge() bool {
	fs := []func() bool{
		c.MembershipRestrictionsMergePlan.PreventsMerge,
		c.SourcePatientClusters.PreventsMerge,
		c.TargetPatientClusters.PreventsMerge,
		c.SettingsPlan.PreventsMerge,
		c.TagsPlan.PreventsMerge,
		c.CliniciansPlan.PreventsMerge,
		c.PatientsPlan.PreventsMerge,
	}

	for _, preventsMerge := range fs {
		if preventsMerge() {
			return true
		}
	}

	return false
}

type ClinicMergePlanner struct {
	clinics    clinics.Service
	patients   patients.Service
	clinicians clinicians.Service

	sourceId string
	targetId string
}

func NewClinicMergePlanner(clinicsService clinics.Service, patientsService patients.Service, cliniciansService clinicians.Service, sourceId, targetId string) Planner[ClinicMergePlan] {
	return &ClinicMergePlanner{
		clinics:    clinicsService,
		patients:   patientsService,
		clinicians: cliniciansService,
		sourceId:   sourceId,
		targetId:   targetId,
	}
}

func (m *ClinicMergePlanner) Plan(ctx context.Context) (plan ClinicMergePlan, err error) {
	intermediate := &intermediatePlanner{}

	source, err := m.clinics.Get(ctx, m.sourceId)
	if err != nil {
		return
	}
	intermediate.SourceClinic = *source

	target, err := m.clinics.Get(ctx, m.targetId)
	if err != nil {
		return
	}
	intermediate.TargetClinic = *target

	intermediate.MembershipRestrictionsMergePlanner, err = m.MembershipRestrictionsMergePlan(*source, *target)
	if err != nil {
		return
	}
	intermediate.SettingsPlanners, err = m.SettingsMergePlan(*source, *target)
	if err != nil {
		return
	}
	intermediate.TagPlanners, err = m.TagsMergePlan(*source, *target)
	if err != nil {
		return
	}
	intermediate.ClinicianPlanners, err = m.CliniciansMergePlan(ctx, *source, *target)
	if err != nil {
		return
	}

	sourcePatients, err := m.listAllPatients(ctx, *source)
	if err != nil {
		return
	}
	targetPatients, err := m.listAllPatients(ctx, *target)
	if err != nil {
		return
	}

	intermediate.PatientPlanner, err = m.PatientsMergePlan(ctx, *source, *target, sourcePatients, targetPatients)
	if err != nil {
		return
	}
	intermediate.SourcePatientClusters = NewPatientClusterReporter(sourcePatients)
	intermediate.TargetPatientClusters = NewPatientClusterReporter(targetPatients)

	return intermediate.Plan(ctx)
}

func (m *ClinicMergePlanner) MembershipRestrictionsMergePlan(source, target clinics.Clinic) (Planner[MembershipRestrictionsMergePlan], error) {
	return NewMembershipRestrictionsMergePlanner(source, target), nil
}

func (m *ClinicMergePlanner) SettingsMergePlan(source, target clinics.Clinic) ([]Planner[SettingsPlan], error) {
	return []Planner[SettingsPlan]{
		NewSettingsReporterPlanner(source, target, GetMRNRequiredSettings, TaskTypeClinicSettingsMRNRequired),
		NewSettingsReporterPlanner(source, target, GetMRNUniqueSettings, TaskTypeClinicSettingsMRNUnique),
		NewSettingsReporterPlanner(source, target, GetGlucoseUnitsSettings, TaskTypeClinicSettingsGlucoseUnits),
		NewSettingsReporterPlanner(source, target, GetTimezoneSettings, TaskTypeClinicSettingsTimezone),
	}, nil
}

func (m *ClinicMergePlanner) TagsMergePlan(source, target clinics.Clinic) ([]Planner[TagPlan], error) {
	plans := make([]Planner[TagPlan], 0, len(source.PatientTags)+len(target.PatientTags))
	for _, tag := range source.PatientTags {
		plans = append(plans, NewSourceTagMergePlanner(tag, source, target))
	}
	for _, tag := range target.PatientTags {
		plans = append(plans, NewTargetTagMergePlanner(tag, source, target))
	}
	return plans, nil
}

func (m *ClinicMergePlanner) PatientsMergePlan(ctx context.Context, source, target clinics.Clinic, sourcePatients, targetPatients []patients.Patient) (Planner[PatientPlans], error) {
	return NewPatientMergePlanner(source, target, sourcePatients, targetPatients)
}

func (m *ClinicMergePlanner) CliniciansMergePlan(ctx context.Context, source, target clinics.Clinic) ([]Planner[ClinicianPlan], error) {
	var sourcePlan []Planner[ClinicianPlan]
	var targetPlan []Planner[ClinicianPlan]

	sourceClinicians, err := m.listAllClinicians(ctx, source)
	if err != nil {
		return nil, err
	}
	if len(sourceClinicians) > 0 {
		sourcePlan = make([]Planner[ClinicianPlan], 0, len(sourceClinicians))
		for _, clinician := range sourceClinicians {
			if clinician != nil {
				sourcePlan = append(sourcePlan, NewSourceClinicianMergePlanner(*clinician, source, target, m.clinicians))
			}
		}
	}
	targetClinicians, err := m.listAllClinicians(ctx, target)
	if err != nil {
		return nil, err
	}
	if len(targetClinicians) > 0 {
		targetPlan = make([]Planner[ClinicianPlan], 0, len(targetClinicians))
		for _, clinician := range targetClinicians {
			if clinician != nil {
				targetPlan = append(targetPlan, NewTargetClinicianMergePlanner(*clinician, source, target, m.clinicians))
			}
		}
	}

	return slices.Concat(sourcePlan, targetPlan), nil
}

func (m *ClinicMergePlanner) listAllPatients(ctx context.Context, clinic clinics.Clinic) ([]patients.Patient, error) {
	clinicId := clinic.Id.Hex()
	limit := 1000000

	page := store.DefaultPagination().WithLimit(limit)
	filter := patients.Filter{
		ClinicId: &clinicId,
	}
	result, err := m.patients.List(ctx, &filter, page, nil)
	if err != nil {
		return nil, err
	}
	if result.TotalCount > limit {
		return nil, fmt.Errorf("too many patients in clinic")
	}

	list := make([]patients.Patient, 0, len(result.Patients))
	for _, p := range result.Patients {
		list = append(list, *p)
	}

	return list, nil
}

func (m *ClinicMergePlanner) listAllClinicians(ctx context.Context, clinic clinics.Clinic) ([]*clinicians.Clinician, error) {
	clinicId := clinic.Id.Hex()
	limit := 1000000

	page := store.DefaultPagination().WithLimit(limit)
	filter := clinicians.Filter{
		ClinicId: &clinicId,
	}
	result, err := m.clinicians.List(ctx, &filter, page)
	if err != nil {
		return nil, err
	}
	if len(result) >= limit {
		return nil, fmt.Errorf("too many clinicians in clinic")
	}

	return result, nil
}

type intermediatePlanner struct {
	SourceClinic clinics.Clinic
	TargetClinic clinics.Clinic

	MembershipRestrictionsMergePlanner Planner[MembershipRestrictionsMergePlan]
	SettingsPlanners                   []Planner[SettingsPlan]
	TagPlanners                        []Planner[TagPlan]
	ClinicianPlanners                  []Planner[ClinicianPlan]

	SourcePatientClusters Planner[PatientClusters]
	TargetPatientClusters Planner[PatientClusters]
	PatientPlanner        Planner[PatientPlans]
}

func (i *intermediatePlanner) Plan(ctx context.Context) (plan ClinicMergePlan, err error) {
	plan.MembershipRestrictionsMergePlan, err = i.MembershipRestrictionsMergePlanner.Plan(ctx)
	if err != nil {
		return
	}
	plan.SourcePatientClusters, err = i.SourcePatientClusters.Plan(ctx)
	if err != nil {
		return
	}
	plan.TargetPatientClusters, err = i.TargetPatientClusters.Plan(ctx)
	if err != nil {
		return
	}
	plan.PatientsPlan, err = i.PatientPlanner.Plan(ctx)
	if err != nil {
		return
	}
	plan.SettingsPlan, err = RunPlanners(ctx, i.SettingsPlanners)
	if err != nil {
		return
	}
	plan.TagsPlan, err = RunPlanners(ctx, i.TagPlanners)
	if err != nil {
		return
	}
	plan.CliniciansPlan, err = RunPlanners(ctx, i.ClinicianPlanners)
	if err != nil {
		return
	}

	plan.Source = i.SourceClinic
	plan.Target = i.TargetClinic
	plan.CreatedTime = time.Now()
	return
}
