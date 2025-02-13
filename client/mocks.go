package client

//go:generate mockgen -source=./client.go -destination=./mock.go -package client ClientInterface
//go:generate mockgen -source=./client.go -destination=./mock.go -package client ClientWithResponsesInterface

import "go.uber.org/mock/gomock"

func (m *MockClientInterface) Reset(ctrl *gomock.Controller) {
	m.ctrl = ctrl
	m.recorder = &MockClientInterfaceMockRecorder{mock: m}
}

func (m *MockClientWithResponsesInterface) Reset(ctrl *gomock.Controller) {
	m.ctrl = ctrl
	m.recorder = &MockClientWithResponsesInterfaceMockRecorder{mock: m}
}
