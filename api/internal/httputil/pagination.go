package httputil

import (
	"net/http"
	"strconv"
)

type Page struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
}

type PageResponse struct {
	Offset int `json:"offset"`
	Limit  int `json:"limit"`
	Total  int `json:"total"`
	Items  any `json:"items"`
}

func ParsePagination(r *http.Request) (offset, limit int) {
	offset = queryInt(r, "offset", 0)
	limit = queryInt(r, "limit", 25)
	if offset < 0 {
		offset = 0
	}
	if limit < 1 {
		limit = 1
	}
	if limit > 200 {
		limit = 200
	}
	return offset, limit
}

func Paginate[T any](items []T, offset, limit int) PageResponse {
	total := len(items)
	if offset >= total {
		return PageResponse{Offset: offset, Limit: limit, Total: total, Items: []T{}}
	}
	end := offset + limit
	if end > total {
		end = total
	}
	return PageResponse{Offset: offset, Limit: limit, Total: total, Items: items[offset:end]}
}

func queryInt(r *http.Request, key string, def int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	return v
}
