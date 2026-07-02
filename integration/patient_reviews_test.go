package integration_test

import (
	"fmt"
	"net/http"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/tidepool-org/clinic/client"
)

// Pins patient review semantics: reviews are a newest-first list, only the
// most recent review can be deleted and only by the clinician who created it,
// and the endpoints require user (not server) tokens.
var _ = Describe("Patient Reviews", Ordered, func() {
	var clinicianA, clinicianB string
	var clinicId string
	var patientId string

	deleteReview := func(reqAuth func(*http.Request)) *http.Response {
		GinkgoHelper()
		req := prepareRequest(http.MethodDelete,
			fmt.Sprintf("/v1/clinics/%s/patients/%s/reviews", clinicId, patientId), "")
		reqAuth(req)
		return do(req)
	}

	BeforeAll(func() {
		adminUser := newStubUser()
		clinicianA = adminUser.UserID
		clinicId = *createClinic(asUser(clinicianA)).Id

		memberUser := newStubUser()
		clinicianB = memberUser.UserID
		createClinicianDirect(clinicId, clinicianB)

		patient := createCustodialPatient(clinicId, asUser(clinicianA), nil)
		patientId = *patient.Id
	})

	It("rejects server tokens", func() {
		// The authorization policy only allows clinicians, so server tokens
		// are rejected before the handler's own token check.
		req := prepareRequest(http.MethodPut,
			fmt.Sprintf("/v1/clinics/%s/patients/%s/reviews", clinicId, patientId), "")
		asServer(req)
		expectStatus(do(req), http.StatusForbidden)

		req = prepareRequest(http.MethodDelete,
			fmt.Sprintf("/v1/clinics/%s/patients/%s/reviews", clinicId, patientId), "")
		asServer(req)
		expectStatus(do(req), http.StatusForbidden)
	})

	It("returns reviews newest first", func() {
		first := addReview(clinicId, patientId, asUser(clinicianA))
		Expect(first).To(HaveLen(1))
		Expect(first[0].ClinicianId).To(Equal(clinicianA))

		second := addReview(clinicId, patientId, asUser(clinicianB))
		Expect(second).To(HaveLen(2))
		Expect(second[0].ClinicianId).To(Equal(clinicianB))
		Expect(second[1].ClinicianId).To(Equal(clinicianA))
		Expect(second[0].Time.After(second[1].Time)).To(BeTrue())
	})

	It("includes reviews in the patient representation", func() {
		patient := getPatient(clinicId, patientId)
		Expect(patient.Reviews).To(HaveLen(2))
		Expect(patient.Reviews[0].ClinicianId).To(Equal(clinicianB))
	})

	It("forbids deleting another clinician's review", func() {
		// The most recent review belongs to clinician B.
		resp := deleteReview(asUser(clinicianA))
		expectStatus(resp, http.StatusConflict)
	})

	It("deletes only the most recent review of the owning clinician", func() {
		resp := deleteReview(asUser(clinicianB))
		expectStatus(resp, http.StatusOK)
		reviews := decodeAs[[]client.PatientReviewV1](resp)
		Expect(reviews).To(HaveLen(1))
		Expect(reviews[0].ClinicianId).To(Equal(clinicianA))
	})

	It("rejects deletions from clinicians without the most recent review", func() {
		// Clinician B has no remaining reviews; the most recent review
		// belongs to clinician A.
		resp := deleteReview(asUser(clinicianB))
		expectStatus(resp, http.StatusConflict)
	})
})
