package repository_test

import (
	"context"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
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

type updateDeviceIssuesTestHelper struct {
	ctx        context.Context
	repo       patients.Repository
	collection *mongo.Collection
	inserted   []primitive.ObjectID
}

func newUpdateDeviceIssuesTestHelper() *updateDeviceIssuesTestHelper {
	GinkgoHelper()
	cfg := &config.Config{ClinicDemoPatientUserId: DemoPatientId}
	database := dbTest.GetTestDatabase()
	collection := database.Collection("patients")
	lifecycle := fxtest.NewLifecycle(GinkgoT())
	repo, err := patientsRepository.NewRepository(cfg, database, zap.NewNop().Sugar(), lifecycle)
	Expect(err).ToNot(HaveOccurred())
	lifecycle.RequireStart()
	return &updateDeviceIssuesTestHelper{
		ctx:        context.Background(),
		repo:       repo,
		collection: collection,
	}
}

func (f *updateDeviceIssuesTestHelper) insertPatient(patient patients.Patient) primitive.ObjectID {
	GinkgoHelper()
	result, err := f.collection.InsertOne(f.ctx, patient)
	Expect(err).ToNot(HaveOccurred())
	id := result.InsertedID.(primitive.ObjectID)
	f.inserted = append(f.inserted, id)
	return id
}

func (f *updateDeviceIssuesTestHelper) fetchPatient(id primitive.ObjectID) patients.Patient {
	GinkgoHelper()
	var p patients.Patient
	err := f.collection.FindOne(f.ctx, bson.M{"_id": id}).Decode(&p)
	Expect(err).ToNot(HaveOccurred())
	return p
}

func (f *updateDeviceIssuesTestHelper) cleanup() {
	GinkgoHelper()
	if len(f.inserted) == 0 {
		return
	}
	_, err := f.collection.DeleteMany(f.ctx, bson.M{"_id": bson.M{"$in": f.inserted}})
	Expect(err).ToNot(HaveOccurred())
}

var _ = Describe("UpdateDeviceIssues (stale data)", func() {
	var f *updateDeviceIssuesTestHelper

	BeforeEach(func() { f = newUpdateDeviceIssuesTestHelper() })
	AfterEach(func() { f.cleanup() })

	It("sets staleData for a connected source whose latest data is older than the threshold", func() {
		latest := time.Now().Add(-1000 * time.Hour)
		patient := patientsTest.RandomPatient()
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", LatestDataTime: &latest},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.StaleData.ProviderId).To(Equal("dexcom"))
		// effectiveTime is the moment the data became stale: latest + threshold.
		Expect(updated.DeviceIssues.StaleData.EffectiveTime).
			To(BeTemporally("~", latest.Add(48*time.Hour), time.Millisecond))
	})

	It("does not set staleData for a connected source with recent data", func() {
		latest := time.Now().Add(-time.Hour)
		patient := patientsTest.RandomPatient()
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", LatestDataTime: &latest},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.StaleData.IsZero()).To(BeTrue())
	})

	It("picks the newest stale connected source", func() {
		older := time.Now().Add(-1000 * time.Hour)
		newer := time.Now().Add(-100 * time.Hour)
		patient := patientsTest.RandomPatient()
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", LatestDataTime: &older},
			{ProviderName: "abbott", State: "connected", LatestDataTime: &newer},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.StaleData.ProviderId).To(Equal("abbott"))
		Expect(updated.DeviceIssues.StaleData.EffectiveTime).
			To(BeTemporally("~", newer.Add(48*time.Hour), time.Millisecond))
	})

	It("considers only the primary device's data source when one is set", func() {
		older := time.Now().Add(-1000 * time.Hour)
		newer := time.Now().Add(-100 * time.Hour)
		primary := "dexcom"
		patient := patientsTest.RandomPatient()
		patient.PrimaryDeviceProviderName = &primary
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", LatestDataTime: &older},
			// Newer, so it would win if the primary device were not set.
			{ProviderName: "abbott", State: "connected", LatestDataTime: &newer},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.StaleData.ProviderId).To(Equal("dexcom"))
		Expect(updated.DeviceIssues.StaleData.EffectiveTime).
			To(BeTemporally("~", older.Add(48*time.Hour), time.Millisecond))
	})

	It("does not set staleData when only a non-primary source is stale", func() {
		stale := time.Now().Add(-1000 * time.Hour)
		recent := time.Now().Add(-time.Hour)
		primary := "dexcom"
		patient := patientsTest.RandomPatient()
		patient.PrimaryDeviceProviderName = &primary
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", LatestDataTime: &recent},
			{ProviderName: "abbott", State: "connected", LatestDataTime: &stale},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.StaleData.IsZero()).To(BeTrue())
	})
})

