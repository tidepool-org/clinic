// Code generated by MockGen. DO NOT EDIT.
// Source: ./clinics.go

// Package test is a generated GoMock package.
package test

import (
	context "context"
	reflect "reflect"

	gomock "github.com/golang/mock/gomock"
	clinics "github.com/tidepool-org/clinic/clinics"
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

// Create mocks base method.
func (m *MockService) Create(ctx context.Context, clinic *clinics.Clinic) (*clinics.Clinic, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Create", ctx, clinic)
	ret0, _ := ret[0].(*clinics.Clinic)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Create indicates an expected call of Create.
func (mr *MockServiceMockRecorder) Create(ctx, clinic interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Create", reflect.TypeOf((*MockService)(nil).Create), ctx, clinic)
}

// CreatePatientTag mocks base method.
func (m *MockService) CreatePatientTag(ctx context.Context, clinicId, tagName string) (*clinics.Clinic, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "CreatePatientTag", ctx, clinicId, tagName)
	ret0, _ := ret[0].(*clinics.Clinic)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// CreatePatientTag indicates an expected call of CreatePatientTag.
func (mr *MockServiceMockRecorder) CreatePatientTag(ctx, clinicId, tagName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "CreatePatientTag", reflect.TypeOf((*MockService)(nil).CreatePatientTag), ctx, clinicId, tagName)
}

// Delete mocks base method.
func (m *MockService) Delete(ctx context.Context, id string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Delete", ctx, id)
	ret0, _ := ret[0].(error)
	return ret0
}

// Delete indicates an expected call of Delete.
func (mr *MockServiceMockRecorder) Delete(ctx, id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Delete", reflect.TypeOf((*MockService)(nil).Delete), ctx, id)
}

// DeletePatientTag mocks base method.
func (m *MockService) DeletePatientTag(ctx context.Context, clinicId, tagId string) (*clinics.Clinic, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "DeletePatientTag", ctx, clinicId, tagId)
	ret0, _ := ret[0].(*clinics.Clinic)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// DeletePatientTag indicates an expected call of DeletePatientTag.
func (mr *MockServiceMockRecorder) DeletePatientTag(ctx, clinicId, tagId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "DeletePatientTag", reflect.TypeOf((*MockService)(nil).DeletePatientTag), ctx, clinicId, tagId)
}

// Get mocks base method.
func (m *MockService) Get(ctx context.Context, id string) (*clinics.Clinic, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Get", ctx, id)
	ret0, _ := ret[0].(*clinics.Clinic)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Get indicates an expected call of Get.
func (mr *MockServiceMockRecorder) Get(ctx, id interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Get", reflect.TypeOf((*MockService)(nil).Get), ctx, id)
}

// GetEHRSettings mocks base method.
func (m *MockService) GetEHRSettings(ctx context.Context, clinicId string) (*clinics.EHRSettings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetEHRSettings", ctx, clinicId)
	ret0, _ := ret[0].(*clinics.EHRSettings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetEHRSettings indicates an expected call of GetEHRSettings.
func (mr *MockServiceMockRecorder) GetEHRSettings(ctx, clinicId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetEHRSettings", reflect.TypeOf((*MockService)(nil).GetEHRSettings), ctx, clinicId)
}

// GetMRNSettings mocks base method.
func (m *MockService) GetMRNSettings(ctx context.Context, clinicId string) (*clinics.MRNSettings, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "GetMRNSettings", ctx, clinicId)
	ret0, _ := ret[0].(*clinics.MRNSettings)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// GetMRNSettings indicates an expected call of GetMRNSettings.
func (mr *MockServiceMockRecorder) GetMRNSettings(ctx, clinicId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "GetMRNSettings", reflect.TypeOf((*MockService)(nil).GetMRNSettings), ctx, clinicId)
}

// List mocks base method.
func (m *MockService) List(ctx context.Context, filter *clinics.Filter, pagination store.Pagination) ([]*clinics.Clinic, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", ctx, filter, pagination)
	ret0, _ := ret[0].([]*clinics.Clinic)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockServiceMockRecorder) List(ctx, filter, pagination interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockService)(nil).List), ctx, filter, pagination)
}

// ListMembershipRestrictions mocks base method.
func (m *MockService) ListMembershipRestrictions(ctx context.Context, clinicId string) ([]clinics.MembershipRestrictions, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "ListMembershipRestrictions", ctx, clinicId)
	ret0, _ := ret[0].([]clinics.MembershipRestrictions)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// ListMembershipRestrictions indicates an expected call of ListMembershipRestrictions.
func (mr *MockServiceMockRecorder) ListMembershipRestrictions(ctx, clinicId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "ListMembershipRestrictions", reflect.TypeOf((*MockService)(nil).ListMembershipRestrictions), ctx, clinicId)
}

// RemoveAdmin mocks base method.
func (m *MockService) RemoveAdmin(ctx context.Context, clinicId, clinicianId string, allowOrphaning bool) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "RemoveAdmin", ctx, clinicId, clinicianId, allowOrphaning)
	ret0, _ := ret[0].(error)
	return ret0
}

