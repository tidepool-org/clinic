package integration_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"time"

	"github.com/TwiN/deepmerge"
	"github.com/labstack/echo/v4"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	. "github.com/onsi/gomega/gstruct"
	"github.com/tidepool-org/clinic/client"
	integrationTest "github.com/tidepool-org/clinic/integration/test"
	"github.com/tidepool-org/clinic/test"
	xealthTest "github.com/tidepool-org/clinic/xealth/test"
	"github.com/tidepool-org/clinic/xealth_client"
)

const (
	XealthBearerToken           = "xealth-token"
	XealthAdultOrderId          = "7e316617-ef33-4859-b0c9-36bddbfe9229"
	XealthPediatricOrderId      = "4f5d4267-654e-451f-acde-d56560940989"
	XealtAdultOrderFixture      = "./test/xealth_fixtures/05_read_order_response.json"
	XealthPediatricOrderFixture = "./test/xealth_fixtures/13_read_order_response_pediatric.json"
)

var _ = Describe("Xealth Integration Test", Ordered, func() {
	var clinic client.Clinic
	var patient client.Patient
	var dataTrackingId string

	Describe("Create a clinic", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/clinics", "./test/xealth_fixtures/01_create_clinic.json")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(json.Unmarshal(body, &clinic)).To(Succeed())
			Expect(clinic.Id).ToNot(BeNil())
		})
	})

	Describe("Enable Xealth integration", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, fmt.Sprintf("/v1/clinics/%s/settings/ehr", *clinic.Id), "./test/xealth_fixtures/02_enable_xealth.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Send initial pre-order request", func() {
		It("Succeeds", func() {
			dataTrackingId = sendAdultInitialPreorder(server)
		})
	})

	Describe("Send subsequent pre-order request", func() {
		It("Succeeds", func() {
			bodyReader := sendAdultSubsequentPreorder(dataTrackingId, server)
			body, err := io.ReadAll(bodyReader)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(body)).To(Equal("{}\n"))
		})
	})

	Describe("Send order notification event", func() {
		BeforeEach(func() {
			prepareOrder(dataTrackingId, XealthAdultOrderId, XealtAdultOrderFixture, xealthStub)
		})

		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/xealth/notification", "./test/xealth_fixtures/05_order_notification_event.json")
			asXealth(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Get Patient by MRN", func() {
		It("Returns the patient", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%v/patients?search=%s", *clinic.Id, "e987655")
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())

			response := client.PatientsResponse{}
			Expect(json.Unmarshal(body, &response)).To(Succeed())
			Expect(response.Data).ToNot(BeNil())
			Expect(response.Meta).ToNot(BeNil())
			Expect(response.Meta.Count).To(PointTo(Equal(1)))
			Expect(response.Data).To(PointTo(HaveLen(1)))

			patient = (*response.Data)[0]
			Expect(patient.Id).ToNot(BeNil())
			Expect(patient.Mrn).To(PointTo(Equal("e987655")))
		})
	})

	Describe("Patient", func() {
		It("name is correct", func() {
			Expect(patient.FullName).To(Equal("FirstName First LastName Last"))
		})

		It("email is correct", func() {
			Expect(patient.Email).To(PointTo(Equal("xealth@tidepool.org")))
		})

		It("last request dexcom connect time is set", func() {
			Expect(patient.LastRequestedDexcomConnectTime).To(PointTo(BeTemporally("~", time.Now().UTC(), time.Second*5)))
		})
	})

	Describe("Send get programs request", func() {
		It("Succeeds", func() {
			response := getPrograms(server)
			Expect(response.Present).To(BeTrue())
			Expect(response.Programs).To(HaveLen(1))

			program := response.Programs[0]
			Expect(program.Description).To(PointTo(Equal("Last Upload: N/A | Last Viewed by You: N/A")))
			Expect(program.EnrolledDate).To(PointTo(Equal("2021-01-14")))
			Expect(program.HasStatusView).To(PointTo(BeFalse()))
			Expect(program.HasAlert).To(PointTo(BeFalse()))
			Expect(program.ProgramId).To(PointTo(Equal("100")))
			Expect(program.Status).To(BeNil())
			Expect(program.Title).To(PointTo(Equal("Tidepool")))
		})
	})

	Describe("Update Summary", func() {
		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/patients/%s/summary", *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, endpoint, "./test/xealth_fixtures/07_update_summary.json")
			asServer(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			res, _ := io.ReadAll(rec.Result().Body)
			fmt.Println(string(res))
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})
	})

	Describe("Send get programs request after data upload", func() {
		It("Succeeds", func() {
			response := getPrograms(server)
			Expect(response.Present).To(BeTrue())
			Expect(response.Programs).To(HaveLen(1))

			program := response.Programs[0]

			// Last upload should be set to the summary last updated date
			Expect(program.Description).To(PointTo(Equal("Last Upload: 2024-01-18 | Last Viewed by You: N/A")))
			Expect(program.EnrolledDate).To(PointTo(Equal("2021-01-14")))
			Expect(program.HasStatusView).To(PointTo(BeTrue()))
			Expect(program.HasAlert).To(PointTo(BeTrue()))
			Expect(program.ProgramId).To(PointTo(Equal("100")))
			Expect(program.Status).To(BeNil())
			Expect(program.Title).To(PointTo(Equal("Tidepool")))
		})
	})

	Describe("Send get program url request", func() {
		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPut, "/v1/xealth/program", "./test/xealth_fixtures/08_get_program_url.json")
			asXealth(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())

			response := xealth_client.GetProgramUrlResponse{}
			Expect(json.Unmarshal(body, &response)).To(Succeed())
			Expect(response.Url).ToNot(BeEmpty())

			reportUrl, err := url.Parse(response.Url)
			Expect(err).ToNot(HaveOccurred())
			Expect(reportUrl.Scheme).To(Equal("https"))
			Expect(reportUrl.Host).To(Equal("integration.test.app.url.com"))
			Expect(reportUrl.Path).To(Equal("/v1/xealth/report/web/viewer.html"))
			Expect(reportUrl.Query().Get("clinicId")).To(Equal(*clinic.Id))
			Expect(reportUrl.Query().Get("patientId")).To(Equal(*patient.Id))
			Expect(reportUrl.Query().Get("restricted_token")).To(Equal(integrationTest.TestRestrictedToken))
		})
	})

	Describe("View PDF report", func() {
		It("Succeeds", func() {
			reportUrl := fmt.Sprintf("/v1/xealth/report/web/viewer.html?clinicId=%s&patientId=%s&restricted_token=%s", *clinic.Id, *patient.Id, integrationTest.TestRestrictedToken)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodGet, reportUrl, "")
			asXealth(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())

			Expect(string(body)).To(ContainSubstring("https:\\/\\/integration.test.app.url.com\\/export\\/report\\/1234567891?bgUnits=mg%2FdL\\u0026dob=1985-01-11\\u0026endDate=2024-01-17T06%3A38%3A22Z\\u0026fullName=FirstName\\u002bFirst\\u002bLastName\\u002bLast\\u0026inline=true\\u0026mrn=e987655\\u0026reports=all\\u0026restricted_token=1234567890abcdef1234567890abcdef\\u0026startDate=2024-01-03T06%3A38%3A22Z\\u0026tzName=US%2FPacific\\u0026userId=1234567891"))
		})
	})

	Describe("Send get programs request after view", func() {
		It("Succeeds", func() {
			response := getPrograms(server)
			Expect(response.Present).To(BeTrue())
			Expect(response.Programs).To(HaveLen(1))

			program := response.Programs[0]
			today := time.Now().UTC().Format(time.DateOnly)

			// Last upload should be set to the summary last updated date
			Expect(program.Description).To(PointTo(Equal(fmt.Sprintf("Last Upload: 2024-01-18 | Last Viewed by You: %s", today))))
			Expect(program.EnrolledDate).To(PointTo(Equal("2021-01-14")))
			Expect(program.HasStatusView).To(PointTo(BeTrue()))
			Expect(program.HasAlert).To(PointTo(BeFalse()))
			Expect(program.ProgramId).To(PointTo(Equal("100")))
			Expect(program.Status).To(BeNil())
			Expect(program.Title).To(PointTo(Equal("Tidepool")))
		})
	})

	Describe("Send cancel order notification event", func() {
		BeforeEach(func() {
			body, err := test.LoadFixture("./test/xealth_fixtures/05_read_order_response.json")
			Expect(err).ToNot(HaveOccurred())

			overrides, err := json.Marshal(map[string]interface{}{
				"orderInfo": map[string]interface{}{
					"orderState": "canceled",
				},
				"preorder": map[string]interface{}{
					"dataTrackingId": dataTrackingId,
				},
			})
			Expect(err).ToNot(HaveOccurred())
			body, err = deepmerge.JSON(body, overrides, deepmerge.Config{
				PreventMultipleDefinitionsOfKeysWithPrimitiveValue: false},
			)

			xealthStub.AddOrder("artificialhealthcare", "7e316617-ef33-4859-b0c9-36bddbfe9229", body)
		})

		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/xealth/notification", "./test/xealth_fixtures/09_cancel_order_notification_event.json")
			asXealth(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Removes the subscription", func() {
			response := getPrograms(server)
			Expect(response.Present).To(BeFalse())
			Expect(response.Programs).To(BeEmpty())
		})
	})

	Describe("Send initial pre-order request for a matching patient", func() {
		It("Succeeds with a final response", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/xealth/preorder", "./test/xealth_fixtures/03_initial_pre_order.json")
			asXealth(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(body)).To(Equal("{}\n"))
		})
	})

	Describe("Send initial pre-order request with a birthday mismatch", func() {
		It("Succeeds with a final response", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/xealth/preorder", "./test/xealth_fixtures/03_initial_pre_order_birthday_mismatch.json")
			asXealth(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

			body, err := io.ReadAll(rec.Result().Body)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(body)).To(ContainSubstring("there's a mismatch with the patient's date of birth between Tidepool and the Electronic Health Record (EHR)"))
		})
	})

	Describe("Send order notification event after subscription removal", func() {
		BeforeEach(func() {
			prepareOrder(dataTrackingId, XealthAdultOrderId, XealtAdultOrderFixture, xealthStub)
		})

		It("Succeeds", func() {
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodPost, "/v1/xealth/notification", "./test/xealth_fixtures/05_order_notification_event.json")
			asXealth(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
		})

		It("Recreates the subscription", func() {
			response := getPrograms(server)
			Expect(response.Present).To(BeTrue())
			Expect(response.Programs).ToNot(BeEmpty())
		})
	})

	Describe("Delete patient", func() {
		BeforeEach(func() {
			prepareOrder(dataTrackingId, XealthAdultOrderId, XealtAdultOrderFixture, xealthStub)
		})

		It("Succeeds", func() {
			endpoint := fmt.Sprintf("/v1/clinics/%s/patients/%s", *clinic.Id, *patient.Id)
			rec := httptest.NewRecorder()
			req := prepareRequest(http.MethodDelete, endpoint, "")
			asClinician(req)

			server.ServeHTTP(rec, req)
			Expect(rec.Result()).ToNot(BeNil())
			Expect(rec.Result().StatusCode).To(Equal(http.StatusNoContent))
		})
	})

	Describe("Send initial pre-order after patient deletion", func() {
		It("Succeeds", func() {
			dataTrackingId = sendAdultInitialPreorder(server)
		})
	})

	Describe("Send subsequent pre-order after patient deletion", func() {
		It("Fails due to duplicate email", func() {
			bodyReader := sendAdultSubsequentPreorder(dataTrackingId, server)
			body, err := io.ReadAll(bodyReader)
			Expect(err).ToNot(HaveOccurred())
			Expect(string(body)).To(ContainSubstring("The email address you chose is already in use with another account in Tidepool"))
		})
	})

	Context("Guardian Flow", func() {
		Describe("Send pediatric initial pre-order request", func() {
			It("Succeeds", func() {
				dataTrackingId = sendPediatricInitialPreorder(server)
			})
		})

		Describe("Send pediatric subsequent pre-order request", func() {
			It("Succeeds", func() {
				bodyReader := sendPediatricSubsequentPreorder(dataTrackingId, server)
				body, err := io.ReadAll(bodyReader)
				Expect(err).ToNot(HaveOccurred())
				Expect(string(body)).To(Equal("{}\n"))
			})
		})

		Describe("Send pediatric order notification event", func() {
			BeforeEach(func() {
				prepareOrder(dataTrackingId, XealthPediatricOrderId, XealthPediatricOrderFixture, xealthStub)
			})

			It("Succeeds", func() {
				rec := httptest.NewRecorder()
				req := prepareRequest(http.MethodPost, "/v1/xealth/notification", "./test/xealth_fixtures/12_order_notification_event_pediatric.json")
				asXealth(req)

				server.ServeHTTP(rec, req)
				Expect(rec.Result()).ToNot(BeNil())
				Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))
			})
		})

		Describe("Get Patient by MRN", func() {
			It("Returns the patient", func() {
				endpoint := fmt.Sprintf("/v1/clinics/%v/patients?search=%s", *clinic.Id, "e987656")
				rec := httptest.NewRecorder()
				req := prepareRequest(http.MethodGet, endpoint, "")
				asClinician(req)

				server.ServeHTTP(rec, req)
				Expect(rec.Result()).ToNot(BeNil())
				Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

				body, err := io.ReadAll(rec.Result().Body)
				Expect(err).ToNot(HaveOccurred())

				response := client.PatientsResponse{}
				Expect(json.Unmarshal(body, &response)).To(Succeed())
				Expect(response.Data).ToNot(BeNil())
				Expect(response.Meta).ToNot(BeNil())
				Expect(response.Meta.Count).To(PointTo(Equal(1)))
				Expect(response.Data).To(PointTo(HaveLen(1)))

				patient = (*response.Data)[0]
				Expect(patient.Id).ToNot(BeNil())
				Expect(patient.Mrn).To(PointTo(Equal("e987656")))
			})
		})

		Describe("Patient", func() {
			It("name is correct", func() {
				Expect(patient.FullName).To(Equal("FirstName First LastName Last"))
			})

			It("email is correct", func() {
				Expect(patient.Email).To(PointTo(Equal("xealth+guardian@tidepool.org")))
			})
		})
	})
})

