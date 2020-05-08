package api_test

import (
	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo"
	"github.com/tidepool-org/clinic/api"
	"net/http"
	"net/http/httptest"

	"github.com/onsi/gomega"
)

func getContext(path string) (echo.Context, *httptest.ResponseRecorder) {
	e := echo.New()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPath(path)
	return c, rec
}

var _ = Describe("Store Test", func() {
	Context("Web Controller", func() {

		Context("Basic Functions", func() {

			mockDB := MockDB{error: ""}
			TestClinicServer := api.ClinicServer{Store:mockDB}

			clinicId := "0001"
			patientId := "0001"
			clinicianId := "0001"

			It("Get Clinic returns ok", func() {
				c, rec := getContext("/clinics")
				clinicParams := api.GetClinicsParams{}

				gomega.Expect(TestClinicServer.GetClinics(c, clinicParams)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Post Clinics returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PostClinics(c)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Delete Clinics returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.DeleteClinicsClinicid(c, clinicId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Get Clinics with id returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.GetClinicsClinicid(c, clinicId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Patch Clinics returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PatchClinicsClinicid(c, clinicId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})

			It("Get Clinicians returns ok", func() {
				c, rec := getContext("/clinics")
				params := api.GetClinicsClinicidCliniciansParams{}
				gomega.Expect(TestClinicServer.GetClinicsClinicidClinicians(c, clinicId, params)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Post clinicians returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PostClinicsClinicidClinicians(c, clinicId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Delete clinicians returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.DeleteClinicsClinicidCliniciansClinicianid(c, clinicId, clinicianId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Get clinicians by id returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.GetClinicsClinicidCliniciansClinicianid(c, clinicId, clinicianId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Patch clinicians returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PatchClinicsClinicidCliniciansClinicianid(c, clinicId, clinicianId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})

			It("Get Patients returns ok", func() {
				c, rec := getContext("/clinics")
				params := api.GetClinicsClinicidPatientsParams{}
				gomega.Expect(TestClinicServer.GetClinicsClinicidPatients(c, clinicId, params)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Post Patients returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PostClinicsClinicidPatients(c, clinicId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Delete Patients returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.DeleteClinicClinicidPatientsPatientid(c, clinicId, patientId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Get Patients by id returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.GetClinicsClinicidPatientsPatientid(c, clinicId, patientId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
			It("Patch Patients returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PatchClinicsClinicidPatientsPatientid(c, clinicId, patientId)).NotTo(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusOK)
			})
		})
		Context("Handle Database Error", func() {
			clinicId := "0001"
			patientId := "0001"
			clinicianId := "0001"
			mockDB := MockDB{error: "An error has occurred"}
			TestClinicServer := api.ClinicServer{Store:mockDB}

			It("Get Clinic returns ok", func() {
				c, rec := getContext("/clinics")
				clinicParams := api.GetClinicsParams{}

				gomega.Expect(TestClinicServer.GetClinics(c, clinicParams)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Post Clinics returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PostClinics(c)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Delete Clinics returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.DeleteClinicsClinicid(c, clinicId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Get Clinics with id returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.GetClinicsClinicid(c, clinicId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Patch Clinics returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PatchClinicsClinicid(c, clinicId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})

			It("Get Clinicians returns ok", func() {
				c, rec := getContext("/clinics")
				params := api.GetClinicsClinicidCliniciansParams{}
				gomega.Expect(TestClinicServer.GetClinicsClinicidClinicians(c, clinicId, params)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Post clinicians returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PostClinicsClinicidClinicians(c, clinicId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Delete clinicians returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.DeleteClinicsClinicidCliniciansClinicianid(c, clinicId, clinicianId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Get clinicians by id returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.GetClinicsClinicidCliniciansClinicianid(c, clinicId, clinicianId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Patch clinicians returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PatchClinicsClinicidCliniciansClinicianid(c, clinicId, clinicianId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})

			It("Get Patients returns ok", func() {
				c, rec := getContext("/clinics")
				params := api.GetClinicsClinicidPatientsParams{}
				gomega.Expect(TestClinicServer.GetClinicsClinicidPatients(c, clinicId, params)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Post Patients returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PostClinicsClinicidPatients(c, clinicId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Delete Patients returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.DeleteClinicClinicidPatientsPatientid(c, clinicId, patientId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Get Patients by id returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.GetClinicsClinicidPatientsPatientid(c, clinicId, patientId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
			It("Patch Patients returns ok", func() {
				c, rec := getContext("/clinics")
				gomega.Expect(TestClinicServer.PatchClinicsClinicidPatientsPatientid(c, clinicId, patientId)).To(gomega.HaveOccurred())
				gomega.Expect(rec.Code, http.StatusInternalServerError)
			})
		})
	})
})
