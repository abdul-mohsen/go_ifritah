package helpers

import "math"

type Pagination struct {
	Page       int
	PerPage    int
	Total      int
	TotalPages int
}

func PaginateSlice[T any](items []T, page int, perPage int) ([]T, Pagination) {
	if perPage <= 0 {
		perPage = 10
	}
	if page < 0 {
		page = 0
	}

	total := len(items)
	totalPages := int(math.Ceil(float64(total) / float64(perPage)))
	if totalPages == 0 {
		totalPages = 1
	}

	start := page * perPage
	if start > total {
		start = total
	}
	end := start + perPage
	if end > total {
		end = total
	}

	paged := items[start:end]
	return paged, Pagination{
		Page:       page,
		PerPage:    perPage,
		Total:      total,
		TotalPages: totalPages,
	}
}
