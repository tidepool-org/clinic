package patients_test

import (
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/tidepool-org/clinic/patients"
)

var _ = Describe("DataSources.NewlyConnected", func() {
	earlier := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	later := earlier.Add(time.Hour)
	source := func(provider, state string) patients.DataSource {
		return patients.DataSource{ProviderName: provider, State: state}
	}

	DescribeTable("returns the sources entering the connected state",
		func(existingState *string, incomingState string, expectReturned bool) {
			var existing patients.DataSources
			if existingState != nil {
				existing = patients.DataSources{source("dexcom", *existingState)}
			}
			incoming := patients.DataSources{source("dexcom", incomingState)}

			result := incoming.NewlyConnected(existing)

			if expectReturned {
				Expect(result).To(HaveLen(1))
				Expect(result[0].ProviderName).To(Equal("dexcom"))
			} else {
				Expect(result).To(BeEmpty())
			}
		},
		Entry("disconnected -> connected", strp("disconnected"), "connected", true),
		Entry("error -> connected", strp("error"), "connected", true),
		Entry("not stored -> connected", nil, "connected", true),
		Entry("connected -> connected", strp("connected"), "connected", false),
		Entry("connected -> disconnected", strp("connected"), "disconnected", false),
		Entry("not stored -> disconnected", nil, "disconnected", false),
		Entry("disconnected -> error", strp("disconnected"), "error", false),
	)

	DescribeTable("returns connected sources whose connected time advanced",
		func(existingTime, incomingTime *time.Time, expectReturned bool) {
			existing := source("dexcom", "connected")
			existing.ConnectedTime = existingTime
			incoming := source("dexcom", "connected")
			incoming.ConnectedTime = incomingTime

			result := patients.DataSources{incoming}.
				NewlyConnected(patients.DataSources{existing})

			if expectReturned {
				Expect(result).To(HaveLen(1))
			} else {
				Expect(result).To(BeEmpty())
			}
		},
		Entry("reconnected", &earlier, &later, true),
		Entry("unchanged", &earlier, &earlier, false),
		Entry("went back", &later, &earlier, false),
		// A backfilled connected time is not a reconnection.
		Entry("stored without a connected time", nil, &later, false),
		Entry("incoming without a connected time", &earlier, nil, false),
	)

	It("preserves the incoming order", func() {
		existing := patients.DataSources{source("dexcom", "connected")}
		incoming := patients.DataSources{
			source("abbott", "connected"),
			source("dexcom", "connected"),
			source("twiist", "connected"),
		}

		result := incoming.NewlyConnected(existing)

		Expect(result).To(HaveLen(2))
		Expect(result[0].ProviderName).To(Equal("abbott"))
		Expect(result[1].ProviderName).To(Equal("twiist"))
	})

	It("returns nil for a nil receiver", func() {
		var incoming patients.DataSources

		Expect(incoming.NewlyConnected(nil)).To(BeNil())
	})

	It("returns an empty slice when nothing enters the connected state", func() {
		incoming := patients.DataSources{source("dexcom", "disconnected")}

		result := incoming.NewlyConnected(nil)

		Expect(result).ToNot(BeNil())
		Expect(result).To(BeEmpty())
	})
})

var _ = Describe("DataSources.Newest", func() {
	var older, newer time.Time

	BeforeEach(func() {
		newer = time.Now().UTC().Truncate(time.Millisecond)
		older = newer.Add(-time.Hour)
	})

	source := func(provider string, created *time.Time) patients.DataSource {
		return patients.DataSource{
			ProviderName: provider,
			State:        "connected",
			CreatedTime:  created,
		}
	}

	It("returns nil when empty", func() {
		Expect(patients.DataSources{}.Newest()).To(BeNil())
		Expect(patients.DataSources(nil).Newest()).To(BeNil())
	})

	It("picks the latest created time", func() {
		sources := patients.DataSources{
			source("twiist", &older),
			source("abbott", &newer),
			source("dexcom", &older),
		}

		Expect(sources.Newest().ProviderName).To(Equal("abbott"))
	})

	It("treats a nil created time as the oldest", func() {
		sources := patients.DataSources{
			source("twiist", nil),
			source("abbott", &older),
		}

		Expect(sources.Newest().ProviderName).To(Equal("abbott"))
	})

	It("breaks ties by provider priority regardless of order", func() {
		Expect(patients.DataSources{
			source("abbott", &newer),
			source("dexcom", &newer),
			source("twiist", &newer),
		}.Newest().ProviderName).To(Equal("twiist"))

		Expect(patients.DataSources{
			source("abbott", &newer),
			source("dexcom", &newer),
		}.Newest().ProviderName).To(Equal("dexcom"))

		Expect(patients.DataSources{
			source("twiist", &newer),
			source("dexcom", &newer),
		}.Newest().ProviderName).To(Equal("twiist"))
	})

	It("ranks an unknown provider below any known provider in a tie", func() {
		sources := patients.DataSources{
			source("acme", &newer),
			source("abbott", &newer),
		}

		Expect(sources.Newest().ProviderName).To(Equal("abbott"))
	})

	It("falls back to provider priority when no created times are set", func() {
		sources := patients.DataSources{
			source("abbott", nil),
			source("twiist", nil),
		}

		Expect(sources.Newest().ProviderName).To(Equal("twiist"))
	})

	It("keeps the earlier of two otherwise equal sources", func() {
		sources := patients.DataSources{
			source("acme", &newer),
			source("zeta", &newer),
		}

		Expect(sources.Newest().ProviderName).To(Equal("acme"))
	})
})

func strp(s string) *string {
	return &s
}
