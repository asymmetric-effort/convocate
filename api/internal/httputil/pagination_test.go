package httputil

import (
	"net/http/httptest"
	"testing"
)

func TestParsePagination_Defaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/", nil)
	offset, limit := ParsePagination(r)
	if offset != 0 {
		t.Errorf("offset = %d, want 0", offset)
	}
	if limit != 25 {
		t.Errorf("limit = %d, want 25", limit)
	}
}

func TestParsePagination_Custom(t *testing.T) {
	r := httptest.NewRequest("GET", "/?offset=10&limit=50", nil)
	offset, limit := ParsePagination(r)
	if offset != 10 {
		t.Errorf("offset = %d, want 10", offset)
	}
	if limit != 50 {
		t.Errorf("limit = %d, want 50", limit)
	}
}

func TestParsePagination_MaxLimit(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=500", nil)
	_, limit := ParsePagination(r)
	if limit != 200 {
		t.Errorf("limit = %d, want 200 (max)", limit)
	}
}

func TestPaginate(t *testing.T) {
	items := []string{"a", "b", "c", "d", "e"}

	page := Paginate(items, 0, 3)
	if page.Total != 5 {
		t.Errorf("total = %d, want 5", page.Total)
	}
	result := page.Items.([]string)
	if len(result) != 3 {
		t.Errorf("items len = %d, want 3", len(result))
	}

	page2 := Paginate(items, 3, 3)
	result2 := page2.Items.([]string)
	if len(result2) != 2 {
		t.Errorf("items len = %d, want 2", len(result2))
	}

	page3 := Paginate(items, 10, 3)
	result3 := page3.Items.([]string)
	if len(result3) != 0 {
		t.Errorf("items len = %d, want 0", len(result3))
	}
}
