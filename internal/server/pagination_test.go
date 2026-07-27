package server

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParseCursorLimitBoundaries(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name        string
		query       string
		defaultSize int
		maxSize     int
		want        int
		wantOK      bool
		wantStatus  int
	}{
		{name: "default", defaultSize: 50, maxSize: 100, want: 50, wantOK: true, wantStatus: http.StatusOK},
		{name: "maximum", query: "?limit=100", defaultSize: 50, maxSize: 100, want: 100, wantOK: true, wantStatus: http.StatusOK},
		{name: "above maximum", query: "?limit=101", defaultSize: 50, maxSize: 100, wantOK: false, wantStatus: http.StatusBadRequest},
		{name: "zero", query: "?limit=0", defaultSize: 50, maxSize: 100, wantOK: false, wantStatus: http.StatusBadRequest},
		{name: "not a number", query: "?limit=many", defaultSize: 50, maxSize: 100, wantOK: false, wantStatus: http.StatusBadRequest},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			recorder := httptest.NewRecorder()
			context, _ := gin.CreateTestContext(recorder)
			context.Request = httptest.NewRequest(http.MethodGet, "/items"+tt.query, nil)

			got, ok := parseCursorLimit(context, tt.defaultSize, tt.maxSize)
			if got != tt.want || ok != tt.wantOK {
				t.Fatalf("parseCursorLimit() = %d, %t; want %d, %t", got, ok, tt.want, tt.wantOK)
			}
			if recorder.Code != tt.wantStatus {
				t.Fatalf("status = %d, want %d", recorder.Code, tt.wantStatus)
			}
		})
	}
}

func TestRejectLegacyPagination(t *testing.T) {
	t.Parallel()

	recorder := httptest.NewRecorder()
	context, _ := gin.CreateTestContext(recorder)
	context.Request = httptest.NewRequest(http.MethodGet, "/items?offset=0", nil)

	if !rejectLegacyPagination(context, "page", "offset") {
		t.Fatal("legacy offset parameter was accepted")
	}
	if recorder.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d", recorder.Code, http.StatusBadRequest)
	}
}