func sendAdultInitialPreorder(server *echo.Echo) string {
	rec := httptest.NewRecorder()
	req := prepareRequest(http.MethodPost, "/v1/xealth/preorder", "./test/xealth_fixtures/03_initial_pre_order.json")
	asXealth(req)

	server.ServeHTTP(rec, req)
	Expect(rec.Result()).ToNot(BeNil())
	Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

	body, err := io.ReadAll(rec.Result().Body)
	Expect(err).ToNot(HaveOccurred())

	response := xealth_client.PreorderFormResponse0{}
	Expect(json.Unmarshal(body, &response)).To(Succeed())

	Expect(response.DataTrackingId).ToNot(BeEmpty())
	Expect(response.NotOrderable).To(PointTo(BeFalse()))
	Expect(response.PreorderFormInfo.FormId).To(PointTo(Equal("patient_enrollment_form")))

	return response.DataTrackingId
}

func sendPediatricInitialPreorder(server *echo.Echo) string {
	rec := httptest.NewRecorder()
	req := prepareRequest(http.MethodPost, "/v1/xealth/preorder", "./test/xealth_fixtures/10_initial_pre_order_pediatric.json")
	asXealth(req)

	server.ServeHTTP(rec, req)
	Expect(rec.Result()).ToNot(BeNil())
	Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

	body, err := io.ReadAll(rec.Result().Body)
	Expect(err).ToNot(HaveOccurred())

	response := xealth_client.PreorderFormResponse0{}
	Expect(json.Unmarshal(body, &response)).To(Succeed())

	Expect(response.DataTrackingId).ToNot(BeEmpty())
	Expect(response.NotOrderable).To(PointTo(BeFalse()))
	Expect(response.PreorderFormInfo.FormId).To(PointTo(Equal("guardian_enrollment_form")))

	return response.DataTrackingId
}

