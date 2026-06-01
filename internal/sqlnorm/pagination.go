package sqlnorm

import (
	"fmt"
	"strings"
)

// Pagination describes the paging state for a query.
type Pagination struct {
	Limit  int `json:"limit"`
	Offset int `json:"offset"`
}

// ApplyCursor appends LIMIT/OFFSET to a SQL statement, respecting the maxLimit.
// If offset <= 0, only LIMIT is appended. Returns an error if limit or offset is negative.
func ApplyCursor(sql string, limit, offset, maxLimit int) (string, error) {
	if limit < 0 {
		return "", fmt.Errorf("negative limit: %d", limit)
	}
	if offset < 0 {
		return "", fmt.Errorf("negative offset: %d", offset)
	}

	appliedLimit := limit
	if appliedLimit <= 0 {
		appliedLimit = 100 // sensible default
	}
	if maxLimit > 0 && appliedLimit > maxLimit {
		appliedLimit = maxLimit
	}

	s := strings.TrimSpace(sql)
	hasSemi := strings.HasSuffix(s, ";")
	if hasSemi {
		s = s[:len(s)-1]
	}

	if offset > 0 {
		s = fmt.Sprintf("%s LIMIT %d OFFSET %d", s, appliedLimit, offset)
	} else {
		s = fmt.Sprintf("%s LIMIT %d", s, appliedLimit)
	}

	if hasSemi {
		s += ";"
	}

	return s, nil
}

// IsZero returns true if the pagination is at its default (first page).
func (p Pagination) IsZero() bool {
	return p.Limit <= 0 && p.Offset <= 0
}
