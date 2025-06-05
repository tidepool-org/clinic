package merge_test

import (
	"context"
	"errors"

	mapset "github.com/deckarep/golang-set/v2"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/onsi/gomega/gstruct"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx"
	"go.uber.org/fx/fxtest"
	"go.uber.org/mock/gomock"
	"go.uber.org/zap"

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
)

type ClinicMergeTest struct {
	cliniciansService clinicians.Service
	clinicManager     manager.Manager
	clinicsService    clinics.Service
	patientsService   patients.Service
	userService       *patientsTest.MockUserService
	ctrl              *gomock.Controller
	app               *fxtest.App

	plan     merge.ClinicMergePlan
	planId   primitive.ObjectID
	executor *merge.ClinicPlanExecutor
	planner  merge.Planner[merge.ClinicMergePlan]
	db       *mongo.Database

	source                       clinics.Clinic
	sourceAdmin                  clinicians.Clinician
	sourcePatients               []patients.Patient
	target                       clinics.Clinic
	targetAdmin                  clinicians.Clinician
	targetPatientsWithDuplicates map[string]patients.Patient
	targetPatients               []patients.Patient
}

func NewClinicMergeTest() *ClinicMergeTest {
	return &ClinicMergeTest{}
}

func (t *ClinicMergeTest) Init(params mergeTest.Params) {
	tb := GinkgoT()
	t.ctrl = gomock.NewController(tb)

	t.app = fxtest.New(tb,
		fx.NopLogger,
		fx.Provide(
			zap.NewNop,
			logger.Suggar,
			dbTest.GetTestDatabase,
			func(database *mongo.Database) *mongo.Client {
				t.db = database
				return database.Client()
			},
			func() patients.UserService {
				return patientsTest.NewMockUserService(t.ctrl)
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
			t.cliniciansService = cliniciansSvc
			t.clinicsService = clinicsSvc
			t.executor = &ex
			t.patientsService = patientsSvc
			t.userService = userSvc.(*patientsTest.MockUserService)
			t.clinicManager = cManager
		}),
	)
	t.app.RequireStart()

	data := mergeTest.RandomData(params)

	t.sourceAdmin = data.SourceAdmin
	t.sourcePatients = data.SourcePatients
	t.targetAdmin = data.TargetAdmin
	t.targetPatients = data.TargetPatients
	t.targetPatientsWithDuplicates = data.TargetPatientsWithDuplicates

	summaryPlaceholder := &patients.Summary{
		CGM: &patients.PatientCGMStats{
			Config:  patients.PatientSummaryConfig{},
			Dates:   patients.PatientSummaryDates{},
			Periods: patients.PatientCGMPeriods{},
		},
	}
	t.sourcePatients[0].Summary = summaryPlaceholder
	t.targetPatients[0].Summary = summaryPlaceholder

	t.source = createClinic(t.userService, t.clinicManager, data.Source, data.SourceAdmin)
	t.target = createClinic(t.userService, t.clinicManager, data.Target, data.TargetAdmin)

	if params.UniquePatientCount > clinics.PatientCountSettingsHardLimitPatientCountDefault {
		ctx := context.Background()
		pcs := &clinics.PatientCountSettings{
			HardLimit: &clinics.PatientCountLimit{PatientCount: params.UniquePatientCount * 2},
			SoftLimit: &clinics.PatientCountLimit{PatientCount: params.UniquePatientCount * 2},
		}
		Expect(t.clinicsService.UpdatePatientCountSettings(ctx, t.source.Id.Hex(), pcs)).To(Succeed())
		Expect(t.clinicsService.UpdatePatientCountSettings(ctx, t.target.Id.Hex(), pcs)).To(Succeed())
	}

	for _, p := range t.sourcePatients {
		p.ClinicId = t.source.Id
		_, err := t.patientsService.Create(context.Background(), p)
		Expect(err).ToNot(HaveOccurred())
	}
	for _, p := range t.targetPatients {
		p.ClinicId = t.target.Id
		_, err := t.patientsService.Create(context.Background(), p)
		Expect(err).ToNot(HaveOccurred())
	}

	t.planner = merge.NewClinicMergePlanner(t.clinicsService, t.patientsService, t.cliniciansService, data.Source.Id.Hex(), data.Target.Id.Hex())

}

