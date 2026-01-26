package pagination

import (
	"fmt"
	"strconv"
)

// PaginationParams represents pagination query parameters
type PaginationParams struct {
	Page      int
	Limit     int
	Offset    int
	SortBy    string
	SortOrder string
}

// PaginationResponse represents paginated response
type PaginationResponse struct {
	Page       int         `json:"page"`
	Limit      int         `json:"limit"`
	Offset     int         `json:"offset"`
	Total      int64       `json:"total"`
	TotalPages int         `json:"total_pages"`
	Data       interface{} `json:"data"`
}

// Constants
const (
	DefaultPage  = 1
	DefaultLimit = 20
	MaxLimit     = 100
	MinLimit     = 1
)

// ParsePaginationParams parses pagination parameters from query string
func ParsePaginationParams(pageStr, limitStr string, sortBy, sortOrder string) (*PaginationParams, error) {
	page := DefaultPage
	limit := DefaultLimit

	// Parse page
	if pageStr != "" {
		p, err := strconv.Atoi(pageStr)
		if err != nil {
			return nil, fmt.Errorf("invalid page parameter: %w", err)
		}
		if p < 1 {
			page = DefaultPage
		} else {
			page = p
		}
	}

	// Parse limit
	if limitStr != "" {
		l, err := strconv.Atoi(limitStr)
		if err != nil {
			return nil, fmt.Errorf("invalid limit parameter: %w", err)
		}
		if l < MinLimit {
			limit = MinLimit
		} else if l > MaxLimit {
			limit = MaxLimit
		} else {
			limit = l
		}
	}

	// Calculate offset
	offset := (page - 1) * limit

	// Validate sort order
	if sortOrder != "asc" && sortOrder != "desc" {
		sortOrder = "desc" // Default to descending
	}

	return &PaginationParams{
		Page:      page,
		Limit:     limit,
		Offset:    offset,
		SortBy:    sortBy,
		SortOrder: sortOrder,
	}, nil
}

// CalculateOffset calculates offset from page and limit
func CalculateOffset(page, limit int) int {
	if page < 1 {
		page = 1
	}
	return (page - 1) * limit
}

// CalculateTotalPages calculates total pages from total count and limit
func CalculateTotalPages(total int64, limit int) int {
	if limit <= 0 {
		return 0
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	return totalPages
}

// BuildPaginationResponse creates a standardized pagination response
func BuildPaginationResponse(params *PaginationParams, total int64, data interface{}) *PaginationResponse {
	totalPages := CalculateTotalPages(total, params.Limit)

	return &PaginationResponse{
		Page:       params.Page,
		Limit:      params.Limit,
		Offset:     params.Offset,
		Total:      total,
		TotalPages: totalPages,
		Data:       data,
	}
}

// GetSortClause returns SQL sort clause
func GetSortClause(sortBy, sortOrder string) string {
	if sortBy == "" {
		return ""
	}
	order := "ASC"
	if sortOrder == "desc" {
		order = "DESC"
	}
	return fmt.Sprintf("ORDER BY %s %s", sortBy, order)
}
