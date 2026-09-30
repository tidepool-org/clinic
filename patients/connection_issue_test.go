package patients_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"

	"github.com/tidepool-org/clinic/patients"
)

var _ = Describe("ConnectionIssue", func() {
	var now time.Time

	BeforeEach(func() {
		now = time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	})

	timep := func(t time.Time) *time.Time { return &t }

	// patient returns a dexcom-sourced patient with the given data sources and requests.
	patient := func(sources []patients.DataSource,
		requests ...patients.ConnectionRequest) patients.Patient {

		p := patients.Patient{ConnectionIssueSource: patients.ConnectionIssueSourceDexcom}
		if sources != nil {
			p.DataSources = &sources
		}
		if len(requests) > 0 {
			p.ProviderConnectionRequests = patients.ProviderConnectionRequests{
				patients.DexcomDataSourceProviderName: requests,
			}
		}
		return p
	}

	dexcom := func(state string) patients.DataSource {
		return patients.DataSource{
			ProviderName: patients.DexcomDataSourceProviderName,
			State:        state,
		}
	}

	request := func(created time.Time) patients.ConnectionRequest {
		return patients.ConnectionRequest{
			ProviderName:   patients.DexcomDataSourceProviderName,
			CreatedTime:    created,
			ExpirationTime: created.Add(patients.PendingDataSourceExpirationDuration),
		}
	}

	expectIssue := func(got *patients.ConnectionIssue,
		cause patients.ConnectionIssueCause) {

		GinkgoHelper()
		Expect(got).To(PointTo(MatchFields(IgnoreExtras, Fields{
			"Cause": Equal(cause),
		})))
	}

	Describe("DetectConnectionIssue", func() {
		It("is nil without a connection issue source", func() {
			p := patient([]patients.DataSource{dexcom("error")})
			p.ConnectionIssueSource = ""

			Expect(p.DetectConnectionIssue(now)).To(BeNil())
		})

		Describe("invitation issues", func() {
			// invited returns a custodial patient with an outstanding device non-specific
			// invitation, created and last invited the given durations ago.
			invited := func(createdAgo, sentAgo time.Duration) patients.Patient {
				custodial := patients.Permissions{Custodian: &patients.Permission{}}
				return patients.Patient{
					ConnectionIssueSource: patients.
						ConnectionIssueSourceDeviceNonSpecificInvitation,
					Permissions:        &custodial,
					CreatedTime:        now.Add(-createdAgo),
					LastInvitationSent: now.Add(-sentAgo),
				}
			}

			It("is nil while the invitation is fresh", func() {
				p := invited(24*time.Hour, 24*time.Hour)

				Expect(p.DetectConnectionIssue(now)).To(BeNil())
			})

			It("is stale when the invitation was last sent over 48 hours ago", func() {
				p := invited(10*24*time.Hour, 72*time.Hour)

				expectIssue(p.DetectConnectionIssue(now),
					patients.ConnectionIssueCauseStaleInvite)
			})

			It("is expired once the patient is over 31 days old", func() {
				p := invited(40*24*time.Hour, 72*time.Hour)

				expectIssue(p.DetectConnectionIssue(now),
					patients.ConnectionIssueCauseExpiredInvite)
			})

			It("stays expired after a recent reminder", func() {
				p := invited(40*24*time.Hour, time.Hour)

				expectIssue(p.DetectConnectionIssue(now),
					patients.ConnectionIssueCauseExpiredInvite)
			})

			It("falls back to the creation time when the sent time is unknown", func() {
				p := invited(40*24*time.Hour, 0)
				p.LastInvitationSent = time.Time{}

				expectIssue(p.DetectConnectionIssue(now),
					patients.ConnectionIssueCauseExpiredInvite)
			})

			It("is nil once the patient has claimed the account", func() {
				p := invited(40*24*time.Hour, 72*time.Hour)
				p.Permissions = &patients.Permissions{View: &patients.Permission{}}

				Expect(p.DetectConnectionIssue(now)).To(BeNil())
			})

			It("is nil once the patient has a data source", func() {
				p := invited(40*24*time.Hour, 72*time.Hour)
				p.DataSources = &[]patients.DataSource{dexcom("connected")}

				Expect(p.DetectConnectionIssue(now)).To(BeNil())
			})

			It("is nil when a provider connection request exists", func() {
				p := invited(40*24*time.Hour, 72*time.Hour)
				p.ProviderConnectionRequests = patients.ProviderConnectionRequests{
					patients.DexcomDataSourceProviderName: {request(now)},
				}

				Expect(p.DetectConnectionIssue(now)).To(BeNil())
			})
		})

		It("is nil for a connected source with fresh data", func() {
			source := dexcom("connected")
			source.LatestDataTime = timep(now.Add(-time.Hour))

			Expect(patient([]patients.DataSource{source}).DetectConnectionIssue(now)).
				To(BeNil())
		})

		Describe("state issues", func() {
			It("reports an error", func() {
				got := patient([]patients.DataSource{dexcom("error")}).
					DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseError)
			})

			It("reports a disconnection", func() {
				got := patient([]patients.DataSource{dexcom("disconnected")}).
					DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseDisconnected)
			})

			It("takes precedence over stale data", func() {
				source := dexcom("error")
				source.LatestDataTime = timep(now.Add(-72 * time.Hour))

				got := patient([]patients.DataSource{source}).DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseError)
			})
		})

		Describe("stale data", func() {
			It("is reported when the latest data is over 48 hours old", func() {
				source := dexcom("connected")
				source.LatestDataTime = timep(now.Add(-72 * time.Hour))

				got := patient([]patients.DataSource{source}).DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseStaleData)
			})

			It("is not reported without any data", func() {
				Expect(patient([]patients.DataSource{dexcom("connected")}).
					DetectConnectionIssue(now)).To(BeNil())
			})

			It("ignores sources of other providers", func() {
				source := dexcom("connected")
				source.LatestDataTime = timep(now.Add(-72 * time.Hour))
				source.ProviderName = patients.TwiistDataSourceProviderName

				Expect(patient([]patients.DataSource{source}).
					DetectConnectionIssue(now)).To(BeNil())
			})
		})

		Describe("connection requests", func() {
			It("reports a stale request", func() {
				sent := now.Add(-72 * time.Hour)

				got := patient(nil, request(sent)).DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseStaleInvite)
			})

			It("reports an expired request", func() {
				sent := now.Add(-patients.PendingDataSourceExpirationDuration - time.Hour)

				got := patient(nil, request(sent)).DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseExpiredInvite)
			})

			It("derives the expiration of a request without one", func() {
				sent := now.Add(-patients.PendingDataSourceExpirationDuration - time.Hour)
				legacy := request(sent)
				legacy.ExpirationTime = time.Time{}

				got := patient(nil, legacy).DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseExpiredInvite)
			})

			It("treats a request without an expiration as stale before 30 days", func() {
				sent := now.Add(-patients.PendingDataSourceExpirationDuration + time.Hour)
				legacy := request(sent)
				legacy.ExpirationTime = time.Time{}

				got := patient(nil, legacy).DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseStaleInvite)
			})

			It("honours a recorded expiration over the derived one", func() {
				sent := now.Add(-72 * time.Hour)
				early := request(sent)
				early.ExpirationTime = now.Add(-time.Hour)

				got := patient(nil, early).DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseExpiredInvite)
			})

			It("does not report a fresh request", func() {
				Expect(patient(nil, request(now.Add(-time.Hour))).
					DetectConnectionIssue(now)).To(BeNil())
			})

			It("ignores a request accepted by a newer data source", func() {
				sent := now.Add(-72 * time.Hour)
				source := dexcom("connected")
				source.CreatedTime = timep(sent.Add(time.Hour))
				source.LatestDataTime = timep(now)

				Expect(patient([]patients.DataSource{source}, request(sent)).
					DetectConnectionIssue(now)).To(BeNil())
			})

			It("treats a legacy source without any times as having accepted", func() {
				sent := now.Add(-72 * time.Hour)
				source := dexcom("")

				Expect(patient([]patients.DataSource{source}, request(sent)).
					DetectConnectionIssue(now)).To(BeNil())

				// The same source with a created time before the request leaves it pending
				source.CreatedTime = timep(sent.Add(-time.Hour))
				got := patient([]patients.DataSource{source}, request(sent)).
					DetectConnectionIssue(now)
				expectIssue(got, patients.ConnectionIssueCauseStaleInvite)
			})

			It("still reports the state of a legacy source that accepted", func() {
				sent := now.Add(-72 * time.Hour)
				source := dexcom("error")
				source.ModifiedTime = timep(now.Add(-24 * time.Hour))

				got := patient([]patients.DataSource{source}, request(sent)).
					DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseError)
			})

			It("treats a request as pending when a legacy source changed before it", func() {
				sent := now.Add(-72 * time.Hour)
				source := dexcom("")
				source.ModifiedTime = timep(sent.Add(-time.Hour))

				got := patient([]patients.DataSource{source}, request(sent)).
					DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseStaleInvite)
			})

			It("ignores a request answered by reconnecting an older source", func() {
				sent := now.Add(-72 * time.Hour)
				source := dexcom("connected")
				source.CreatedTime = timep(sent.Add(-30 * 24 * time.Hour))
				source.ConnectedTime = timep(sent.Add(time.Hour))
				source.LatestDataTime = timep(now)

				Expect(patient([]patients.DataSource{source}, request(sent)).
					DetectConnectionIssue(now)).To(BeNil())
			})

			It("ignores a request when the source is connected since before it", func() {
				sent := now.Add(-72 * time.Hour)
				source := dexcom("connected")
				source.CreatedTime = timep(sent.Add(-time.Hour))
				source.ConnectedTime = timep(sent.Add(-time.Hour))
				source.LatestDataTime = timep(now)

				Expect(patient([]patients.DataSource{source}, request(sent)).
					DetectConnectionIssue(now)).To(BeNil())
			})

			It("treats a request as pending when a never connected source is older than it",
				func() {
					sent := now.Add(-72 * time.Hour)
					source := dexcom("")
					source.CreatedTime = timep(sent.Add(-time.Hour))

					got := patient([]patients.DataSource{source}, request(sent)).
						DetectConnectionIssue(now)

					expectIssue(got, patients.ConnectionIssueCauseStaleInvite)
				})

			It("uses the most recent request regardless of order", func() {
				old := now.Add(-patients.PendingDataSourceExpirationDuration - time.Hour)
				recent := now.Add(-time.Hour)

				Expect(patient(nil, request(old), request(recent)).
					DetectConnectionIssue(now)).To(BeNil())
				Expect(patient(nil, request(recent), request(old)).
					DetectConnectionIssue(now)).To(BeNil())
			})

			It("ranks below stale data", func() {
				sent := now.Add(-patients.PendingDataSourceExpirationDuration - time.Hour)
				source := dexcom("connected")
				source.LatestDataTime = timep(now.Add(-72 * time.Hour))

				got := patient([]patients.DataSource{source}, request(sent)).
					DetectConnectionIssue(now)

				expectIssue(got, patients.ConnectionIssueCauseStaleData)
			})
		})
	})

	Describe("hidden", func() {
		staleSource := func() []patients.DataSource {
			source := dexcom("connected")
			source.LatestDataTime = timep(now.Add(-72 * time.Hour))
			return []patients.DataSource{source}
		}

		It("is false without a stored issue", func() {
			got := patient(staleSource()).DetectConnectionIssue(now)

			Expect(got).ToNot(BeNil())
			Expect(got.Hidden).To(BeFalse())
		})

		It("is kept while the cause stays the same", func() {
			p := patient(staleSource())
			p.ConnectionIssue = &patients.ConnectionIssue{
				Cause:  patients.ConnectionIssueCauseStaleData,
				Hidden: true,
			}

			got := p.DetectConnectionIssue(now)

			expectIssue(got, patients.ConnectionIssueCauseStaleData)
			Expect(got.Hidden).To(BeTrue())
		})

		It("is dropped when the cause changes", func() {
			p := patient(staleSource())
			p.ConnectionIssue = &patients.ConnectionIssue{
				Cause:  patients.ConnectionIssueCauseError,
				Hidden: true,
			}

			got := p.DetectConnectionIssue(now)

			Expect(got.Cause).To(Equal(patients.ConnectionIssueCauseStaleData))
			Expect(got.Hidden).To(BeFalse())
		})
	})

	Describe("ParseConnectionIssueCause", func() {
		It("accepts every known cause", func() {
			for _, cause := range []patients.ConnectionIssueCause{
				patients.ConnectionIssueCauseError,
				patients.ConnectionIssueCauseDisconnected,
				patients.ConnectionIssueCauseStaleData,
				patients.ConnectionIssueCauseStaleInvite,
				patients.ConnectionIssueCauseExpiredInvite,
			} {
				got, ok := patients.ParseConnectionIssueCause(string(cause))
				Expect(ok).To(BeTrue(), string(cause))
				Expect(got).To(Equal(cause))
			}
		})

		It("rejects unknown values", func() {
			for _, value := range []string{"", "bogus", "Error", "stale_data"} {
				_, ok := patients.ParseConnectionIssueCause(value)
				Expect(ok).To(BeFalse(), value)
			}
		})
	})

	Describe("Equal", func() {
		issue := func(cause patients.ConnectionIssueCause) *patients.ConnectionIssue {
			return &patients.ConnectionIssue{Cause: cause}
		}

		It("treats two absent issues as equal", func() {
			var a, b *patients.ConnectionIssue
			Expect(a.Equal(b)).To(BeTrue())
		})

		It("treats an absent and a present issue as different", func() {
			var absent *patients.ConnectionIssue
			present := issue(patients.ConnectionIssueCauseError)
			Expect(absent.Equal(present)).To(BeFalse())
			Expect(present.Equal(absent)).To(BeFalse())
		})

		It("ignores hidden", func() {
			a := issue(patients.ConnectionIssueCauseError)
			b := issue(patients.ConnectionIssueCauseError)
			b.Hidden = true
			Expect(a.Equal(b)).To(BeTrue())
		})

		It("compares the cause", func() {
			a := issue(patients.ConnectionIssueCauseError)
			Expect(a.Equal(issue(patients.ConnectionIssueCauseError))).To(BeTrue())
			Expect(a.Equal(issue(patients.ConnectionIssueCauseDisconnected))).
				To(BeFalse())
		})
	})
})

