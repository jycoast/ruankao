package handler

import (
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func ctxWithQuery(query string) *gin.Context {
	gin.SetMode(gin.TestMode)
	c, _ := gin.CreateTestContext(httptest.NewRecorder())
	c.Request = httptest.NewRequest("GET", "/?"+query, nil)
	return c
}

func TestParsePage(t *testing.T) {
	cases := []struct {
		query        string
		wantPage     int
		wantPageSize int
	}{
		{"", 1, 20},
		{"page=3&page_size=30", 3, 30},
		{"page=0&page_size=0", 1, 20},
		{"page=-5&page_size=-1", 1, 20},
		{"page_size=10000", 1, maxPageSize},
		{"page_size=50", 1, 50},
		{"page=abc&page_size=abc", 1, 20},
	}
	for _, tc := range cases {
		page, pageSize := parsePage(ctxWithQuery(tc.query))
		if page != tc.wantPage || pageSize != tc.wantPageSize {
			t.Errorf("query %q: got (%d,%d), want (%d,%d)", tc.query, page, pageSize, tc.wantPage, tc.wantPageSize)
		}
	}
}

func TestClampCount(t *testing.T) {
	cases := []struct {
		in, def, max, want int
	}{
		{10, 10, 50, 10},
		{0, 10, 50, 10},
		{-3, 10, 50, 10},
		{10000, 10, 50, 50},
		{50, 10, 50, 50},
	}
	for _, tc := range cases {
		if got := clampCount(tc.in, tc.def, tc.max); got != tc.want {
			t.Errorf("clampCount(%d,%d,%d)=%d, want %d", tc.in, tc.def, tc.max, got, tc.want)
		}
	}
}
