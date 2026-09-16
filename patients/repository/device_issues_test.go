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
	expired := patients.PrimaryIssueKindExpiredInvite
	staleInvite := patients.PrimaryIssueKindStaleInvite
	staleDataKind := patients.PrimaryIssueKindStaleData

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

	// dataSourceWithData builds a data source in the given state whose latest data arrived
	// at latestDataTime.
	dataSourceWithData := func(
		provider, state string, createdTime, latestDataTime time.Time,
	) patients.DataSource {
		return patients.DataSource{
			ProviderName:   provider,
			State:          state,
			CreatedTime:    &createdTime,
			LatestDataTime: &latestDataTime,
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
		// expire: lapsed and longLapsed have expired, an hour and two hours ago. current
		// was created an hour ago, so it has neither expired nor gone stale.
		var lapsed, longLapsed, current time.Time

		BeforeEach(func() {
			expiresNow := now.Add(-patients.PendingDataSourceExpirationDuration)
			lapsed = expiresNow.Add(-time.Hour)
			longLapsed = expiresNow.Add(-2 * time.Hour)
			current = now.Add(-time.Hour)
		})

		It("classifies the issue when the invite expired with no data source", func() {
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

		It("never expires an invitation without an expiration time", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {{ProviderName: dexcom, CreatedTime: lapsed}},
			})
			update()
			// Old enough to have gone stale, which is the most the check can conclude.
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", staleInvite)))
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

	Describe("stale invitations", func() {
		// Creation times for requests, relative to when a request created now would go
		// stale: stale went stale an hour ago, fresh goes stale in an hour, and lapsed
		// has expired as well as gone stale.
		var stale, fresh, lapsed time.Time
		// staleAt is when the stale request went stale.
		var staleAt time.Time

		BeforeEach(func() {
			staleNow := now.Add(-patients.PendingDataSourceStaleDuration)
			stale = staleNow.Add(-time.Hour)
			fresh = staleNow.Add(time.Hour)
			lapsed = now.Add(-patients.PendingDataSourceExpirationDuration - time.Hour)
			staleAt = stale.Add(patients.PendingDataSourceStaleDuration)
		})

		It("classifies the issue when the invitation went stale unaccepted", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, stale)},
			})
			update()
			got := get(s)
			Expect(got.PrimaryIssue).To(PointTo(Equal(patients.PrimaryIssue{
				Source:        dexcom,
				Kind:          staleInvite,
				EffectiveTime: staleAt,
			})))
			Expect(got.UpdatedTime).To(BeTemporally("~", time.Now(), time.Second))
		})

		It("ignores an invitation that was accepted", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, stale)},
			}, dataSource(dexcom, stale.Add(time.Minute)))
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("classifies the issue when the data source predates the invitation", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, stale)},
			}, dataSource(dexcom, stale.Add(-time.Minute)))
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", staleInvite)))
		})

		It("ignores a fresh invitation", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, fresh)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("considers only the newest invitation", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, fresh), request(dexcom, stale)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("classifies an expired invitation as expired rather than stale", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, lapsed)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", expired)))
		})

		It("ignores stale invitations for other providers", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				twiist: {request(twiist, stale)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("ignores the device-non-specific invitation", func() {
			s := seed(issue(invite), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, stale)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(invite)))
		})

		It("ignores patients without a primary issue", func() {
			s := seed(nil, patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, stale)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(BeNil())
		})

		It("replaces an existing kind", func() {
			classified := issue(dexcom)
			classified.Kind = patients.PrimaryIssueKindStaleData
			s := seed(classified, patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, stale)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", staleInvite)))
		})

		It("leaves an already classified patient alone", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, stale)},
			})
			update()
			first := get(s)
			Expect(first.PrimaryIssue).To(PointTo(HaveField("Kind", staleInvite)))

			update()
			second := get(s)
			Expect(second.PrimaryIssue).To(Equal(first.PrimaryIssue))
			Expect(second.UpdatedTime).To(BeTemporally("==", first.UpdatedTime))
		})

		It("is superseded by a newer connection request", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, stale)},
			})
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", staleInvite)))

			// A new event replaces the primary issue wholesale: fresh source and
			// effective time, no kind.
			newer := request(twiist, now)
			Expect(repo.AddProviderConnectionRequest(ctx, s.clinicId, s.userId, newer)).
				To(Succeed())
			Expect(get(s).PrimaryIssue).To(PointTo(Equal(patients.PrimaryIssue{
				Source:        twiist,
				EffectiveTime: now,
			})))
		})
	})

	Describe("stale data", func() {
		// Times for a data source's latest data, relative to when data received now would
		// go stale: oldData went stale an hour ago, recentData goes stale in an hour. The
		// data source itself was created a day ago.
		var oldData, recentData, sourceCreated time.Time
		// staleAt is when oldData went stale.
		var staleAt time.Time
		connected := patients.DataSourceStateConnected

		BeforeEach(func() {
			staleNow := now.Add(-patients.DataSourceStaleDataDuration)
			oldData = staleNow.Add(-time.Hour)
			recentData = staleNow.Add(time.Hour)
			sourceCreated = now.Add(-24 * time.Hour)
			staleAt = oldData.Add(patients.DataSourceStaleDataDuration)
		})

		It("classifies the issue when connected data went stale", func() {
			s := seed(issue(dexcom), nil,
				dataSourceWithData(dexcom, connected, sourceCreated, oldData))
			update()
			got := get(s)
			Expect(got.PrimaryIssue).To(PointTo(Equal(patients.PrimaryIssue{
				Source:        dexcom,
				Kind:          staleDataKind,
				EffectiveTime: staleAt,
			})))
			Expect(got.UpdatedTime).To(BeTemporally("~", time.Now(), time.Second))
		})

		It("ignores recent data", func() {
			s := seed(issue(dexcom), nil,
				dataSourceWithData(dexcom, connected, sourceCreated, recentData))
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("ignores a data source without latest data", func() {
			s := seed(issue(dexcom), nil, dataSource(dexcom, sourceCreated))
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("ignores stale data from a disconnected data source", func() {
			disconnected := patients.DataSourceStateDisconnected
			s := seed(issue(dexcom), nil,
				dataSourceWithData(dexcom, disconnected, sourceCreated, oldData))
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("ignores stale data from an erroring data source", func() {
			s := seed(issue(dexcom), nil,
				dataSourceWithData(dexcom, patients.DataSourceStateError, sourceCreated,
					oldData))
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("classifies the issue when the request predates the data source", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, sourceCreated.Add(-time.Hour))},
			}, dataSourceWithData(dexcom, connected, sourceCreated, oldData))
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", staleDataKind)))
		})

		It("ignores stale data when a newer connection request exists", func() {
			s := seed(issue(dexcom), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, now.Add(-time.Hour))},
			}, dataSourceWithData(dexcom, connected, sourceCreated, oldData))
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("ignores stale data for other providers", func() {
			s := seed(issue(dexcom), nil,
				dataSourceWithData(twiist, connected, sourceCreated, oldData))
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("ignores the device-non-specific invitation", func() {
			s := seed(issue(invite), nil,
				dataSourceWithData(dexcom, connected, sourceCreated, oldData))
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(invite)))
		})

		It("ignores patients without a primary issue", func() {
			s := seed(nil, nil,
				dataSourceWithData(dexcom, connected, sourceCreated, oldData))
			update()
			Expect(get(s).PrimaryIssue).To(BeNil())
		})

		It("replaces an existing kind", func() {
			classified := issue(dexcom)
			classified.Kind = staleInvite
			s := seed(classified, nil,
				dataSourceWithData(dexcom, connected, sourceCreated, oldData))
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", staleDataKind)))
		})

		It("leaves an already classified patient alone", func() {
			s := seed(issue(dexcom), nil,
				dataSourceWithData(dexcom, connected, sourceCreated, oldData))
			update()
			first := get(s)
			Expect(first.PrimaryIssue).To(PointTo(HaveField("Kind", staleDataKind)))

			update()
			second := get(s)
			Expect(second.PrimaryIssue).To(Equal(first.PrimaryIssue))
			Expect(second.UpdatedTime).To(BeTemporally("==", first.UpdatedTime))
		})

		It("keeps the classification when data becomes recent again", func() {
			s := seed(issue(dexcom), nil,
				dataSourceWithData(dexcom, connected, sourceCreated, oldData))
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", staleDataKind)))

			// No criterion un-classifies an issue; only a new event replaces it.
			sources := patients.DataSources{
				dataSourceWithData(dexcom, connected, sourceCreated, recentData),
			}
			Expect(repo.UpdatePatientDataSources(ctx, s.userId, &sources)).To(Succeed())
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", staleDataKind)))
		})
	})

	Describe("stale device-non-specific invitations", func() {
		// Creation times for patients, relative to when a patient created now would see
		// its invitation go stale: old went stale an hour ago, recent goes stale in an
		// hour.
		var old, recent time.Time
		// staleAt is when the old patient's invitation went stale.
		var staleAt time.Time

		BeforeEach(func() {
			staleNow := now.Add(-patients.PendingDataSourceStaleDuration)
			old = staleNow.Add(-time.Hour)
			recent = staleNow.Add(time.Hour)
			staleAt = old.Add(patients.PendingDataSourceStaleDuration)
		})

		// agePatient sets the subject's creation time directly, since the repository
		// stamps it on creation.
		agePatient := func(s subject, createdTime time.Time) {
			GinkgoHelper()
			clinicId, err := primitive.ObjectIDFromHex(s.clinicId)
			Expect(err).ToNot(HaveOccurred())
			selector := bson.M{"clinicId": clinicId, "userId": s.userId}
			_, err = collection.UpdateOne(ctx, selector,
				bson.M{"$set": bson.M{"createdTime": createdTime}})
			Expect(err).ToNot(HaveOccurred())
		}

		It("classifies an old patient's unanswered invitation", func() {
			s := seed(issue(invite), nil)
			agePatient(s, old)
			update()
			got := get(s)
			Expect(got.PrimaryIssue).To(PointTo(Equal(patients.PrimaryIssue{
				Source:        invite,
				Kind:          staleInvite,
				EffectiveTime: staleAt,
			})))
			Expect(got.UpdatedTime).To(BeTemporally("~", time.Now(), time.Second))
		})

		It("ignores a recent patient", func() {
			s := seed(issue(invite), nil)
			agePatient(s, recent)
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(invite)))
		})

		It("ignores an old patient with a connection request", func() {
			s := seed(issue(invite), patients.ProviderConnectionRequests{
				dexcom: {request(dexcom, now.Add(-time.Hour))},
			})
			agePatient(s, old)
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(invite)))
		})

		It("ignores an old patient with a data source in any state", func() {
			disconnected := patients.DataSourceStateDisconnected
			s := seed(issue(invite), nil,
				dataSourceWithData(dexcom, disconnected, old, old))
			agePatient(s, old)
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(invite)))
		})

		It("does not classify a provider issue by the patient's age", func() {
			s := seed(issue(dexcom), nil)
			agePatient(s, old)
			update()
			Expect(get(s).PrimaryIssue).To(Equal(issue(dexcom)))
		})

		It("replaces an existing kind", func() {
			classified := issue(invite)
			classified.Kind = staleDataKind
			s := seed(classified, nil)
			agePatient(s, old)
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", staleInvite)))
		})

		It("leaves an already classified patient alone", func() {
			s := seed(issue(invite), nil)
			agePatient(s, old)
			update()
			first := get(s)
			Expect(first.PrimaryIssue).To(PointTo(HaveField("Kind", staleInvite)))

			update()
			second := get(s)
			Expect(second.PrimaryIssue).To(Equal(first.PrimaryIssue))
			Expect(second.UpdatedTime).To(BeTemporally("==", first.UpdatedTime))
		})

		It("is superseded by a later connection request", func() {
			s := seed(issue(invite), nil)
			agePatient(s, old)
			update()
			Expect(get(s).PrimaryIssue).To(PointTo(HaveField("Kind", staleInvite)))

			newer := request(dexcom, now)
			Expect(repo.AddProviderConnectionRequest(ctx, s.clinicId, s.userId, newer)).
				To(Succeed())
			Expect(get(s).PrimaryIssue).To(PointTo(Equal(patients.PrimaryIssue{
				Source:        dexcom,
				EffectiveTime: now,
			})))
		})
	})
})