func sendAdultSubsequentPreorder(dataTrackingId string, server *echo.Echo) io.ReadCloser {
	// Set the data tracking id from the previous step when sending the subsequent preorer requrest
	overrides, err := json.Marshal(map[string]interface{}{
		"formData": map[string]interface{}{
			"dataTrackingId": dataTrackingId,
		},
	})
	Expect(err).ToNot(HaveOccurred())

	body, err := test.LoadFixture("./test/xealth_fixtures/04_subsequent_pre_order.json")
	Expect(err).ToNot(HaveOccurred())

	body, err = deepmerge.JSON(body, overrides, deepmerge.Config{
		PreventMultipleDefinitionsOfKeysWithPrimitiveValue: false},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/xealth/preorder", bytes.NewReader(body))
	asXealth(req)

	server.ServeHTTP(rec, req)
	Expect(rec.Result()).ToNot(BeNil())
	Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

	return rec.Result().Body
}

func sendPediatricSubsequentPreorder(dataTrackingId string, server *echo.Echo) io.ReadCloser {
	// Set the data tracking id from the previous step when sending the subsequent preorer requrest
	overrides, err := json.Marshal(map[string]interface{}{
		"formData": map[string]interface{}{
			"dataTrackingId": dataTrackingId,
		},
	})
	Expect(err).ToNot(HaveOccurred())

	body, err := test.LoadFixture("./test/xealth_fixtures/11_subsequent_pre_order_pediatric.json")
	Expect(err).ToNot(HaveOccurred())

	body, err = deepmerge.JSON(body, overrides, deepmerge.Config{
		PreventMultipleDefinitionsOfKeysWithPrimitiveValue: false},
	)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/v1/xealth/preorder", bytes.NewReader(body))
	asXealth(req)

	server.ServeHTTP(rec, req)
	Expect(rec.Result()).ToNot(BeNil())
	Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

	return rec.Result().Body
}

func prepareOrder(dataTrackingId, orderId, fixturePath string, xealthStub *xealthTest.XealthServer) {
	// Set the data tracking id from the previous step when sending the subsequent preorer requrest
	overrides, err := json.Marshal(map[string]interface{}{
		"preorder": map[string]interface{}{
			"dataTrackingId": dataTrackingId,
		},
	})
	Expect(err).ToNot(HaveOccurred())

	body, err := test.LoadFixture(fixturePath)
	Expect(err).ToNot(HaveOccurred())

	body, err = deepmerge.JSON(body, overrides, deepmerge.Config{
		PreventMultipleDefinitionsOfKeysWithPrimitiveValue: false},
	)

	xealthStub.AddOrder("artificialhealthcare", orderId, body)
}

func getPrograms(server *echo.Echo) xealth_client.GetProgramsResponse0 {
	rec := httptest.NewRecorder()
	req := prepareRequest(http.MethodPut, "/v1/xealth/programs", "./test/xealth_fixtures/06_get_programs.json")
	asXealth(req)

	server.ServeHTTP(rec, req)
	Expect(rec.Result()).ToNot(BeNil())
	Expect(rec.Result().StatusCode).To(Equal(http.StatusOK))

	body, err := io.ReadAll(rec.Result().Body)
	Expect(err).ToNot(HaveOccurred())

	response := xealth_client.GetProgramsResponse0{}
	Expect(json.Unmarshal(body, &response)).To(Succeed())

	return response
}

func prepareRequest(method, endpoint string, fixturePath string) *http.Request {
	var body io.Reader
	if fixturePath != "" {
		b, err := test.LoadFixture(fixturePath)
		Expect(err).ToNot(HaveOccurred())
		body = bytes.NewReader(b)
	}

	req := httptest.NewRequest(method, endpoint, body)
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	return req
}

func asClinician(req *http.Request) {
	req.Header.Set("x-tidepool-session-token", integrationTest.TestUserToken)
}

func asServer(req *http.Request) {
	req.Header.Set("x-tidepool-session-token", integrationTest.TestServerToken)
}

func asServiceAccount(req *http.Request) {
	req.Header.Set("x-tidepool-session-token", integrationTest.TestServiceAccountToken)
}

func asXealth(req *http.Request) {
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", XealthBearerToken))
}
