package observability

import (
	"fmt"
	"net/http"
	"sort"
	"sync"
)

// Metrics accepts only fixed server-owned metric names; no request values become labels.
type Metrics struct {
	mu     sync.Mutex
	values map[string]float64
}

func NewMetrics() *Metrics { return &Metrics{values: make(map[string]float64)} }
func (m *Metrics) Add(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[name] += value
}
func (m *Metrics) Set(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[name] = value
}
func (m *Metrics) Observe(name string, value float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.values[name+"_sum"] += value
	m.values[name+"_count"]++
}
func (m *Metrics) ServeHTTP(w http.ResponseWriter, _ *http.Request) {
	m.mu.Lock()
	defer m.mu.Unlock()
	keys := make([]string, 0, len(m.values))
	for key := range m.values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	w.Header().Set("Content-Type", "text/plain; version=0.0.4")
	for _, key := range keys {
		fmt.Fprintf(w, "api_engine_%s %g\n", key, m.values[key])
	}
}
