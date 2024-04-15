package merge

import (
	"context"
	"github.com/tidepool-org/clinic/clinics"
	"sort"
	"strings"
)

const (
	TaskTypeClinicSettingsHasPartialSSO = "Partial SSO*"
	TaskTypeClinicSettingsMRNRequired   = "MRN Required"
	TaskTypeClinicSettingsMRNUnique     = "MRN Unique"
	TaskTypeClinicSettingsGlucoseUnits  = "Glucose Units"
	TaskTypeClinicSettingsTimezone      = "Timezone"
)

type ClinicPropertyGetter func(clinics.Clinic) string

type SettingsReportDetails struct {
	Name        string `bson:"name"`
	SourceValue string `bson:"sourceValue"`
	TargetValue string `bson:"targetValue"`
	ValuesMatch bool   `bson:"valuesMatch"`
}

type SettingsReporterMergeTask struct {
	getter ClinicPropertyGetter
	source clinics.Clinic
	target clinics.Clinic
	typ    string

	result TaskResult[SettingsReportDetails]
	err    error
}

func NewSettingsReporterMergeTask(source, target clinics.Clinic, getter ClinicPropertyGetter, typ string) Task[SettingsReportDetails] {
	return &SettingsReporterMergeTask{
		getter: getter,
		source: source,
		target: target,
		typ:    typ,
	}
}

func (d *SettingsReporterMergeTask) CanRun() bool {
	return d.ValuesMatch()
}

func (d *SettingsReporterMergeTask) DryRun(ctx context.Context) error {
	d.result = TaskResult[SettingsReportDetails]{
		ReportDetails: d.getReportDetails(),
		PreventsMerge: !d.CanRun(),
	}
	return nil
}

func (d *SettingsReporterMergeTask) Run(ctx context.Context) error {
	// Noop, it's just for reporting purposes
	return d.DryRun(ctx)
}

func (d *SettingsReporterMergeTask) GetResult() (TaskResult[SettingsReportDetails], error) {
	return d.result, d.err
}

func (d *SettingsReporterMergeTask) getReportDetails() SettingsReportDetails {
	return SettingsReportDetails{
		Name:        d.GetType(),
		SourceValue: d.GetSourceValue(),
		TargetValue: d.GetTargetValue(),
		ValuesMatch: d.ValuesMatch(),
	}
}

func (d *SettingsReporterMergeTask) getClinicName(clinic clinics.Clinic) (val string) {
	if clinic.Name != nil {
		val = *clinic.Name
	}
	return
}

func (d *SettingsReporterMergeTask) GetSourceValue() string {
	return d.getter(d.source)
}

func (d *SettingsReporterMergeTask) GetTargetValue() string {
	return d.getter(d.target)
}

func (d *SettingsReporterMergeTask) GetType() string {
	return d.typ
}

func (d *SettingsReporterMergeTask) ValuesMatch() bool {
	return d.GetSourceValue() == d.GetTargetValue()
}

func GetMRNRequiredSettings(clinic clinics.Clinic) (result string) {
	result = "Not Required"
	if clinic.MRNSettings != nil && clinic.MRNSettings.Required {
		result = "Required"
	}

	return
}

func GetMRNUniqueSettings(clinic clinics.Clinic) (result string) {
	result = "No"
	if clinic.MRNSettings != nil && clinic.MRNSettings.Unique {
		result = "Yes"
	}

	return
}

func GetGlucoseUnitsSettings(clinic clinics.Clinic) string {
	return clinic.PreferredBgUnits
}

func GetTimezoneSettings(clinic clinics.Clinic) (result string) {
	if clinic.Timezone != nil {
		result = *clinic.Timezone
	}
	return
}

type MembershipRestrictionsMergeTask struct {
	source clinics.Clinic
	target clinics.Clinic

	result TaskResult[SettingsReportDetails]
	err    error
}

func NewMembershipRestrictionsMergeTask(source, target clinics.Clinic) Task[SettingsReportDetails] {
	return &MembershipRestrictionsMergeTask{
		source: source,
		target: target,
	}
}

func (m *MembershipRestrictionsMergeTask) CanRun() bool {
	sourceMap := m.membershipRestrictionsToMap(m.source)
	targetMap := m.membershipRestrictionsToMap(m.target)

	// Check if the target clinic is a superset of the target map
	for domain, idp := range sourceMap {
		if targetIdp, ok := targetMap[domain]; ok && idp != targetIdp {
			return false
		}
	}

	return true
}

func (m *MembershipRestrictionsMergeTask) DryRun(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (m *MembershipRestrictionsMergeTask) Run(ctx context.Context) error {
	//TODO implement me
	panic("implement me")
}

func (m *MembershipRestrictionsMergeTask) GetResult() (TaskResult[SettingsReportDetails], error) {
	return m.result, m.err
}

func (m *MembershipRestrictionsMergeTask) GetSourceValue() string {
	return m.getSerializedValue(m.source)
}

func (m *MembershipRestrictionsMergeTask) GetTargetValue() string {
	return m.getSerializedValue(m.target)
}

func (m *MembershipRestrictionsMergeTask) GetType() string {
	return TaskTypeClinicSettingsHasPartialSSO
}

func (m *MembershipRestrictionsMergeTask) ValuesMatch() bool {
	return m.getSerializedValue(m.source) == m.getSerializedValue(m.target)
}

func (m *MembershipRestrictionsMergeTask) membershipRestrictionsToMap(clinic clinics.Clinic) map[string]string {
	result := map[string]string{}
	for _, r := range clinic.MembershipRestrictions {
		result[r.EmailDomain] = r.RequiredIdp
	}
	return result
}

func (m *MembershipRestrictionsMergeTask) getSerializedValue(clinic clinics.Clinic) string {
	result := "N/A"

	if count := len(clinic.MembershipRestrictions); count > 0 {
		list := make([]string, 0, count)
		for _, m := range clinic.MembershipRestrictions {
			list = append(list, m.String())
		}
		sort.Strings(list)
		result = strings.Join(list, ", ")
	}

	return result
}
