package kong

import (
	"net/url"
	"strconv"
)

// PaginationResponse represents the standard Kong API pagination response structure
type PaginationResponse struct {
	Data   []interface{} `json:"data"`
	Offset string        `json:"offset"`
}

// BuildPaginationParams builds the standard pagination query parameters for Kong API
func BuildPaginationParams(pageSize int, offset string) url.Values {
	if pageSize <= 0 {
		pageSize = 1000 // Kong default page size
	}

	params := url.Values{}
	params.Add("size", strconv.Itoa(pageSize))
	if offset != "" {
		params.Add("offset", offset)
	}

	return params
}

// PaginationHelper provides a helper for iterating through paginated results
// It manages the loop logic so callers just need to define what to do with each page
type PaginationHelper struct {
	PageSize int
	Offset   string
}

// NewPaginationHelper creates a new pagination helper with default page size
func NewPaginationHelper() *PaginationHelper {
	return &PaginationHelper{
		PageSize: 1000,
		Offset:   "",
	}
}

// GetParams returns the URL parameters for the current page
func (ph *PaginationHelper) GetParams() url.Values {
	return BuildPaginationParams(ph.PageSize, ph.Offset)
}

// HasMore checks if there are more pages to fetch
// lastPageDataLength is the length of the data returned in the last response
// nextOffset is the offset value from the last response
func (ph *PaginationHelper) HasMore(nextOffset string, lastPageDataLength int) bool {
	if nextOffset == "" || lastPageDataLength < ph.PageSize {
		return false
	}
	ph.Offset = nextOffset
	return true
}

// Reset resets the pagination helper to start over
func (ph *PaginationHelper) Reset() {
	ph.Offset = ""
}
