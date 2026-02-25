package test

import (
	"fmt"
	"math/rand/v2"
	"strings"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/onsi/gomega/types"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"github.com/tidepool-org/go-common/clients/shoreline"

	"github.com/tidepool-org/clinic/api"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/test"
)

var permissions = []string{"view", "upload", "note", "custodian"}
var devices = []string{"dexcom_g6", "dexcom_g7", "t:slim_X2", "medtronic_630G"}

func strp(s string) *string {
	return &s
}

func RandomPatient() patients.Patient {
	clinicId := primitive.NewObjectID()
	devices := []string{test.Faker.Company().Name(), test.Faker.Company().Name(), test.Faker.Company().Name()}
	tags := []primitive.ObjectID{primitive.NewObjectID()}
	permissions := RandomPermissions()
	dataSources := RandomDataSources()
	mrn := test.Faker.UUID().V4()
	return patients.Patient{
		ClinicId:         &clinicId,
		UserId:           strp(test.Faker.UUID().V4()),
		BirthDate:        strp(test.Faker.Time().ISO8601(time.Now())[:10]),
		Email:            strp(test.Faker.Internet().Email()),
		FullName:         strp(test.Faker.Person().Name()),
		Mrn:              strp(test.Faker.RandomStringElement([]string{mrn, strings.ToUpper(mrn)})),
		Tags:             &tags,
		TargetDevices:    &devices,
		Permissions:      &permissions,
		IsMigrated:       test.Faker.Bool(),
		DataSources:      (*[]patients.DataSource)(&dataSources),
		EHRSubscriptions: RandomSubscriptions(),
		Sites:            &[]sites.Site{},
		GlycemicRanges:   RandomGlycemicRanges(),
		DiagnosisType:    RandomDiagnosisType(),
		UpdatedTime:      test.Faker.Time().TimeBetween(time.Now().Add(-time.Hour), time.Now().Add(-time.Minute)),
	}
}

func RandomGlycemicRanges() patients.GlycemicRanges {
	if rand.IntN(2)%2 == 0 {
		return RandomGlycemicRangesCustom()
	}
	return RandomGlycemicRangesPreset()
}

func RandomGlycemicRangesCustom() patients.GlycemicRanges {
	return patients.GlycemicRanges{
		Type: "custom",
		Custom: patients.GlycemicRangesCustom{
			Name: test.Faker.Lorem().Sentence(1 + rand.IntN(5)),
			Thresholds: []patients.GlycemicRangeThreshold{
				{
					Name: test.Faker.Lorem().Sentence(1 + rand.IntN(5)),
					UpperBound: patients.ValueWithUnits{
						Units: test.Faker.RandomStringElement([]string{
							"mg/dL",
							"mmol/L",
						}),
						Value: test.Faker.Float32(1, 0, 100),
					},
					Inclusive: test.Faker.Bool(),
				},
			},
		},
	}
}

var glycemicRangesPresets = []api.GlycemicRangesPresetV1{
	api.ADAStandard,
	api.ADAPregnancyType1,
	api.ADAPregnancyType2,
	api.ADAHighRisk,
}

func RandomGlycemicRangesPreset() patients.GlycemicRanges {
	choice := glycemicRangesPresets[rand.IntN(len(glycemicRangesPresets))]
	return patients.GlycemicRanges{
		Type:   "preset",
		Preset: patients.GlycemicRangesPreset(string(choice)),
	}
}

func RandomDiagnosisType() *patients.DiagnosisType {
	all := []api.DiagnosisTypeV1{
		api.DiagnosisTypeV1Empty,
		api.DiagnosisTypeV1Gestational,
		api.DiagnosisTypeV1Lada,
		api.DiagnosisTypeV1Mody,
		api.DiagnosisTypeV1NotApplicable,
		api.DiagnosisTypeV1Other,
		api.DiagnosisTypeV1Prediabetes,
		api.DiagnosisTypeV1Type1,
		api.DiagnosisTypeV1Type2,
	}
	choice := patients.DiagnosisType(all[rand.IntN(len(all))])
	return &choice
}

func RandomSubscriptions() patients.EHRSubscriptions {
	subs := make(patients.EHRSubscriptions)
	subs[patients.SubscriptionRedoxSummaryAndReports] = patients.EHRSubscription{
		Active: true,
		MatchedMessages: []patients.MatchedMessage{{
			DocumentId: primitive.NewObjectID(),
			DataModel:  "Order",
			EventType:  "New",
		}},
	}
	subs[patients.SubscriptionXealthReports] = patients.EHRSubscription{
		Active: true,
		MatchedMessages: []patients.MatchedMessage{{
			DocumentId: primitive.NewObjectID(),
			DataModel:  "Order",
			EventType:  "New",
		}},
	}
	return subs
}

