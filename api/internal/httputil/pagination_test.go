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

func TestParsePagination_NegativeOffset(t *testing.T) {
	r := httptest.NewRequest("GET", "/?offset=-5", nil)
	offset, _ := ParsePagination(r)
	if offset != 0 {
		t.Errorf("offset = %d, want 0 (clamped)", offset)
	}
}

func TestParsePagination_ZeroLimit(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=0", nil)
	_, limit := ParsePagination(r)
	if limit != 1 {
		t.Errorf("limit = %d, want 1 (min)", limit)
	}
}

func TestParsePagination_NegativeLimit(t *testing.T) {
	r := httptest.NewRequest("GET", "/?limit=-10", nil)
	_, limit := ParsePagination(r)
	if limit != 1 {
		t.Errorf("limit = %d, want 1 (min)", limit)
	}
}

func TestParsePagination_InvalidValues(t *testing.T) {
	r := httptest.NewRequest("GET", "/?offset=abc&limit=xyz", nil)
	offset, limit := ParsePagination(r)
	if offset != 0 {
		t.Errorf("offset = %d, want 0 (default for invalid)", offset)
	}
	if limit != 25 {
		t.Errorf("limit = %d, want 25 (default for invalid)", limit)
	}
}

func TestPaginate_EmptySlice(t *testing.T) {
	page := Paginate([]int{}, 0, 10)
	if page.Total != 0 {
		t.Errorf("total = %d, want 0", page.Total)
	}
	result := page.Items.([]int)
	if len(result) != 0 {
		t.Errorf("items len = %d, want 0", len(result))
	}
}

func TestPaginate_ExactBoundary(t *testing.T) {
	items := []string{"a", "b", "c"}
	page := Paginate(items, 0, 3)
	result := page.Items.([]string)
	if len(result) != 3 {
		t.Errorf("items len = %d, want 3", len(result))
	}
	if page.Total != 3 {
		t.Errorf("total = %d, want 3", page.Total)
	}
}

func TestPaginate_OffsetAtTotal(t *testing.T) {
	items := []string{"a", "b"}
	page := Paginate(items, 2, 5)
	result := page.Items.([]string)
	if len(result) != 0 {
		t.Errorf("items len = %d, want 0", len(result))
	}
}