var _ = Describe("DataSource.Accepted", func() {
	sent := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	before := sent.Add(-time.Hour)
	after := sent.Add(time.Hour)
	request := patients.ConnectionRequest{
		ProviderName: patients.DexcomDataSourceProviderName,
		CreatedTime:  sent,
	}

	DescribeTable("reports whether the source answered the request",
		func(state string, created, modified, connected *time.Time, expected bool) {
			source := patients.DataSource{
				ProviderName:  patients.DexcomDataSourceProviderName,
				State:         state,
				CreatedTime:   created,
				ModifiedTime:  modified,
				ConnectedTime: connected,
			}
			Expect(source.Accepted(request)).To(Equal(expected))
		},
		Entry("created after the request", "disconnected", &after, nil, nil, true),
		Entry("created before the request", "disconnected", &before, nil, nil, false),
		Entry("created at the request", "disconnected", &sent, nil, nil, false),
		Entry("legacy, modified after the request", "disconnected", nil, &after, nil, true),
		Entry("legacy, modified before the request",
			"disconnected", nil, &before, nil, false),
		Entry("legacy, without created or modified times",
			"disconnected", nil, nil, nil, true),

		// A healthy connection wins, however long it has been connected.
		Entry("connected, created before the request",
			"connected", &before, nil, nil, true),
		Entry("connected, connected before the request",
			"connected", &before, &after, &before, true),

		// A reconnected source keeps its created time, but its connected time is reset.
		Entry("disconnected, connected after the request",
			"disconnected", &before, &after, &after, true),
		Entry("errored, connected after the request",
			"error", &before, &after, &after, true),
		Entry("disconnected, connected before the request",
			"disconnected", &before, &after, &before, false),
		Entry("errored, connected at the request", "error", &before, &after, &sent, false),

		// The platform modifies sources without the patient doing anything.
		Entry("disconnected, created before and modified after the request",
			"disconnected", &before, &after, nil, false),
		Entry("errored, created before and modified after the request",
			"error", &before, &after, nil, false),
	)
})
