// Code generated by MockGen. DO NOT EDIT.
// Source: ./patients.go

// Package test is a generated GoMock package.
package test

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	patients "github.com/tidepool-org/clinic/patients"
	store "github.com/tidepool-org/clinic/store"
)

// MockService is a mock of Service interface.
type MockService struct {
	ctrl     *gomock.Controller
	recorder *MockServiceMockRecorder
}

// MockServiceMockRecorder is the mock recorder for MockService.
type MockServiceMockRecorder struct {
	mock *MockService
}

// NewMockService creates a new mock instance.
func NewMockService(ctrl *gomock.Controller) *MockService {
	mock := &MockService{ctrl: ctrl}
	mock.recorder = &MockServiceMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockService) EXPECT() *MockServiceMockRecorder {
	return m.recorder
}

// AssignPatientTagToClinicPatients mocks base method.
func (m *MockService) AssignPatientTagToClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "AssignPatientTagToClinicPatients", ctx, clinicId, tagId, patientIds)
	ret0, _ := ret[0].(error)
	return ret0
}

// AssignPatientTagToClinicPatients indicates an expected call of AssignPatientTagToClinicPatients.
func (mr *MockServiceMockRecorder) AssignPatientTagToClinicPatients(ctx, clinicId, tagId, patientIds interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "AssignPatientTagToClinicPatients", reflect.TypeOf((*MockService)(nil).AssignPatientTagToClinicPatients), ctx, clinicId, tagId, patientIds)
}

// Create mocks base method.
func (m *MockService) Create(ctx context.Context, patient patients.Patient) (*patients.Patient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", ctx, patient)
	ret0, _ := ret[0].(*patients.Patient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockServiceMockRecorder) Create(ctx, patient interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockService)(nil).Create), ctx, patient)
}

// DeleteFromAllClinics mocks base method.
func (m *MockService) DeleteFromAllClinics(ctx context.Context, userId string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteFromAllClinics", ctx, userId)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteFromAllClinics indicates an expected call of DeleteFromAllClinics.
func (mr *MockServiceMockRecorder) DeleteFromAllClinics(ctx, userId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteFromAllClinics", reflect.TypeOf((*MockService)(nil).DeleteFromAllClinics), ctx, userId)
}

// DeleteNonCustodialPatientsOfClinic mocks base method.
func (m *MockService) DeleteNonCustodialPatientsOfClinic(ctx context.Context, clinicId string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeleteNonCustodialPatientsOfClinic", ctx, clinicId)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeleteNonCustodialPatientsOfClinic indicates an expected call of DeleteNonCustodialPatientsOfClinic.
func (mr *MockServiceMockRecorder) DeleteNonCustodialPatientsOfClinic(ctx, clinicId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeleteNonCustodialPatientsOfClinic", reflect.TypeOf((*MockService)(nil).DeleteNonCustodialPatientsOfClinic), ctx, clinicId)
}

// DeletePatientTagFromClinicPatients mocks base method.
func (m *MockService) DeletePatientTagFromClinicPatients(ctx context.Context, clinicId, tagId string, patientIds []string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeletePatientTagFromClinicPatients", ctx, clinicId, tagId, patientIds)
	ret0, _ := ret[0].(error)
	return ret0
}

// DeletePatientTagFromClinicPatients indicates an expected call of DeletePatientTagFromClinicPatients.
func (mr *MockServiceMockRecorder) DeletePatientTagFromClinicPatients(ctx, clinicId, tagId, patientIds interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeletePatientTagFromClinicPatients", reflect.TypeOf((*MockService)(nil).DeletePatientTagFromClinicPatients), ctx, clinicId, tagId, patientIds)
}

// DeletePermission mocks base method.
func (m *MockService) DeletePermission(ctx context.Context, clinicId, userId, permission string) (*patients.Patient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeletePermission", ctx, clinicId, userId, permission)
	ret0, _ := ret[0].(*patients.Patient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeletePermission indicates an expected call of DeletePermission.
func (mr *MockServiceMockRecorder) DeletePermission(ctx, clinicId, userId, permission interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeletePermission", reflect.TypeOf((*MockService)(nil).DeletePermission), ctx, clinicId, userId, permission)
}

// Get mocks base method.
func (m *MockService) Get(ctx context.Context, clinicId, userId string) (*patients.Patient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", ctx, clinicId, userId)
	ret0, _ := ret[0].(*patients.Patient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockServiceMockRecorder) Get(ctx, clinicId, userId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockService)(nil).Get), ctx, clinicId, userId)
}

// List mocks base method.
func (m *MockService) List(ctx context.Context, filter *patients.Filter, pagination store.Pagination, sort []*store.Sort) (*patients.ListResult, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", ctx, filter, pagination, sort)
	ret0, _ := ret[0].(*patients.ListResult)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockServiceMockRecorder) List(ctx, filter, pagination, sort interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockService)(nil).List), ctx, filter, pagination, sort)
}

// Remove mocks base method.
func (m *MockService) Remove(ctx context.Context, clinicId, userId string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Remove", ctx, clinicId, userId)
	ret0, _ := ret[0].(error)
	return ret0
}

// Remove indicates an expected call of Remove.
func (mr *MockServiceMockRecorder) Remove(ctx, clinicId, userId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Remove", reflect.TypeOf((*MockService)(nil).Remove), ctx, clinicId, userId)
}

