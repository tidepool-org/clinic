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

type SettingsPlan struct {
	Name        string `bson:"name"`
	SourceValue string `bson:"sourceValue"`
	TargetValue string `bson:"targetValue"`
}

func (s SettingsPlan) ValuesMatch() bool {
	return s.SourceValue == s.TargetValue
}

func (s SettingsPlan) PreventsMerge() bool {
	return !s.ValuesMatch()
}

type SettingsPlans []SettingsPlan

func (s SettingsPlans) PreventsMerge() bool {
	return false
}

type SettingsReporterPlanner struct {
	getter ClinicPropertyGetter
	source clinics.Clinic
	target clinics.Clinic
	typ    string
}

func NewSettingsReporterPlanner(source, target clinics.Clinic, getter ClinicPropertyGetter, typ string) Planner[SettingsPlan] {
	return &SettingsReporterPlanner{
		getter: getter,
		source: source,
		target: target,
		typ:    typ,
	}
}

func (d *SettingsReporterPlanner) Plan(ctx context.Context) (SettingsPlan, error) {
	return SettingsPlan{
		Name:        d.GetType(),
		SourceValue: d.GetSourceValue(),
		TargetValue: d.GetTargetValue(),
	}, nil
}

func (d *SettingsReporterPlanner) getClinicName(clinic clinics.Clinic) (val string) {
	if clinic.Name != nil {
		val = *clinic.Name
	}
	return
}

func (d *SettingsReporterPlanner) GetSourceValue() string {
	return d.getter(d.source)
}

func (d *SettingsReporterPlanner) GetTargetValue() string {
	return d.getter(d.target)
}

func (d *SettingsReporterPlanner) GetType() string {
	return d.typ
}

func (d *SettingsReporterPlanner) ValuesMatch() bool {
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

type MembershipRestrictionsMergePlan struct {
	SourceValue []clinics.MembershipRestrictions `bson:"sourceValue"`
	TargetValue []clinics.MembershipRestrictions `bson:"targetValue"`
}

func (m MembershipRestrictionsMergePlan) ValuesMatch() bool {
	return m.GetSourceValue() == m.GetTargetValue()
}

func (m MembershipRestrictionsMergePlan) GetSourceValue() string {
	return m.getSerializedValue(m.SourceValue)
}

func (m MembershipRestrictionsMergePlan) GetTargetValue() string {
	return m.getSerializedValue(m.TargetValue)
}

func (m MembershipRestrictionsMergePlan) getSerializedValue(restrictions []clinics.MembershipRestrictions) string {
	result := "N/A"

	if count := len(restrictions); count > 0 {
		list := make([]string, 0, count)
		for _, m := range restrictions {
			list = append(list, m.String())
		}
		sort.Strings(list)
		result = strings.Join(list, ", ")
	}

	return result
}

func (m MembershipRestrictionsMergePlan) PreventsMerge() bool {
	sourceMap := m.membershipRestrictionsToMap(m.SourceValue)
	targetMap := m.membershipRestrictionsToMap(m.TargetValue)

	// Check if the source map is a subset of the target map
	for sourceDomain, sourceIDP := range sourceMap {
		targetIDP, ok := targetMap[sourceDomain]
		return !ok || sourceIDP != targetIDP
	}

	return false
}

func (m MembershipRestrictionsMergePlan) membershipRestrictionsToMap(restrictions []clinics.MembershipRestrictions) map[string]string {
	result := map[string]string{}
	for _, r := range restrictions {
		result[r.EmailDomain] = r.RequiredIdp
	}
	return result
}

type MembershipRestrictionsMergePlanner struct {
	source clinics.Clinic
	target clinics.Clinic
}

func NewMembershipRestrictionsMergePlanner(source, target clinics.Clinic) Planner[MembershipRestrictionsMergePlan] {
	return &MembershipRestrictionsMergePlanner{
		source: source,
		target: target,
	}
}

func (m *MembershipRestrictionsMergePlanner) Plan(ctx context.Context) (MembershipRestrictionsMergePlan, error) {
	return MembershipRestrictionsMergePlan{
		SourceValue: m.source.MembershipRestrictions,
		TargetValue: m.target.MembershipRestrictions,
	}, nil
}