func RandomPatientUpdate() patients.PatientUpdate {
	patient := RandomPatient()
	return patients.PatientUpdate{
		Patient: patients.Patient{
			BirthDate:        patient.BirthDate,
			Email:            patient.Email,
			FullName:         patient.FullName,
			Mrn:              patient.Mrn,
			Tags:             patient.Tags,
			TargetDevices:    patient.TargetDevices,
			Permissions:      patient.Permissions,
			DataSources:      patient.DataSources,
			EHRSubscriptions: RandomSubscriptions(),
			Sites:            patient.Sites,
			GlycemicRanges:   patient.GlycemicRanges,
			DiagnosisType:    patient.DiagnosisType,
		},
	}
}

func RandomPermission() string {
	return test.Faker.RandomStringElement(permissions)
}

func RandomPermissions() patients.Permissions {
	a := append([]string{}, permissions...)
	test.Rand.Shuffle(len(a), func(i, j int) { a[i], a[j] = a[j], a[i] })
	count := test.Faker.IntBetween(1, len(a))
	a = a[:count]
	permissions := patients.Permissions{}
	for _, p := range a {
		setPermission(&permissions, p)
	}
	return permissions
}

func RandomDataSources() patients.DataSources {
	return []patients.DataSource{
		{State: test.Faker.RandomStringElement([]string{"pending", "connected"}), ProviderName: test.Faker.Company().Name()},
	}
}

func RandomProfile() patients.Profile {
	return patients.Profile{
		FullName: strp(test.Faker.Person().Name()),
		Patient: patients.PatientProfile{
			Mrn:           strp(test.Faker.UUID().V4()),
			Birthday:      strp(test.Faker.Time().ISO8601(time.Now())[:10]),
			TargetDevices: &[]string{test.Faker.RandomStringElement(devices)},
			Email:         strp(test.Faker.Internet().Email()),
			FullName:      strp(test.Faker.Person().Name()),
		},
	}
}

func RandomUser() shoreline.UserData {
	email := test.Faker.Internet().Email()
	return shoreline.UserData{
		UserID:         test.Faker.UUID().V4(),
		Username:       email,
		Emails:         []string{email},
		PasswordExists: true,
		Roles:          []string{"patient"},
		EmailVerified:  true,
		TermsAccepted:  fmt.Sprintf("%v", test.Faker.Time().Unix(time.Now())),
	}
}

func setPermission(permissions *patients.Permissions, p string) {
	switch p {
	case "view":
		permissions.View = &patients.Permission{}
	case "note":
		permissions.View = &patients.Permission{}
	case "upload":
		permissions.Upload = &patients.Permission{}
	case "custodian":
		permissions.Custodian = &patients.Permission{}
	}
}

func PatientFieldsMatcher(patient patients.Patient) types.GomegaMatcher {
	GinkgoHelper()
	return MatchAllFields(Fields{
		"Id":                             PointTo(Not(BeEmpty())),
		"UserId":                         PointTo(Equal(*patient.UserId)),
		"ClinicId":                       PointTo(Equal(*patient.ClinicId)),
		"BirthDate":                      PointTo(Equal(*patient.BirthDate)),
		"Email":                          PointTo(Equal(*patient.Email)),
		"FullName":                       PointTo(Equal(*patient.FullName)),
		"Mrn":                            PointTo(Equal(*patient.Mrn)),
		"Tags":                           PointTo(Equal(*patient.Tags)),
		"TargetDevices":                  PointTo(Equal(*patient.TargetDevices)),
		"Permissions":                    PointTo(Equal(*patient.Permissions)),
		"IsMigrated":                     Equal(patient.IsMigrated),
		"LegacyClinicianIds":             ConsistOf(patient.LegacyClinicianIds),
		"UpdatedTime":                    Ignore(),
		"CreatedTime":                    Ignore(),
		"InvitedBy":                      Ignore(),
		"Summary":                        Ignore(),
		"Reviews":                        Ignore(),
		"ProviderConnectionRequests":     Equal(patient.ProviderConnectionRequests),
		"LastUploadReminderTime":         Equal(patient.LastUploadReminderTime),
		"LastRequestedDexcomConnectTime": Equal(patient.LastRequestedDexcomConnectTime),
		"DataSources":                    PointTo(Equal(*patient.DataSources)),
		"RequireUniqueMrn":               Equal(patient.RequireUniqueMrn),
		"EHRSubscriptions":               Equal(patient.EHRSubscriptions),
		"Sites":                          Equal(patient.Sites),
		"GlycemicRanges":                 Equal(patient.GlycemicRanges),
		"DiagnosisType":                  Equal(patient.DiagnosisType),
	})
}
