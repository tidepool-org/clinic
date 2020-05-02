package store_test


import (
	. "github.com/onsi/ginkgo"
	//. "github.com/onsi/ginkgo/extensions/table"
	"github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/store"

	//. "github.com/onsi/gomega"
)

// NewClinic defines model for NewClinic.
type NewTestDoc struct {
	Name         *string                 `json:"name,omitempty"`
	Metadata     *map[string]interface{} `json:"metadata,omitempty"`
	PhoneNumbers *[]struct {
		Number *string `json:"number,omitempty"`
		Type   *string `json:"type,omitempty"`
	} `json:"phoneNumbers,omitempty"`
}
type FullTestDoc struct {
	// Embedded fields due to inline allOf schema
	Id    *string `json:"clinicId,omitempty"`
	// Embedded struct due to allOf(#/components/schemas/ClinicianPermissions)
	NewTestDoc
}


var _ = Describe("Store Test", func() {
	Context("Database Operations", func() {
		Context("Parse", func() {
			// TODO
		})

		Context("InsertOne", func() {
			It("should populate write data", func() {
				name := "test"
				testDoc := NewTestDoc{Name: &name}
				_, err := mongoClient.InsertOne(store.ClinicsCollection, &testDoc)
				gomega.Expect(err).NotTo(gomega.HaveOccurred())
			})
		})
	})
})