// RemoveAdmin indicates an expected call of RemoveAdmin.
func (mr *MockServiceMockRecorder) RemoveAdmin(ctx, clinicId, clinicianId, allowOrphaning interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "RemoveAdmin", reflect.TypeOf((*MockService)(nil).RemoveAdmin), ctx, clinicId, clinicianId, allowOrphaning)
}

// Update mocks base method.
func (m *MockService) Update(ctx context.Context, id string, clinic *clinics.Clinic) (*clinics.Clinic, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "Update", ctx, id, clinic)
	ret0, _ := ret[0].(*clinics.Clinic)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// Update indicates an expected call of Update.
func (mr *MockServiceMockRecorder) Update(ctx, id, clinic interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "Update", reflect.TypeOf((*MockService)(nil).Update), ctx, id, clinic)
}

// UpdateEHRSettings mocks base method.
func (m *MockService) UpdateEHRSettings(ctx context.Context, clinicId string, settings *clinics.EHRSettings) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateEHRSettings", ctx, clinicId, settings)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateEHRSettings indicates an expected call of UpdateEHRSettings.
func (mr *MockServiceMockRecorder) UpdateEHRSettings(ctx, clinicId, settings interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateEHRSettings", reflect.TypeOf((*MockService)(nil).UpdateEHRSettings), ctx, clinicId, settings)
}

// UpdateMRNSettings mocks base method.
func (m *MockService) UpdateMRNSettings(ctx context.Context, clinicId string, settings *clinics.MRNSettings) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateMRNSettings", ctx, clinicId, settings)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateMRNSettings indicates an expected call of UpdateMRNSettings.
func (mr *MockServiceMockRecorder) UpdateMRNSettings(ctx, clinicId, settings interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateMRNSettings", reflect.TypeOf((*MockService)(nil).UpdateMRNSettings), ctx, clinicId, settings)
}

// UpdateMembershipRestrictions mocks base method.
func (m *MockService) UpdateMembershipRestrictions(ctx context.Context, clinicId string, restrictions []clinics.MembershipRestrictions) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateMembershipRestrictions", ctx, clinicId, restrictions)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateMembershipRestrictions indicates an expected call of UpdateMembershipRestrictions.
func (mr *MockServiceMockRecorder) UpdateMembershipRestrictions(ctx, clinicId, restrictions interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateMembershipRestrictions", reflect.TypeOf((*MockService)(nil).UpdateMembershipRestrictions), ctx, clinicId, restrictions)
}

// UpdatePatientTag mocks base method.
func (m *MockService) UpdatePatientTag(ctx context.Context, clinicId, tagId, tagName string) (*clinics.Clinic, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdatePatientTag", ctx, clinicId, tagId, tagName)
	ret0, _ := ret[0].(*clinics.Clinic)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// UpdatePatientTag indicates an expected call of UpdatePatientTag.
func (mr *MockServiceMockRecorder) UpdatePatientTag(ctx, clinicId, tagId, tagName interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdatePatientTag", reflect.TypeOf((*MockService)(nil).UpdatePatientTag), ctx, clinicId, tagId, tagName)
}

// UpdateSuppressedNotifications mocks base method.
func (m *MockService) UpdateSuppressedNotifications(ctx context.Context, clinicId string, suppressedNotifications clinics.SuppressedNotifications) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateSuppressedNotifications", ctx, clinicId, suppressedNotifications)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateSuppressedNotifications indicates an expected call of UpdateSuppressedNotifications.
func (mr *MockServiceMockRecorder) UpdateSuppressedNotifications(ctx, clinicId, suppressedNotifications interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateSuppressedNotifications", reflect.TypeOf((*MockService)(nil).UpdateSuppressedNotifications), ctx, clinicId, suppressedNotifications)
}

// UpdateTier mocks base method.
func (m *MockService) UpdateTier(ctx context.Context, clinicId, tier string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpdateTier", ctx, clinicId, tier)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpdateTier indicates an expected call of UpdateTier.
func (mr *MockServiceMockRecorder) UpdateTier(ctx, clinicId, tier interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpdateTier", reflect.TypeOf((*MockService)(nil).UpdateTier), ctx, clinicId, tier)
}

// UpsertAdmin mocks base method.
func (m *MockService) UpsertAdmin(ctx context.Context, clinicId, clinicianId string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "UpsertAdmin", ctx, clinicId, clinicianId)
	ret0, _ := ret[0].(error)
	return ret0
}

// UpsertAdmin indicates an expected call of UpsertAdmin.
func (mr *MockServiceMockRecorder) UpsertAdmin(ctx, clinicId, clinicianId interface{}) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "UpsertAdmin", reflect.TypeOf((*MockService)(nil).UpsertAdmin), ctx, clinicId, clinicianId)
}