// RescheduleLastSubscriptionOrderForAllPatients mocks base method.
func (m *MockService) RescheduleLastSubscriptionOrderForAllPatients(ctx context.Context, clinicId, subscription, ordersCollection, targetCollection string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RescheduleLastSubscriptionOrderForAllPatients", ctx, clinicId, subscription, ordersCollection, targetCollection)
	ret0, _ := ret[0].(error)
	return ret0
}

// RescheduleLastSubscriptionOrderForAllPatients indicates an expected call of RescheduleLastSubscriptionOrderForAllPatients.
func (mr *MockServiceMockRecorder) RescheduleLastSubscriptionOrderForAllPatients(ctx, clinicId, subscription, ordersCollection, targetCollection interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RescheduleLastSubscriptionOrderForAllPatients", reflect.TypeOf((*MockService)(nil).RescheduleLastSubscriptionOrderForAllPatients), ctx, clinicId, subscription, ordersCollection, targetCollection)
}

// Update mocks base method.
func (m *MockService) Update(ctx context.Context, update patients.PatientUpdate) (*patients.Patient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", ctx, update)
	ret0, _ := ret[0].(*patients.Patient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockServiceMockRecorder) Update(ctx, update interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockService)(nil).Update), ctx, update)
}

// UpdateEHRSubscription mocks base method.
func (m *MockService) UpdateEHRSubscription(ctx context.Context, clinicId, userId string, update patients.SubscriptionUpdate) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateEHRSubscription", ctx, clinicId, userId, update)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateEHRSubscription indicates an expected call of UpdateEHRSubscription.
func (mr *MockServiceMockRecorder) UpdateEHRSubscription(ctx, clinicId, userId, update interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateEHRSubscription", reflect.TypeOf((*MockService)(nil).UpdateEHRSubscription), ctx, clinicId, userId, update)
}

// UpdateEmail mocks base method.
func (m *MockService) UpdateEmail(ctx context.Context, userId string, email *string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateEmail", ctx, userId, email)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateEmail indicates an expected call of UpdateEmail.
func (mr *MockServiceMockRecorder) UpdateEmail(ctx, userId, email interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateEmail", reflect.TypeOf((*MockService)(nil).UpdateEmail), ctx, userId, email)
}

// UpdateLastRequestedDexcomConnectTime mocks base method.
func (m *MockService) UpdateLastRequestedDexcomConnectTime(ctx context.Context, update *patients.LastRequestedDexcomConnectUpdate) (*patients.Patient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateLastRequestedDexcomConnectTime", ctx, update)
	ret0, _ := ret[0].(*patients.Patient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateLastRequestedDexcomConnectTime indicates an expected call of UpdateLastRequestedDexcomConnectTime.
func (mr *MockServiceMockRecorder) UpdateLastRequestedDexcomConnectTime(ctx, update interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateLastRequestedDexcomConnectTime", reflect.TypeOf((*MockService)(nil).UpdateLastRequestedDexcomConnectTime), ctx, update)
}

// UpdateLastUploadReminderTime mocks base method.
func (m *MockService) UpdateLastUploadReminderTime(ctx context.Context, update *patients.UploadReminderUpdate) (*patients.Patient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateLastUploadReminderTime", ctx, update)
	ret0, _ := ret[0].(*patients.Patient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdateLastUploadReminderTime indicates an expected call of UpdateLastUploadReminderTime.
func (mr *MockServiceMockRecorder) UpdateLastUploadReminderTime(ctx, update interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateLastUploadReminderTime", reflect.TypeOf((*MockService)(nil).UpdateLastUploadReminderTime), ctx, update)
}

// UpdatePatientDataSources mocks base method.
func (m *MockService) UpdatePatientDataSources(ctx context.Context, userId string, dataSources *patients.DataSources) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdatePatientDataSources", ctx, userId, dataSources)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdatePatientDataSources indicates an expected call of UpdatePatientDataSources.
func (mr *MockServiceMockRecorder) UpdatePatientDataSources(ctx, userId, dataSources interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdatePatientDataSources", reflect.TypeOf((*MockService)(nil).UpdatePatientDataSources), ctx, userId, dataSources)
}

// UpdatePermissions mocks base method.
func (m *MockService) UpdatePermissions(ctx context.Context, clinicId, userId string, permissions *patients.Permissions) (*patients.Patient, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdatePermissions", ctx, clinicId, userId, permissions)
	ret0, _ := ret[0].(*patients.Patient)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdatePermissions indicates an expected call of UpdatePermissions.
func (mr *MockServiceMockRecorder) UpdatePermissions(ctx, clinicId, userId, permissions interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdatePermissions", reflect.TypeOf((*MockService)(nil).UpdatePermissions), ctx, clinicId, userId, permissions)
}

// UpdateSummaryInAllClinics mocks base method.
func (m *MockService) UpdateSummaryInAllClinics(ctx context.Context, userId string, summary *patients.Summary) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateSummaryInAllClinics", ctx, userId, summary)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateSummaryInAllClinics indicates an expected call of UpdateSummaryInAllClinics.
func (mr *MockServiceMockRecorder) UpdateSummaryInAllClinics(ctx, userId, summary interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateSummaryInAllClinics", reflect.TypeOf((*MockService)(nil).UpdateSummaryInAllClinics), ctx, userId, summary)
}