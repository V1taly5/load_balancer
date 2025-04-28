package healthchecker

import (
	"loadbalancer/internal/balancer"
	"net/url"
	"sync"

	"github.com/stretchr/testify/mock"
)

type MockBackend struct {
	mock.Mock
	url    *url.URL
	isDown bool
	mu     sync.Mutex
}

func (m *MockBackend) URL() *url.URL {
	return m.url
}

func (m *MockBackend) IsDown() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.isDown
}

func (m *MockBackend) SetHealth(down bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isDown = down
}

// Мок балансировщика
type MockBalancer struct {
	mock.Mock
	backends []*balancer.Backend
}

func (m *MockBalancer) Next() (*balancer.Backend, error) {
	args := m.Called()
	return args.Get(0).(*balancer.Backend), args.Error(1)
}

func (m *MockBalancer) MarkAsDown(b *balancer.Backend) {
	m.Called(b)
}

func (m *MockBalancer) AddBackend(b balancer.Backend) {
	m.Called(b)
}

func (m *MockBalancer) RemoveBackend(url string) bool {
	args := m.Called(url)
	return args.Bool(0)
}

func (m *MockBalancer) GetAllBackends() []*balancer.Backend {
	args := m.Called()
	return args.Get(0).([]*balancer.Backend)
}
