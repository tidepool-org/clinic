package integration_test

import (
	"fmt"
	"net/http"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/client"
	"github.com/tidepool-org/clinic/store/test"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Pins patient account lifecycle behavior: custodial creation, uniqueness
// constraints (user, email, conditionally unique MRN), updates, clinic
// membership listings, cross-clinic patient search, and the cascading
// cross-clinic deletion and email update endpoints.
var _ = Describe("Patients CRUD", Ordered, func() {
	var adminId string
	var auth func(*http.Request)
	var clinicA, clinicB string

	patientDoc := func(clinicId, userId string) bson.M {
		GinkgoHelper()
		clinicObjId, err := primitive.ObjectIDFromHex(clinicId)
		Expect(err).ToNot(HaveOccurred())
		doc := bson.M{}
		err = test.GetTestDatabase().Collection("patients").
			FindOne(testCtx(), bson.M{"clinicId": clinicObjId, "userId": userId}).Decode(&doc)
		Expect(err).ToNot(HaveOccurred())
		return doc
	}

	BeforeAll(func() {
		admin := newStubUser()
		adminId = admin.UserID
		auth = asUser(adminId)
		clinicA = *createClinic(auth).Id
		clinicB = *createClinic(auth).Id
	})

	Describe("custodial accounts", func() {
		It("creates accounts with custodian permissions and a generated user id", func() {
			patient := createCustodialPatient(clinicA, auth, nil)
			Expect(patient.Id).ToNot(BeNil())
			Expect(patient.Permissions).ToNot(BeNil())
			Expect(patient.Permissions.Custodian).ToNot(BeNil())
			Expect(patient.Permissions.View).ToNot(BeNil())
			Expect(patient.Permissions.Upload).ToNot(BeNil())
			Expect(patient.Permissions.Note).ToNot(BeNil())

			// invitedBy is not exposed via the API but is persisted for
			// clinician-initiated creations. PORT-TO-PG: patients.invited_by
			doc := patientDoc(clinicA, *patient.Id)
			Expect(doc["invitedBy"]).To(Equal(adminId))
		})

		It("does not record invitedBy for server-initiated creations", func() {
			patient := createCustodialPatient(clinicA, asServer, nil)
			// PORT-TO-PG: patients.invited_by
			doc := patientDoc(clinicA, *patient.Id)
			Expect(doc).ToNot(HaveKey("invitedBy"))
		})

		It("lowercases the email provided at creation", func() {
			email := fmt.Sprintf("Patient+%s@Integration.Test", uniqueId())
			patient := createCustodialPatient(clinicA, auth, map[string]interface{}{"email": email})
			Expect(patient.Email).To(PointTo(Equal(strings.ToLower(email))))
		})

		It("rejects custodial accounts with duplicate emails", func() {
			email := fmt.Sprintf("dup+%s@integration.test", uniqueId())
			createCustodialPatient(clinicA, auth, map[string]interface{}{"email": email})

			id := uniqueId()
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients", clinicA),
				jsonBody(map[string]interface{}{
					"fullName": "Patient " + id, "birthDate": "1990-01-01", "email": email,
				}))
			auth(req)
			expectStatus(do(req), http.StatusConflict)
		})
	})

	Describe("patients created from existing users", func() {
		It("lowercases the email of the existing user", func() {
			user := newStubUser()
			user.Username = strings.ToUpper(user.Username)
			user.Emails = []string{user.Username}
			stubUsers.AddUser(user)

			patient := createPatientFromUser(clinicA, user.UserID, asServer, nil)
			Expect(patient.Email).To(PointTo(Equal(strings.ToLower(user.Username))))
		})

		It("rejects adding the same user to the same clinic twice", func() {
			user := newStubUser()
			createPatientFromUser(clinicA, user.UserID, asServer, nil)

			req := prepareRequestWithBody(http.MethodPost,
				fmt.Sprintf("/v1/clinics/%s/patients/%s", clinicA, user.UserID), jsonBody(map[string]interface{}{}))
			asServer(req)
			expectStatus(do(req), http.StatusConflict)
		})
	})

	Describe("MRN uniqueness", func() {
		var clinic string
		var mrn string

		updateMRNSettings := func(unique bool) {
			GinkgoHelper()
			req := prepareRequestWithBody(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/settings/mrn", clinic),
				jsonBody(map[string]interface{}{"required": true, "unique": unique}))
			asServer(req)
			expectStatus(do(req), http.StatusOK)
		}

		createWithMRN := func(mrn string) *http.Response {
			GinkgoHelper()
			id := uniqueId()
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/clinics/%s/patients", clinic),
				jsonBody(map[string]interface{}{
					"fullName": "Patient " + id, "birthDate": "1990-01-01",
					"email": fmt.Sprintf("patient+%s@integration.test", id), "mrn": mrn,
				}))
			auth(req)
			return do(req)
		}

		BeforeAll(func() {
			clinic = *createClinic(auth).Id
			mrn = "MRN" + uniqueId()
		})

		It("rejects duplicate MRNs when the clinic requires unique MRNs", func() {
			updateMRNSettings(true)
			expectStatus(createWithMRN(mrn), http.StatusOK)
			// The service validates MRN uniqueness before hitting the partial
			// unique index, so this surfaces as a 400 rather than a 409.
			expectStatus(createWithMRN(mrn), http.StatusBadRequest)
		})

		It("allows duplicate MRNs when uniqueness is disabled", func() {
			updateMRNSettings(false)
			expectStatus(createWithMRN(mrn), http.StatusOK)
		})
	})

	Describe("updates", func() {
		It("round-trips updated fields", func() {
			patient := createCustodialPatient(clinicA, auth, nil)
			tag := createPatientTag(clinicA, "crud-"+uniqueId(), auth)

			id := uniqueId()
			update := map[string]interface{}{
				"fullName":  "Updated Patient " + id,
				"birthDate": "1980-02-02",
				"mrn":       "M" + id,
				"email":     fmt.Sprintf("updated+%s@integration.test", id),
				"tags":      []string{*tag.Id},
				"glycemicRanges": map[string]interface{}{
					"type": "preset", "preset": "adaHighRisk",
				},
			}
			req := prepareRequestWithBody(http.MethodPut,
				fmt.Sprintf("/v1/clinics/%s/patients/%s", clinicA, *patient.Id), jsonBody(update))
			auth(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)

			fetched := getPatient(clinicA, *patient.Id)
			Expect(fetched.FullName).To(Equal("Updated Patient " + id))
			Expect(fetched.BirthDate.String()).To(Equal("1980-02-02"))
			Expect(fetched.Mrn).To(PointTo(Equal("M" + id)))
			Expect(fetched.Email).To(PointTo(Equal(fmt.Sprintf("updated+%s@integration.test", id))))
			Expect(fetched.Tags).To(PointTo(ConsistOf(*tag.Id)))
			Expect(fetched.GlycemicRanges).ToNot(BeNil())
			Expect(string(fetched.GlycemicRanges.Preset)).To(Equal("adaHighRisk"))
		})
	})

	Describe("clinic memberships of a patient", func() {
		var patientUserId string

		BeforeAll(func() {
			user := newStubUser()
			patientUserId = user.UserID
			createPatientFromUser(clinicA, patientUserId, asServer, nil)
			createPatientFromUser(clinicB, patientUserId, asServer, nil)
		})

		listClinics := func(reqAuth func(*http.Request)) *http.Response {
			GinkgoHelper()
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/patients/%s/clinics", patientUserId), "")
			reqAuth(req)
			return do(req)
		}

		It("returns the relationships to the patient themselves and to servers", func() {
			for _, reqAuth := range []func(*http.Request){asUser(patientUserId), asServer} {
				resp := listClinics(reqAuth)
				expectStatus(resp, http.StatusOK)
				relationships := decodeAs[client.PatientClinicRelationshipsV1](resp)
				Expect(relationships).To(HaveLen(2))
				clinicIds := []string{*relationships[0].Clinic.Id, *relationships[1].Clinic.Id}
				Expect(clinicIds).To(ConsistOf(clinicA, clinicB))
				Expect(relationships[0].Patient.Id).To(PointTo(Equal(patientUserId)))
			}
		})

		It("is forbidden for other users", func() {
			other := newStubUser()
			expectStatus(listClinics(asUser(other.UserID)), http.StatusForbidden)
		})
	})

	Describe("finding patients across clinics", func() {
		var mrn, birthDate string
		var inClinicA client.PatientV1

		BeforeAll(func() {
			mrn = "FIND" + uniqueId()
			birthDate = "1975-03-03"

			inClinicA = createCustodialPatient(clinicA, auth, map[string]interface{}{
				"mrn": mrn, "birthDate": birthDate,
			})

			// The same MRN and birth date in a clinic the caller is not a
			// member of must not be returned.
			otherAdmin := newStubUser()
			otherClinic := createClinic(asUser(otherAdmin.UserID))
			createCustodialPatient(*otherClinic.Id, asUser(otherAdmin.UserID), map[string]interface{}{
				"mrn": mrn, "birthDate": birthDate,
			})
		})

		It("finds patients only in the caller's clinics", func() {
			endpoint := fmt.Sprintf("/v1/patients?mrn=%s&birthDate=%s", mrn, birthDate)
			req := prepareRequest(http.MethodGet, endpoint, "")
			auth(req)
			resp := do(req)
			expectStatus(resp, http.StatusOK)

			relationships := decodeAs[client.PatientClinicRelationshipsV1](resp)
			Expect(relationships).To(HaveLen(1))
			Expect(relationships[0].Clinic.Id).To(PointTo(Equal(clinicA)))
			Expect(relationships[0].Patient.Id).To(PointTo(Equal(*inClinicA.Id)))
		})

		It("rejects server tokens", func() {
			// The authorization policy only allows user tokens, so server
			// tokens are rejected before the handler's own 401 check.
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/patients?mrn=%s", mrn), "")
			asServer(req)
			expectStatus(do(req), http.StatusForbidden)
		})

		It("requires a workspace id type when a workspace id is given", func() {
			req := prepareRequest(http.MethodGet, fmt.Sprintf("/v1/patients?mrn=%s&workspaceId=abc", mrn), "")
			auth(req)
			expectStatus(do(req), http.StatusBadRequest)
		})
	})

	Describe("deleting a user from all clinics", func() {
		var userId string

		BeforeAll(func() {
			user := newStubUser()
			userId = user.UserID
			createPatientFromUser(clinicA, userId, asServer, nil)
			createClinicianDirect(clinicB, userId)
		})

		It("is forbidden for regular users", func() {
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/users/%s/clinics", userId), "")
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})

		It("removes patient and clinician memberships", func() {
			req := prepareRequest(http.MethodDelete, fmt.Sprintf("/v1/users/%s/clinics", userId), "")
			asServer(req)
			expectStatus(do(req), http.StatusOK)

			req = prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/patients/%s", clinicA, userId), "")
			asServer(req)
			expectStatus(do(req), http.StatusNotFound)

			req = prepareRequest(http.MethodGet, fmt.Sprintf("/v1/clinics/%s/clinicians/%s", clinicB, userId), "")
			asServer(req)
			expectStatus(do(req), http.StatusNotFound)
		})

		It("creates deletion audit records", func() {
			// PORT-TO-PG: deletion audit records have no read endpoint.
			db := test.GetTestDatabase()

			err := db.Collection("patient_deletions").
				FindOne(testCtx(), bson.M{"patient.userId": userId}).Err()
			Expect(err).ToNot(HaveOccurred())

			err = db.Collection("clinician_deletions").
				FindOne(testCtx(), bson.M{"clinician.userId": userId}).Err()
			Expect(err).ToNot(HaveOccurred())
		})
	})

	Describe("updating user details in all clinics", func() {
		var userId string

		BeforeAll(func() {
			user := newStubUser()
			userId = user.UserID
			createPatientFromUser(clinicA, userId, asServer, nil)
			createClinicianDirect(clinicB, userId)
		})

		It("updates the email on patient and clinician records", func() {
			email := fmt.Sprintf("renamed+%s@integration.test", uniqueId())
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/users/%s/clinics", userId),
				jsonBody(map[string]interface{}{"email": email}))
			asServer(req)
			expectStatus(do(req), http.StatusOK)

			patient := getPatient(clinicA, userId)
			Expect(patient.Email).To(PointTo(Equal(email)))

			clinician := getClinician(clinicB, userId)
			Expect(clinician.Email).To(Equal(email))
		})

		It("is forbidden for regular users", func() {
			req := prepareRequestWithBody(http.MethodPost, fmt.Sprintf("/v1/users/%s/clinics", userId),
				jsonBody(map[string]interface{}{"email": "nope@integration.test"}))
			auth(req)
			expectStatus(do(req), http.StatusForbidden)
		})
	})
})
