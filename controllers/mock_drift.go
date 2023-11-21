// Code generated by MockGen. DO NOT EDIT.
// Source: controllers/drift.go
//
// Generated by this command:
//
//	mockgen -source controllers/drift.go -package controllers -self_package=github.com/hybrid-cloud-patterns/patterns-operator/controllers
//
// Package controllers is a generated GoMock package.
package controllers

import (
	reflect "reflect"

	v5 "github.com/go-git/go-git/v5"
	config "github.com/go-git/go-git/v5/config"
	plumbing "github.com/go-git/go-git/v5/plumbing"
	gomock "go.uber.org/mock/gomock"
)

// MockRemoteClient is a mock of RemoteClient interface.
type MockRemoteClient struct {
	ctrl     *gomock.Controller
	recorder *MockRemoteClientMockRecorder
}

// MockRemoteClientMockRecorder is the mock recorder for MockRemoteClient.
type MockRemoteClientMockRecorder struct {
	mock *MockRemoteClient
}

// NewMockRemoteClient creates a new mock instance.
func NewMockRemoteClient(ctrl *gomock.Controller) *MockRemoteClient {
	mock := &MockRemoteClient{ctrl: ctrl}
	mock.recorder = &MockRemoteClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockRemoteClient) EXPECT() *MockRemoteClientMockRecorder {
	return m.recorder
}

// List mocks base method.
func (m *MockRemoteClient) List(o *v5.ListOptions) ([]*plumbing.Reference, error) {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "List", o)
	ret0, _ := ret[0].([]*plumbing.Reference)
	ret1, _ := ret[1].(error)
	return ret0, ret1
}

// List indicates an expected call of List.
func (mr *MockRemoteClientMockRecorder) List(o any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "List", reflect.TypeOf((*MockRemoteClient)(nil).List), o)
}

// MockGitClient is a mock of GitClient interface.
type MockGitClient struct {
	ctrl     *gomock.Controller
	recorder *MockGitClientMockRecorder
}

// MockGitClientMockRecorder is the mock recorder for MockGitClient.
type MockGitClientMockRecorder struct {
	mock *MockGitClient
}

// NewMockGitClient creates a new mock instance.
func NewMockGitClient(ctrl *gomock.Controller) *MockGitClient {
	mock := &MockGitClient{ctrl: ctrl}
	mock.recorder = &MockGitClientMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockGitClient) EXPECT() *MockGitClientMockRecorder {
	return m.recorder
}

// NewRemoteClient mocks base method.
func (m *MockGitClient) NewRemoteClient(c *config.RemoteConfig) RemoteClient {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "NewRemoteClient", c)
	ret0, _ := ret[0].(RemoteClient)
	return ret0
}

// NewRemoteClient indicates an expected call of NewRemoteClient.
func (mr *MockGitClientMockRecorder) NewRemoteClient(c any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "NewRemoteClient", reflect.TypeOf((*MockGitClient)(nil).NewRemoteClient), c)
}

// MockdriftWatcher is a mock of driftWatcher interface.
type MockdriftWatcher struct {
	ctrl     *gomock.Controller
	recorder *MockdriftWatcherMockRecorder
}

// MockdriftWatcherMockRecorder is the mock recorder for MockdriftWatcher.
type MockdriftWatcherMockRecorder struct {
	mock *MockdriftWatcher
}

// NewMockdriftWatcher creates a new mock instance.
func NewMockdriftWatcher(ctrl *gomock.Controller) *MockdriftWatcher {
	mock := &MockdriftWatcher{ctrl: ctrl}
	mock.recorder = &MockdriftWatcherMockRecorder{mock}
	return mock
}

// EXPECT returns an object that allows the caller to indicate expected use.
func (m *MockdriftWatcher) EXPECT() *MockdriftWatcherMockRecorder {
	return m.recorder
}

// add mocks base method.
func (m *MockdriftWatcher) add(name, namespace string, interval int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "add", name, namespace, interval)
	ret0, _ := ret[0].(error)
	return ret0
}

// add indicates an expected call of add.
func (mr *MockdriftWatcherMockRecorder) add(name, namespace, interval any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "add", reflect.TypeOf((*MockdriftWatcher)(nil).add), name, namespace, interval)
}

// isWatching mocks base method.
func (m *MockdriftWatcher) isWatching(name, namespace string) bool {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "isWatching", name, namespace)
	ret0, _ := ret[0].(bool)
	return ret0
}

// isWatching indicates an expected call of isWatching.
func (mr *MockdriftWatcherMockRecorder) isWatching(name, namespace any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "isWatching", reflect.TypeOf((*MockdriftWatcher)(nil).isWatching), name, namespace)
}

// remove mocks base method.
func (m *MockdriftWatcher) remove(name, namespace string) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "remove", name, namespace)
	ret0, _ := ret[0].(error)
	return ret0
}

// remove indicates an expected call of remove.
func (mr *MockdriftWatcherMockRecorder) remove(name, namespace any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "remove", reflect.TypeOf((*MockdriftWatcher)(nil).remove), name, namespace)
}

// updateInterval mocks base method.
func (m *MockdriftWatcher) updateInterval(name, namespace string, interval int) error {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "updateInterval", name, namespace, interval)
	ret0, _ := ret[0].(error)
	return ret0
}

// updateInterval indicates an expected call of updateInterval.
func (mr *MockdriftWatcherMockRecorder) updateInterval(name, namespace, interval any) *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "updateInterval", reflect.TypeOf((*MockdriftWatcher)(nil).updateInterval), name, namespace, interval)
}

// watch mocks base method.
func (m *MockdriftWatcher) watch() chan any {
	m.ctrl.T.Helper()
	ret := m.ctrl.Call(m, "watch")
	ret0, _ := ret[0].(chan any)
	return ret0
}

// watch indicates an expected call of watch.
func (mr *MockdriftWatcherMockRecorder) watch() *gomock.Call {
	mr.mock.ctrl.T.Helper()
	return mr.mock.ctrl.RecordCallWithMethodType(mr.mock, "watch", reflect.TypeOf((*MockdriftWatcher)(nil).watch))
}