var _ = Describe("New Clinic Merge Planner", Ordered, func() {
	var t *ClinicMergeTest
	var params = mergeTest.Params{
		UniquePatientCount:           patientCount,
		DuplicateAccountsCount:       duplicateAccountsCount,
		LikelyDuplicateAccountsCount: likelyDuplicateAccountsCount,
		NameOnlyMatchAccountsCount:   nameOnlyMatchAccountsCount,
		MrnOnlyMatchAccountsCount:    mrnOnlyMatchAccountsCount,
	}

	BeforeAll(func() {
		t = NewClinicMergeTest()
		t.Init(params)
	})

	AfterAll(func() {
		t.app.RequireStop()
	})

	It("successfully generates the plan", func() {
		var err error
		t.plan, err = t.planner.Plan(context.Background())
		Expect(err).ToNot(HaveOccurred())
	})

	It("doesn't remove patient summaries", func() {
		// Summaries should only be removed just before persistence. Removing them too early
		// might preclude information from them appearing in generated merge reports.
		sourceFound := false
		targetFound := false
		for _, p := range t.plan.PatientPlans {
			if p.SourcePatient != nil && p.SourcePatient.Summary != nil {
				sourceFound = true
				continue
			}
			if p.TargetPatient != nil && p.TargetPatient.Summary != nil {
				targetFound = true
				continue
			}
			if targetFound && sourceFound {
				break
			}
		}
		Expect(sourceFound).To(BeTrue())
		Expect(targetFound).To(BeTrue())
	})

	It("successfully executes the plan", func() {
		var err error
		t.planId, err = t.executor.Execute(context.Background(), t.plan)
		Expect(err).ToNot(HaveOccurred())
	})

	It("moves the source patients to the target clinic", func() {
		clinicId := t.target.Id.Hex()
		filter := patients.Filter{ClinicId: &clinicId}
		page := store.DefaultPagination().WithLimit(100000)
		result, err := t.patientsService.List(context.Background(), &filter, page, nil)
		Expect(err).ToNot(HaveOccurred())

		ids := make([]string, len(result.Patients))
		for i, p := range result.Patients {
			ids[i] = *p.UserId
		}

		expectedLen := len(t.sourcePatients) + len(t.targetPatients) - len(t.targetPatientsWithDuplicates)
		expected := make([]string, 0, expectedLen)
		for _, p := range t.sourcePatients {
			if _, ok := t.targetPatientsWithDuplicates[*p.UserId]; !ok {
				expected = append(expected, *p.UserId)
			}
		}
		for _, p := range t.targetPatients {
			expected = append(expected, *p.UserId)
		}

		Expect(ids).To(ConsistOf(expected))
	})

	It("moves the source clinician to the target clinic and retains the target clinic admin", func() {
		clinicId := t.target.Id.Hex()
		filter := clinicians.Filter{ClinicId: &clinicId}
		page := store.DefaultPagination().WithLimit(100000)
		result, err := t.cliniciansService.List(context.Background(), &filter, page)
		Expect(err).ToNot(HaveOccurred())

		ids := make([]string, len(result))
		for i, p := range result {
			ids[i] = *p.UserId
		}

		expected := []string{
			*t.sourceAdmin.UserId,
			*t.targetAdmin.UserId,
		}
		Expect(ids).To(ConsistOf(expected))
	})

	It("removes the source clinic", func() {
		_, err := t.clinicsService.Get(context.Background(), t.source.Id.Hex())
		Expect(errors.Is(err, errs.NotFound)).To(BeTrue())
	})

	It("merges share codes correctly", func() {
		result, err := t.clinicsService.Get(context.Background(), t.target.Id.Hex())
		Expect(err).ToNot(HaveOccurred())
		Expect(result.ShareCodes).To(gstruct.PointTo(ConsistOf([]string{*t.source.CanonicalShareCode, *t.target.CanonicalShareCode})))
	})

	It("add clinician user ids to the admins array", func() {
		expectedAdmins := []string{
			*t.sourceAdmin.UserId,
			*t.targetAdmin.UserId,
		}
		result, err := t.clinicsService.Get(context.Background(), t.target.Id.Hex())
		Expect(err).ToNot(HaveOccurred())
		Expect(result.Admins).To(gstruct.PointTo(ConsistOf(expectedAdmins)))
	})

	It("merge tags correctly", func() {
		uniqueTags := mapset.NewSet[string]()
		for _, t := range t.source.PatientTags {
			uniqueTags.Append(t.Name)
		}
		for _, t := range t.target.PatientTags {
			uniqueTags.Append(t.Name)
		}
		expectedTagNames := uniqueTags.ToSlice()

		result, err := t.clinicsService.Get(context.Background(), t.target.Id.Hex())
		Expect(err).ToNot(HaveOccurred())

		resultingTagNames := make([]string, 0, len(result.PatientTags))
		for _, tag := range result.PatientTags {
			resultingTagNames = append(resultingTagNames, tag.Name)
		}

		Expect(resultingTagNames).To(ConsistOf(expectedTagNames))
	})

	It("contains plan for each source tag", func() {
		for _, tag := range t.source.PatientTags {
			expectedCount := 1
			if clinicHasTagWithName(t.target, tag.Name) {
				expectedCount = 2
			}
			hasMergePlan(t.db, bson.M{
				"planId":    t.planId,
				"type":      "tag",
				"plan.name": tag.Name,
			}, expectedCount)
		}
	})

	It("contains plan for each target tag", func() {
		for _, tag := range t.target.PatientTags {
			expectedCount := 1
			if clinicHasTagWithName(t.source, tag.Name) {
				expectedCount = 2
			}
			hasMergePlan(t.db, bson.M{
				"planId":    t.planId,
				"type":      "tag",
				"plan.name": tag.Name,
			}, expectedCount)
		}
	})

	It("contains plan for each source patient", func() {
		for _, patient := range t.sourcePatients {
			hasMergePlan(t.db, bson.M{
				"planId":                    t.planId,
				"type":                      "patient",
				"plan.sourcePatient.userId": *patient.UserId,
			}, 1)
		}
	})

	It("contains plan for source admin", func() {
		hasMergePlan(t.db, bson.M{
			"planId":                t.planId,
			"type":                  "clinician",
			"plan.clinician.userId": *t.sourceAdmin.UserId,
		}, 1)
	})

	It("contains plan for target admin", func() {
		hasMergePlan(t.db, bson.M{
			"planId":                t.planId,
			"type":                  "clinician",
			"plan.clinician.userId": *t.targetAdmin.UserId,
		}, 1)
	})

	It("contains plan for clinics", func() {
		hasMergePlan(t.db, bson.M{
			"planId":          t.planId,
			"type":            "clinic",
			"plan.source._id": t.source.Id,
			"plan.target._id": t.target.Id,
		}, 1)
	})
})

var _ = Describe("New Clinic Merge Planner (w/ Large patient populations)", Ordered, func() {
	var t *ClinicMergeTest
	var params = mergeTest.Params{UniquePatientCount: 1025}

	BeforeAll(func() {
		t = NewClinicMergeTest()
		t.Init(params)
	})

	AfterAll(func() {
		t.app.RequireStop()
	})

	It("successfully handles multiple passes", func() {
		var err error

		t.plan, err = t.planner.Plan(context.Background())
		Expect(err).ToNot(HaveOccurred())
		Expect(t.plan.PatientPlans.GetResultingPatientsCount()).To(Equal(2050))
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
