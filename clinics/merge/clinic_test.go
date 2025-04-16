package merge_test

import (
	"context"
	"errors"
	mapset "github.com/deckarep/golang-set/v2"
	"go.uber.org/mock/gomock"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/clinicians"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/clinics/manager"
	"github.com/tidepool-org/clinic/clinics/merge"
	mergeTest "github.com/tidepool-org/clinic/clinics/merge/test"
	"github.com/tidepool-org/clinic/config"
	errs "github.com/tidepool-org/clinic/errors"
	"github.com/tidepool-org/clinic/logger"
	"github.com/tidepool-org/clinic/patients"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	"github.com/tidepool-org/clinic/store"
	dbTest "github.com/tidepool-org/clinic/store/test"
	"github.com/tidepool-org/go-common/clients/shoreline"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"
)

var _ = Describe("New Clinic Merge Planner", Ordered, func() {
	var cliniciansService clinicians.Service
	var clinicManager manager.Manager
	var clinicsService clinics.Service
	var patientsService patients.Service
	var userService *patientsTest.MockUserService
	var ctrl *gomock.Controller
	var app *fxtest.App

	var plan merge.ClinicMergePlan
	var planId primitive.ObjectID
	var executor *merge.ClinicPlanExecutor
	var planner merge.Planner[merge.ClinicMergePlan]
	var db *mongo.Database

	var source clinics.Clinic
	var sourceAdmin clinicians.Clinician
	var sourcePatients []patients.Patient
	var target clinics.Clinic
	var targetAdmin clinicians.Clinician
	var targetPatientsWithDuplicates map[string]patients.Patient
	var targetPatients []patients.Patient

	BeforeAll(func() {
		tb := GinkgoT()
		ctrl = gomock.NewController(tb)
		app = fxtest.New(tb,
			fx.NopLogger,
			fx.Provide(
				zap.NewNop,
				logger.Suggar,
				dbTest.GetTestDatabase,
				func(database *mongo.Database) *mongo.Client {
					db = database
					return database.Client()
				},
				func() patients.UserService {
					return patientsTest.NewMockUserService(ctrl)
				},
				patientsTest.NewMockUserService,
				config.NewConfig,
				clinics.NewRepository,
				clinicians.NewRepository,
				clinicians.NewService,
				patients.NewDeletionsRepository,
				patients.NewRepository,
				patients.NewService,
				patients.NewCustodialService,
				clinics.NewShareCodeGenerator,
				manager.NewManager,
			),
			fx.Invoke(func(ex merge.ClinicPlanExecutor, cliniciansSvc clinicians.Service, clinicsSvc clinics.Service, patientsSvc patients.Service, cManager manager.Manager, userSvc patients.UserService) {
				cliniciansService = cliniciansSvc
				clinicsService = clinicsSvc
				executor = &ex
				patientsService = patientsSvc
				userService = userSvc.(*patientsTest.MockUserService)
				clinicManager = cManager
			}),
		)
		app.RequireStart()

		data := mergeTest.RandomData(mergeTest.Params{
			PatientCount:                 patientCount,
			DuplicateAccountsCount:       duplicateAccountsCount,
			LikelyDuplicateAccountsCount: likelyDuplicateAccountsCount,
			NameOnlyMatchAccountsCount:   nameOnlyMatchAccountsCount,
			MrnOnlyMatchAccountsCount:    mrnOnlyMatchAccountsCount,
		})

		sourceAdmin = data.SourceAdmin
		sourcePatients = data.SourcePatients
		targetAdmin = data.TargetAdmin
		targetPatients = data.TargetPatients
		targetPatientsWithDuplicates = data.TargetPatientsWithDuplicates

		source = createClinic(userService, clinicManager, data.Source, data.SourceAdmin)
		target = createClinic(userService, clinicManager, data.Target, data.TargetAdmin)

		for _, p := range sourcePatients {
			p.ClinicId = source.Id
			_, err := patientsService.Create(context.Background(), p)
			Expect(err).ToNot(HaveOccurred())
		}
		for _, p := range targetPatients {
			p.ClinicId = target.Id
			_, err := patientsService.Create(context.Background(), p)
			Expect(err).ToNot(HaveOccurred())
		}

		planner = merge.NewClinicMergePlanner(clinicsService, patientsService, cliniciansService, data.Source.Id.Hex(), data.Target.Id.Hex())

	})

	AfterAll(func() {
		app.RequireStop()
	})

	It("successfully generates the plan", func() {
		var err error
		plan, err = planner.Plan(context.Background())
		Expect(err).ToNot(HaveOccurred())
	})

	It("successfully executes the plan", func() {
		var err error
		planId, err = executor.Execute(context.Background(), plan)
		Expect(err).ToNot(HaveOccurred())
	})

	It("moves the source patients to the target clinic", func() {
		clinicId := target.Id.Hex()
		filter := patients.Filter{ClinicId: &clinicId}
		page := store.DefaultPagination().WithLimit(100000)
		result, err := patientsService.List(context.Background(), &filter, page, nil)
		Expect(err).ToNot(HaveOccurred())

		ids := make([]string, len(result.Patients))
		for i, p := range result.Patients {
			ids[i] = *p.UserId
		}

		expectedLen := len(sourcePatients) + len(targetPatients) - len(targetPatientsWithDuplicates)
		expected := make([]string, 0, expectedLen)
		for _, p := range sourcePatients {
			if _, ok := targetPatientsWithDuplicates[*p.UserId]; !ok {
				expected = append(expected, *p.UserId)
			}
		}
		for _, p := range targetPatients {
			expected = append(expected, *p.UserId)
		}

		Expect(ids).To(ConsistOf(expected))
	})

	It("moves the source clinician to the target clinic and retains the target clinic admin", func() {
		clinicId := target.Id.Hex()
		filter := clinicians.Filter{ClinicId: &clinicId}
		page := store.DefaultPagination().WithLimit(100000)
		result, err := cliniciansService.List(context.Background(), &filter, page)
		Expect(err).ToNot(HaveOccurred())

		ids := make([]string, len(result))
		for i, p := range result {
			ids[i] = *p.UserId
		}

		expected := []string{
			*sourceAdmin.UserId,
			*targetAdmin.UserId,
		}
		Expect(ids).To(ConsistOf(expected))
	})

	It("removes the source clinic", func() {
		_, err := clinicsService.Get(context.Background(), source.Id.Hex())
		Expect(errors.Is(err, errs.NotFound)).To(BeTrue())
	})

	It("merges share codes correctly", func() {
		result, err := clinicsService.Get(context.Background(), target.Id.Hex())
		Expect(err).ToNot(HaveOccurred())
		Expect(result.ShareCodes).To(gstruct.PointTo(ConsistOf([]string{*source.CanonicalShareCode, *target.CanonicalShareCode})))
	})

	It("add clinician user ids to the admins array", func() {
		expectedAdmins := []string{
			*sourceAdmin.UserId,
			*targetAdmin.UserId,
		}
		result, err := clinicsService.Get(context.Background(), target.Id.Hex())
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Admins).To(gstruct.PointTo(ConsistOf(expectedAdmins)))
	})

	It("merge tags correctly", func() {
		uniqueTags := mapset.NewSet[string]()
		for _, t := range source.PatientTags {
			uniqueTags.Append(t.Name)
		}
		for _, t := range target.PatientTags {
			uniqueTags.Append(t.Name)
		}
		expectedTagNames := uniqueTags.ToSlice()

		result, err := clinicsService.Get(context.Background(), target.Id.Hex())
		Expect(err).ToNot(HaveOccurred())

		resultingTagNames := make([]string, 0, len(result.PatientTags))
		for _, tag := range result.PatientTags {
			resultingTagNames = append(resultingTagNames, tag.Name)
		}

		Expect(resultingTagNames).To(ConsistOf(expectedTagNames))
	})

	It("contains plan for each source tag", func() {
		for _, tag := range source.PatientTags {
			expectedCount := 1
			if clinicHasTagWithName(target, tag.Name) {
				expectedCount = 2
			}
			hasMergePlan(db, bson.M{
				"planId":              planId,
				"type":                "tag",
				"plan.name":           tag.Name,
			}, expectedCount)
		}
	})

	It("contains plan for each target tag", func() {
		for _, tag := range target.PatientTags {
			expectedCount := 1
			if clinicHasTagWithName(source, tag.Name) {
				expectedCount = 2
			}
			hasMergePlan(db, bson.M{
				"planId":              planId,
				"type":                "tag",
				"plan.name":           tag.Name,
			}, expectedCount)
		}
	})

	It("contains plan for each source patient", func() {
		for _, patient := range sourcePatients {
			hasMergePlan(db, bson.M{
				"planId":                    planId,
				"type":                      "patient",
				"plan.sourcePatient.userId": *patient.UserId,
			}, 1)
		}
	})

	It("contains plan for source admin", func() {
		hasMergePlan(db, bson.M{
			"planId":                planId,
			"type":                  "clinician",
			"plan.clinician.userId": *sourceAdmin.UserId,
		}, 1)
	})

	It("contains plan for target admin", func() {
		hasMergePlan(db, bson.M{
			"planId":                planId,
			"type":                  "clinician",
			"plan.clinician.userId": *targetAdmin.UserId,
		}, 1)
	})

	It("contains plan for clinics", func() {
		hasMergePlan(db, bson.M{
			"planId":          planId,
			"type":            "clinic",
			"plan.source._id": source.Id,
			"plan.target._id": target.Id,
		}, 1)
	})
})

