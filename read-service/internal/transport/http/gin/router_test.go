package gin

// ATG Update

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ATG Update: This test verifies that all expected routes are registered in the Gin router.
// It serves as a smoke test to prevent regression where creating a handler but forgetting to register it
// causes the API to return 404s.
func TestRouter_Routes(t *testing.T) {
	// We pass nil pool because we only want to test route registration,
	// not the actual handler execution (which would panic on nil pool).
	// In a real unit test we would mock the dependencies.

	// However, NewRouter creates handlers that depend on pool internally.
	// To test that routes are registered, we can invoke the router
	// and assert we don't get 404.

	router := NewRouter(&pgxpool.Pool{}) // Use empty struct pointer to avoid immediate nil pointer dereference in NewRouter if it stores it.
	// Note: NewRouter stores pool in db.New(pool). db.New takes DBTX interface.
	// pgxpool.Pool implementation of DBTX methods (Exec, Query, etc) will panic if pool is not initialized or nil.
	// But they are not called until Handle() is executed.

	tests := []struct {
		name   string
		method string
		path   string
	}{
		{"Health", "GET", "/health"},
		{"Ready", "GET", "/ready"},
		{"ListSeries", "GET", "/v1/series"},
		{"GetSeriesState", "GET", "/v1/series/state/source|metric|test|us-east-1"}, // ATG Update: Valid key format required for parameter matching
		{"ListAnomalies", "GET", "/v1/anomalies"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := httptest.NewRecorder()
			req, _ := http.NewRequest(tt.method, tt.path, nil)
			router.ServeHTTP(w, req)

			// We expect anything BUT 404.
			// 500 is expected because of nil pool/DB connection.
			// 200 is expected for /health.
			if w.Code == http.StatusNotFound {
				t.Errorf("Route %s %s not registered (got 404)", tt.method, tt.path)
			}

			if tt.path == "/health" && w.Code != http.StatusOK {
				t.Errorf("Health check failed: got %d want 200", w.Code)
			}
		})
	}
}
