package merge

import (
	"context"
	"github.com/tidepool-org/clinic/clinics"
	"time"
)

type Planner struct {
	clinics clinics.Service
}

func (m *Planner) CreateMergePlan(ctx context.Context, sourceId, targetId string) (plan Plan, err error) {
	source, err := m.clinics.Get(ctx, sourceId)
	if err != nil {
		return
	}
	plan.Source = *source

	target, err := m.clinics.Get(ctx, targetId)
	if err != nil {
		return
	}
	plan.Target = *target

	plan.SettingsTasks, err = m.SettingsMergePlan(*source, *target)
	if err != nil {
		return
	}
	plan.TagTasks, err = m.TagsMergePlan(*source, *target)
	if err != nil {
		return
	}
	plan.ClinicianTasks, err = m.CliniciansMergePlan(*source, *target)
	if err != nil {
		return
	}
	plan.PatientTasks, err = m.PatientsMergePlan(*source, *target)
	if err != nil {
		return
	}

	plan.CreatedTime = time.Now()
	return
}

func (m *Planner) SettingsMergePlan(source, target clinics.Clinic) ([]Task[SettingsReportDetails], error) {
	return []Task[SettingsReportDetails]{
		NewMembershipRestrictionsMergeTask(source, target),
		NewSettingsReporterMergeTask(source, target, GetMRNRequiredSettings, TaskTypeClinicSettingsMRNRequired),
		NewSettingsReporterMergeTask(source, target, GetMRNUniqueSettings, TaskTypeClinicSettingsMRNUnique),
		NewSettingsReporterMergeTask(source, target, GetGlucoseUnitsSettings, TaskTypeClinicSettingsGlucoseUnits),
		NewSettingsReporterMergeTask(source, target, GetTimezoneSettings, TaskTypeClinicSettingsTimezone),
	}, nil
}

func (m *Planner) TagsMergePlan(source, target clinics.Clinic) ([]Task[TagReportDetails], error) {
	tasks := make([]Task[TagReportDetails], 0, len(source.PatientTags)+len(target.PatientTags))
	for _, tag := range source.PatientTags {
		tasks = append(tasks, NewSourceTagMergeTask(tag, source, target))
	}
	for _, tag := range target.PatientTags {
		tasks = append(tasks, NewTargetTagMergeTask(tag, source, target))
	}
	return []Task[TagReportDetails]{}, nil
}

func (m *Planner) PatientsMergePlan(source, target clinics.Clinic) ([]Task[any], error) {
	return []Task[any]{}, nil
}

func (m *Planner) CliniciansMergePlan(source, target clinics.Clinic) ([]Task[ClinicianReportDetails], error) {
	return []Task[ClinicianReportDetails]{}, nil
}