func hasMergePlan(db *mongo.Database, filter bson.M, expectedCount int) {
	res, err := db.Collection("merge_plans").CountDocuments(context.Background(), filter)
	Expect(err).ToNot(HaveOccurred())
	Expect(res).To(BeNumerically("==", expectedCount))
}

func createClinic(userService *patientsTest.MockUserService, clinicManager manager.Manager, clinic clinics.Clinic, admin clinicians.Clinician) clinics.Clinic {
	userService.EXPECT().GetUser(*admin.UserId).Return(&shoreline.UserData{
		UserID:         *admin.UserId,
		Username:       *admin.Email,
		Emails:         []string{*admin.Email},
		PasswordExists: true,
		EmailVerified:  true,
	}, nil)
	userService.EXPECT().GetUserProfile(gomock.Any(), *admin.UserId).Return(&patients.Profile{
		FullName: admin.Name,
	}, nil)

	clinic.Admins = nil
	result, err := clinicManager.CreateClinic(context.Background(), &manager.CreateClinic{
		Clinic:        clinic,
		CreatorUserId: *admin.UserId,
	})
	Expect(err).ToNot(HaveOccurred())
	return *result
}

func clinicHasTagWithName(clinic clinics.Clinic, tagName string) bool {
	tagNames := mapset.NewSet[string]()
	for _, tag := range clinic.PatientTags {
		tagNames.Append(tag.Name)
	}
	return tagNames.Contains(tagName)
}