var _ = Describe("UpdateDeviceIssues (expired connection invitations)", func() {
	var f *updateDeviceIssuesTestHelper

	BeforeEach(func() { f = newUpdateDeviceIssuesTestHelper() })
	AfterEach(func() { f.cleanup() })

	It("sets expiredConnectionInvitation for a request whose expiration is in the past", func() {
		expiration := time.Now().Add(-time.Hour)
		patient := patientsTest.RandomPatient()
		patient.ProviderConnectionRequests = patients.ProviderConnectionRequests{
			"dexcom": patients.ConnectionRequests{
				{ProviderName: "dexcom", CreatedTime: time.Now().Add(-2 * time.Hour), ExpirationTime: expiration},
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.ExpiredConnectionInvitation.ProviderId).To(Equal("dexcom"))
		Expect(updated.DeviceIssues.ExpiredConnectionInvitation.EffectiveTime).
			To(BeTemporally("~", expiration, time.Millisecond))
	})

	It("does not set expiredConnectionInvitation for a request expiring in the future", func() {
		patient := patientsTest.RandomPatient()
		patient.ProviderConnectionRequests = patients.ProviderConnectionRequests{
			"dexcom": patients.ConnectionRequests{
				{ProviderName: "dexcom", CreatedTime: time.Now(), ExpirationTime: time.Now().Add(time.Hour)},
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.ExpiredConnectionInvitation.IsZero()).To(BeTrue())
	})

	It("picks the most-recently-expired invitation across providers", func() {
		olderExp := time.Now().Add(-10 * time.Hour)
		newerExp := time.Now().Add(-time.Hour)
		created := time.Now().Add(-2 * time.Hour)
		patient := patientsTest.RandomPatient()
		patient.ProviderConnectionRequests = patients.ProviderConnectionRequests{
			"dexcom": patients.ConnectionRequests{
				{ProviderName: "dexcom", CreatedTime: created, ExpirationTime: olderExp},
			},
			"abbott": patients.ConnectionRequests{
				{ProviderName: "abbott", CreatedTime: created, ExpirationTime: newerExp},
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.ExpiredConnectionInvitation.ProviderId).To(Equal("abbott"))
		Expect(updated.DeviceIssues.ExpiredConnectionInvitation.EffectiveTime).
			To(BeTemporally("~", newerExp, time.Millisecond))
	})
})

var _ = Describe("UpdateDeviceIssues (stale connection invitations)", func() {
	var f *updateDeviceIssuesTestHelper

	BeforeEach(func() { f = newUpdateDeviceIssuesTestHelper() })
	AfterEach(func() { f.cleanup() })

	It("sets staleConnectionInvitation for a request created more than the threshold ago", func() {
		created := time.Now().Add(-60 * time.Hour)
		patient := patientsTest.RandomPatient()
		patient.ProviderConnectionRequests = patients.ProviderConnectionRequests{
			"dexcom": patients.ConnectionRequests{
				// Future expiration so this is stale, not expired.
				{ProviderName: "dexcom", CreatedTime: created, ExpirationTime: time.Now().Add(time.Hour)},
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.StaleConnectionInvitation.ProviderId).To(Equal("dexcom"))
		Expect(updated.DeviceIssues.StaleConnectionInvitation.EffectiveTime).
			To(BeTemporally("~", created.Add(48*time.Hour), time.Millisecond))
	})

	It("does not set staleConnectionInvitation for a recently created request", func() {
		patient := patientsTest.RandomPatient()
		patient.ProviderConnectionRequests = patients.ProviderConnectionRequests{
			"dexcom": patients.ConnectionRequests{
				{ProviderName: "dexcom", CreatedTime: time.Now().Add(-time.Hour), ExpirationTime: time.Now().Add(time.Hour)},
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.StaleConnectionInvitation.IsZero()).To(BeTrue())
	})
})

var _ = Describe("UpdateDeviceIssues (resolved issues removal)", func() {
	var f *updateDeviceIssuesTestHelper

	BeforeEach(func() { f = newUpdateDeviceIssuesTestHelper() })
	AfterEach(func() { f.cleanup() })

	It("removes deviceIssues.disconnected when the device has reconnected", func() {
		now := time.Now()
		patient := patientsTest.RandomPatient()
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", ModifiedTime: &now, LatestDataTime: &now},
		}
		patient.DeviceIssues = patients.DeviceIssues{
			Disconnected: patients.DeviceIssue{
				EffectiveTime: now.Add(-24 * time.Hour),
				ProviderId:    "dexcom",
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.Disconnected.IsZero()).To(BeTrue())
	})

	It("removes deviceIssues.erroring when the device has recovered", func() {
		now := time.Now()
		patient := patientsTest.RandomPatient()
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", ModifiedTime: &now, LatestDataTime: &now},
		}
		patient.DeviceIssues = patients.DeviceIssues{
			Erroring: patients.DeviceIssue{
				EffectiveTime: now.Add(-24 * time.Hour),
				ProviderId:    "dexcom",
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.Erroring.IsZero()).To(BeTrue())
	})

	It("does not remove deviceIssues.disconnected while any device remains disconnected", func() {
		now := time.Now()
		modTime := now.Add(-time.Hour)
		patient := patientsTest.RandomPatient()
		// dexcom reconnected, but abbott is still disconnected: the issue is
		// re-pointed at abbott rather than removed.
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", ModifiedTime: &now, LatestDataTime: &now},
			{ProviderName: "abbott", State: "disconnected", ModifiedTime: &modTime},
		}
		patient.DeviceIssues = patients.DeviceIssues{
			Disconnected: patients.DeviceIssue{
				EffectiveTime: now.Add(-24 * time.Hour),
				ProviderId:    "dexcom",
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.Disconnected.ProviderId).To(Equal("abbott"))
		Expect(updated.DeviceIssues.Disconnected.EffectiveTime).
			To(BeTemporally("~", modTime, time.Millisecond))
	})

	It("removes deviceIssues.staleData when the primary device's data is no longer stale", func() {
		now := time.Now()
		latest := now.Add(-time.Hour)
		patient := patientsTest.RandomPatient()
		patient.PrimaryDeviceProviderName = strp("dexcom")
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", ModifiedTime: &now, LatestDataTime: &latest},
		}
		patient.DeviceIssues = patients.DeviceIssues{
			StaleData: patients.DeviceIssue{
				EffectiveTime: now.Add(-24 * time.Hour),
				ProviderId:    "dexcom",
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.StaleData.IsZero()).To(BeTrue())
	})

	It("does not remove deviceIssues.staleData while the primary device's data remains stale", func() {
		now := time.Now()
		latest := now.Add(-1000 * time.Hour)
		patient := patientsTest.RandomPatient()
		patient.PrimaryDeviceProviderName = strp("dexcom")
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", ModifiedTime: &now, LatestDataTime: &latest},
		}
		patient.DeviceIssues = patients.DeviceIssues{
			StaleData: patients.DeviceIssue{
				EffectiveTime: latest.Add(48 * time.Hour),
				ProviderId:    "dexcom",
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.StaleData.ProviderId).To(Equal("dexcom"))
		Expect(updated.DeviceIssues.StaleData.EffectiveTime).
			To(BeTemporally("~", latest.Add(48*time.Hour), time.Millisecond))
	})

	It("does not remove deviceIssues.staleData when only a non-primary source has recent data", func() {
		now := time.Now()
		stale := now.Add(-1000 * time.Hour)
		recent := now.Add(-time.Hour)
		patient := patientsTest.RandomPatient()
		patient.PrimaryDeviceProviderName = strp("dexcom")
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", ModifiedTime: &now, LatestDataTime: &stale},
			{ProviderName: "abbott", State: "connected", ModifiedTime: &now, LatestDataTime: &recent},
		}
		patient.DeviceIssues = patients.DeviceIssues{
			StaleData: patients.DeviceIssue{
				EffectiveTime: stale.Add(48 * time.Hour),
				ProviderId:    "dexcom",
			},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.StaleData.ProviderId).To(Equal("dexcom"))
	})
})

var _ = Describe("UpdateDeviceIssues (general behavior)", func() {
	var f *updateDeviceIssuesTestHelper

	BeforeEach(func() { f = newUpdateDeviceIssuesTestHelper() })
	AfterEach(func() { f.cleanup() })

	It("leaves a patient with no detectable issues untouched", func() {
		patient := patientsTest.RandomPatient()
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "connected", LatestDataTime: pointerTime(time.Now())},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.IsZero()).To(BeTrue())
	})

	It("records multiple distinct issue types on a single patient in one pass", func() {
		disconnectedMod := time.Now().Add(-time.Hour)
		staleLatest := time.Now().Add(-1000 * time.Hour)
		patient := patientsTest.RandomPatient()
		patient.DataSources = &[]patients.DataSource{
			{ProviderName: "dexcom", State: "disconnected", ModifiedTime: &disconnectedMod},
			{ProviderName: "abbott", State: "connected", LatestDataTime: &staleLatest},
		}
		id := f.insertPatient(patient)

		Expect(f.repo.UpdateDeviceIssues(f.ctx)).To(Succeed())

		updated := f.fetchPatient(id)
		Expect(updated.DeviceIssues.Disconnected.ProviderId).To(Equal("dexcom"))
		Expect(updated.DeviceIssues.Disconnected.EffectiveTime).
			To(BeTemporally("~", disconnectedMod, time.Millisecond))
		Expect(updated.DeviceIssues.StaleData.ProviderId).To(Equal("abbott"))
		Expect(updated.DeviceIssues.StaleData.EffectiveTime).
			To(BeTemporally("~", staleLatest.Add(48*time.Hour), time.Millisecond))
	})
})

func pointerTime(t time.Time) *time.Time { return &t }
