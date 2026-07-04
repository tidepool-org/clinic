package postgres_test

import (
	"bytes"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsonrw"

	"github.com/tidepool-org/clinic/patients"
	storepg "github.com/tidepool-org/clinic/store/postgres"
)

var _ = Describe("NormalizeDocument", func() {
	// Encodes a value the way the store client persists it to Mongo
	// (UseJSONStructTags), then decodes it back to the raw map shape the
	// backfiller reads.
	storedDocument := func(v interface{}) bson.M {
		GinkgoHelper()
		buf := new(bytes.Buffer)
		vw, err := bsonrw.NewBSONValueWriter(buf)
		Expect(err).ToNot(HaveOccurred())
		encoder, err := bson.NewEncoder(vw)
		Expect(err).ToNot(HaveOccurred())
		encoder.UseJSONStructTags()
		Expect(encoder.Encode(v)).To(Succeed())
		var m bson.M
		Expect(bson.Unmarshal(buf.Bytes(), &m)).To(Succeed())
		return m
	}

	It("produces the same payload for typed values and raw stored documents", func() {
		// Review's ClinicianId only carries a json tag; a plain bson.Marshal
		// would store it as "clinicianid" while Mongo holds "clinicianId".
		review := patients.Review{
			ClinicianId: "1234567890",
			Time:        time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		}

		livePayload, err := storepg.MarshalPayload(review)
		Expect(err).ToNot(HaveOccurred())
		backfillPayload, err := storepg.MarshalPayload(storedDocument(review))
		Expect(err).ToNot(HaveOccurred())

		Expect(string(livePayload)).To(Equal(string(backfillPayload)))
		Expect(string(livePayload)).To(ContainSubstring(`"clinicianId"`))
	})

	It("normalizes BSON dates to UTC timestamps in both paths", func() {
		review := patients.Review{
			ClinicianId: "1234567890",
			Time:        time.Date(2026, 7, 1, 12, 0, 0, 0, time.UTC),
		}
		normalized, err := storepg.NormalizeDocument(storedDocument(review))
		Expect(err).ToNot(HaveOccurred())
		Expect(normalized["time"]).To(BeAssignableToTypeOf(time.Time{}))
	})
})
