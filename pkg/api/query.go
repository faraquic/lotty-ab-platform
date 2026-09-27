package api

import (
	"strings"

	"github.com/gin-gonic/gin"
)

// DefaultLimit is the fallback page size for list endpoints.
const DefaultLimit = 20

// MaxLimit is the maximum accepted page size for list endpoints.
const MaxLimit = 100

// MaxSearchLen caps the free-text search query length.
const MaxSearchLen = 128

// ListQuery carries the common pagination, search, and sort parameters shared
// by list endpoints.
type ListQuery struct {
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
	Q      string `form:"q"`
	Sort   string `form:"sort"`
	Order  string `form:"order"`
}

// Normalize clamps pagination values, trims the search string, and normalizes
// sort fields.
func (q *ListQuery) Normalize() {
	if q.Limit < 1 || q.Limit > MaxLimit {
		q.Limit = DefaultLimit
	}
	if q.Offset < 0 {
		q.Offset = 0
	}
	q.Q = strings.TrimSpace(q.Q)
	if len(q.Q) > MaxSearchLen {
		q.Q = q.Q[:MaxSearchLen]
	}
	q.Sort = strings.TrimSpace(q.Sort)
	q.Order = strings.ToLower(strings.TrimSpace(q.Order))
	if q.Order != "asc" && q.Order != "desc" {
		q.Order = "asc"
	}
}

// Search returns the trimmed query as a pointer, or nil when empty.
func (q ListQuery) Search() *string {
	if q.Q == "" {
		return nil
	}
	return &q.Q
}

// BindListQuery binds the common pagination/search parameters from the query
// string. On binding failure it writes a 400 response and returns false.
func BindListQuery(c *gin.Context, dst any) bool {
	if err := c.ShouldBindQuery(dst); err != nil {
		Error(c.Writer, 400, BadRequest, err.Error())
		return false
	}
	return true
}
