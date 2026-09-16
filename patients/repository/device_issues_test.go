package repository_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.uber.org/fx/fxtest"
	"go.uber.org/zap"

	"github.com/tidepool-org/clinic/config"
	"github.com/tidepool-org/clinic/patients"
	patientsRepository "github.com/tidepool-org/clinic/patients/repository"
	patientsTest "github.com/tidepool-org/clinic/patients/test"
	dbTest "github.com/tidepool-org/clinic/store/test"
)

var _ = Describe("Patients Repository Device Issues", func() {
	var repo patients.Repository
	var collection *mongo.Collection
	var ctx context.Context
	var now time.Time
	var created []primitive.ObjectID

	dexcom := patients.DexcomDataSourceProviderName
	twiist := patients.TwiistDataSourceProviderName
	invite := patients.PrimaryIssueSourceDeviceNonSpecificInvite
	expired := patients.PrimaryIssueKindInvitationExpired

	type subject struct{ clinicId, userId string }

	BeforeEach(func() {
		ctx = context.Background()
		// Mongo stores dates at millisecond precision, so times that must compare equal
		// after a round trip are truncated.
		now = time.Now().UTC().Truncate(time.Millisecond)
		created = nil

		cfg := &config.Config{ClinicDemoPatientUserId: DemoPatientId}
		database := dbTest.GetTestDatabase()
		collection = database.Collection("patients")
		lifecycle := fxtest.NewLifecycle(GinkgoT())
		var err error
		repo, err = patientsRepository.NewRepository(cfg, database, zap.NewNop().Sugar(),
			lifecycle)
		Expect(err).ToNot(HaveOccurred())
		lifecycle.RequireStart()
	})

	AfterEach(func() {
		// Other specs in this package count documents, so leave none behind.
		_, err := collection.DeleteMany(ctx, bson.M{"_id": bson.M{"$in": created}})
		Expect(err).ToNot(HaveOccurred())
	})

	// request builds a connection request created at the given time that expires after
	// PendingDataSourceExpirationDuration, as the service creates them.
	request := func(provider string, createdTime time.Time) patients.ConnectionRequest {
		return patients.ConnectionRequest{
			ProviderName:   provider,
			CreatedTime:    createdTime,
			ExpirationTime: createdTime.Add(patients.PendingDataSourceExpirationDuration),
		}
	}

	dataSource := func(provider string, createdTime time.Time) patients.DataSource {
		return patients.DataSource{
			ProviderName: provider,
			State:        patients.DataSourceStateConnected,
			CreatedTime:  &createdTime,
		}
	}

	issue := func(source string) *patients.PrimaryIssue {
		return &patients.PrimaryIssue{
			Source:        source,
			EffectiveTime: now.Add(-24 * time.Hour),
		}
	}

	// seed stores a patient with the given primary issue, connection requests (newest
	// first, as the service stores them) and data sources, bypassing the derivations that
	// normally maintain them.
	seed := func(
		primary *patients.PrimaryIssue, requests patients.ProviderConnectionRequests,
		sources ...patients.DataSource,
	) subject {
		GinkgoHelper()
		patient := patientsTest.RandomPatient()
		patient.PrimaryIssue = primary
		patient.ProviderConnectionRequests = requests
		patient.DataSources = &sources
		stored, err := repo.Create(ctx, patient)
		Expect(err).ToNot(HaveOccurred())
		created = append(created, *stored.Id)
		return subject{clinicId: stored.ClinicId.Hex(), userId: *stored.UserId}
	}

	get := func(s subject) *patients.Patient {
		GinkgoHelper()
		got, err := repo.Get(ctx, s.clinicId, s.userId)
		Expect(err).ToNot(HaveOccurred())
		return got
	}

	update := func() {
		GinkgoHelper()
		Expect(repo.UpdateDeviceIssues(ctx)).To(Succeed())
	}

	Describe("expired provider-specific invitations", func() {
		// Creation times for requests, relative to when a request created now would
		// expire: lapsed and longLapsed have expired, an hour and two hours ago; current
		// expires in half an hour.
		var lapsed, longLapsed, current time.Time

		BeforeEach(func() {
			expiresNow := now.Add(-patients.PendingDataSourceExpirationDuration)
			lapsed = expiresNow.Add(-time.Hour)
			longLapsed = expiresNow.Add(-2 * time.Hour)
			current = expiresNow.Add(30 * time.Minute)
		})

		It("classifies the issue when the invitation expired with no data source", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, lapsed)},
			})
			update()
			got := get(s)
			Expect(got.PrimaryIssue).To(PointTo(Equal(patients.PrimaryIssue{
				Source:        dexcom,
				Kind:          expired,
				EffectiveTime: lapsed.Add(patients.PendingDataSourceExpirationDuration),
			})))
			Expect(got.UpdatedTime).To(BeTemporally("~", time.Now(), time.Second))
		})

		It("classifies the issue when the data source predates the invitation", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, lapsed)},
			}, dataSource(dexcom, longLapsed))
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", expired)))
		})

		It("ignores an invitation that was followed by a data source", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, lapsed)},
			}, dataSource(dexcom, now.Add(-time.Hour)))
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("ignores a current invitation", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, current)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("considers only the newest invitation", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, current), request(dexcom, lapsed)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("uses the newest invitation's expiration as the effective time", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, lapsed), request(dexcom, longLapsed)},
			})
			update()
			expiration := lapsed.Add(patients.PendingDataSourceExpirationDuration)
			Expect(get(s).PrimaryIssue).To(PointTo(And(
				HaveField("Kind", expired),
				HaveField("EffectiveTime", BeTemporally("==", expiration)),
			)))
		})

		It("ignores invitations for other providers", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				twiist: {request(twiist, lapsed)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("ignores an invitation without an expiration time", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {{ProviderName: dexcom, CreatedTime: lapsed}},
			})
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("ignores the device-non-specific invitation", func() {
			s := seed(issue(invite), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, lapsed)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(invite)))
		})

		It("ignores patients without a primary issue", func() {
			s := seed(nil, patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, lapsed)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(BeNil())
		})

		It("replaces an existing kind", func() {
			classified := issue(dexcom)
			classified.Kind = patients.PrimaryIssueKindStaleData
			s := seed(classified, patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, lapsed)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", expired)))
		})

		It("leaves an already classified patient alone", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, lapsed)},
			})
			update()
			first := get(s)
			Expect(first.PrimaryIssue).To(PointTo(HaveField("Kind", expired)))

			update()
			second := get(s)
			Expect(second.PrimaryIssue).To(Equal(first.PrimaryIssue))
			Expect(second.UpdatedTime).To(BeTemporally("==", first.UpdatedTime))
		})

		It("classifies only the qualifying patients in one run", func() {
			expiredSubject := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, lapsed)},
			})
			pending := seed(issue(twiist), patients.ProviderConnectionRequests{
				twiist: {request(twiist, current)},
			})
			update()
			Expect(get(expiredSubject).PrimaryIssue).To(PointTo(HaveField("Kind", expired)))
			Expect(get(pending).PrimaryIssue).To(Equal(issue(twiist)))
		})
	})
})
