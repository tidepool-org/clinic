package xealth_test

import (
	"net/url"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/clinics"
	"github.com/tidepool-org/clinic/patients"
	"github.com/tidepool-org/clinic/sites"
	"github.com/tidepool-org/clinic/xealth"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var _ = Describe("GenerateReportUrl", func() {
	const baseUrl = "https://example.com"
	const token = "token"

	var patient patients.Patient
	var clinic clinics.Clinic
	var siteChangeSource string

	strp := func(s string) *string { return &s }

	tag := func(name string) clinics.PatientTag {
		id := primitive.NewObjectID()
		return clinics.PatientTag{Id: &id, Name: name}
	}

	site := func(name string) sites.Site {
		return sites.Site{Id: primitive.NewObjectID(), Name: name}
	}

	tagIds := func(tags ...clinics.PatientTag) *[]primitive.ObjectID {
		ids := make([]primitive.ObjectID, 0, len(tags))
		for _, t := range tags {
			ids = append(ids, *t.Id)
		}
		return &ids
	}

	generateQuery := func() url.Values {
		u, err := xealth.GenerateReportUrl(baseUrl, token, patient, clinic, siteChangeSource)
		Expect(err).ToNot(HaveOccurred())
		query, err := url.ParseQuery(u.RawQuery)
		Expect(err).ToNot(HaveOccurred())
		return query
	}

	BeforeEach(func() {
		patient = patients.Patient{
			UserId:    strp("1234567890"),
			BirthDate: strp("2000-01-01"),
			Mrn:       strp("mrn-1"),
			FullName:  strp("Test Patient"),
		}
		clinic = clinics.Clinic{}
		siteChangeSource = ""
	})

	It("adds the patient's tags and sites as repeated params in the clinic's order", func() {
		clinic.PatientTags = []clinics.PatientTag{tag("gamma"), tag("alpha"), tag("Beta")}
		clinic.Sites = []sites.Site{site("west"), site("East"), site("north")}
		patient.Tags = tagIds(clinic.PatientTags[2], clinic.PatientTags[1], clinic.PatientTags[0])
		patient.Sites = &[]sites.Site{clinic.Sites[2], clinic.Sites[1], clinic.Sites[0]}

		query := generateQuery()
		Expect(query["patientTags"]).To(Equal([]string{"gamma", "alpha", "Beta"}))
		Expect(query["sites"]).To(Equal([]string{"west", "East", "north"}))
	})

	It("round-trips names with reserved and non-ASCII characters", func() {
		names := []string{"A & B", "Café Ünïcode", "one+two", "this/that"}
		for _, name := range names {
			clinic.PatientTags = append(clinic.PatientTags, tag(name))
			clinic.Sites = append(clinic.Sites, site(name))
		}
		patient.Tags = tagIds(clinic.PatientTags...)
		patient.Sites = &clinic.Sites

		query := generateQuery()
		Expect(query["patientTags"]).To(Equal(names))
		Expect(query["sites"]).To(Equal(names))
	})

	It("skips a tag that is missing from the clinic and still sends the resolved tags", func() {
		clinic.PatientTags = []clinics.PatientTag{tag("alpha"), tag("beta")}
		patient.Tags = tagIds(clinic.PatientTags[0], tag("removed"), clinic.PatientTags[1])

		Expect(generateQuery()["patientTags"]).To(Equal([]string{"alpha", "beta"}))
	})

	It("sends the clinic's current name for a renamed site", func() {
		stored := site("Old Name")
		clinic.Sites = []sites.Site{{Id: stored.Id, Name: "New Name"}}
		patient.Sites = &[]sites.Site{stored}

		Expect(generateQuery()["sites"]).To(Equal([]string{"New Name"}))
	})

	It("skips a site that is missing from the clinic and still sends the resolved sites", func() {
		clinic.Sites = []sites.Site{site("East"), site("West")}
		patient.Sites = &[]sites.Site{clinic.Sites[0], site("Removed"), clinic.Sites[1]}

		Expect(generateQuery()["sites"]).To(Equal([]string{"East", "West"}))
	})

	It("omits the params when the patient has no tags or sites", func() {
		clinic.PatientTags = []clinics.PatientTag{tag("alpha")}
		clinic.Sites = []sites.Site{site("East")}

		query := generateQuery()
		Expect(query).ToNot(HaveKey("patientTags"))
		Expect(query).ToNot(HaveKey("sites"))
		Expect(query).To(Equal(url.Values{
			"reports":          {"all"},
			"inline":           {"true"},
			"userId":           {"1234567890"},
			"dob":              {"2000-01-01"},
			"mrn":              {"mrn-1"},
			"fullName":         {"Test Patient"},
			"restricted_token": {token},
		}))
	})

	DescribeTable("adds an allowed site change type as a single param",
		func(value string) {
			siteChangeSource = value
			Expect(generateQuery()["siteChangeSource"]).To(Equal([]string{value}))
		},
		Entry("cannulaPrime", "cannulaPrime"),
		Entry("reservoirChange", "reservoirChange"),
		Entry("tubingPrime", "tubingPrime"),
	)

	DescribeTable("omits a site change type that is not allowed and leaves the other params unchanged",
		func(value string) {
			expected := generateQuery()
			siteChangeSource = value

			query := generateQuery()
			Expect(query).ToNot(HaveKey("siteChangeSource"))
			Expect(query).To(Equal(expected))
		},
		Entry("an unknown value", "bogus"),
		Entry("a wrong-case value", "TubingPrime"),
	)

	It("omits an empty site change type and still sends the tags and sites", func() {
		clinic.PatientTags = []clinics.PatientTag{tag("alpha")}
		clinic.Sites = []sites.Site{site("East")}
		patient.Tags = tagIds(clinic.PatientTags[0])
		patient.Sites = &clinic.Sites

		query := generateQuery()
		Expect(query).ToNot(HaveKey("siteChangeSource"))
		Expect(query["patientTags"]).To(Equal([]string{"alpha"}))
		Expect(query["sites"]).To(Equal([]string{"East"}))
	})
})
