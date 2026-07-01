package handler

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestParsePagination(t *testing.T) {
	gin.SetMode(gin.TestMode)

	tests := []struct {
		name            string
		rawQuery        string
		defaultPageSize int
		wantPage        int
		wantPageSize    int
	}{
		{
			name:            "uses defaults",
			defaultPageSize: 10,
			wantPage:        1,
			wantPageSize:    10,
		},
		{
			name:            "accepts valid values",
			rawQuery:        "page=3&page_size=25",
			defaultPageSize: 10,
			wantPage:        3,
			wantPageSize:    25,
		},
		{
			name:            "rejects zero and negative values",
			rawQuery:        "page=0&page_size=-5",
			defaultPageSize: 20,
			wantPage:        1,
			wantPageSize:    20,
		},
		{
			name:            "rejects non numeric values",
			rawQuery:        "page=abc&page_size=xyz",
			defaultPageSize: 20,
			wantPage:        1,
			wantPageSize:    20,
		},
		{
			name:            "caps oversized page size",
			rawQuery:        "page=2&page_size=1000",
			defaultPageSize: 10,
			wantPage:        2,
			wantPageSize:    maxPageSize,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, _ := gin.CreateTestContext(httptest.NewRecorder())
			req := httptest.NewRequest(http.MethodGet, "/?"+tt.rawQuery, nil)
			ctx.Request = req

			page, pageSize := parsePagination(ctx, tt.defaultPageSize)
			if page != tt.wantPage {
				t.Fatalf("page = %d, want %d", page, tt.wantPage)
			}
			if pageSize != tt.wantPageSize {
				t.Fatalf("pageSize = %d, want %d", pageSize, tt.wantPageSize)
			}
		})
	}
}
