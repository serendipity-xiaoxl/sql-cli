package db

import (
	"context"
	"fmt"
	"time"

	"github.com/xiaoxl/sql-cli/internal/sqlnorm"
	"github.com/xiaoxl/sql-cli/pkg/result"
)

// convertRowBytes converts []byte values to string after MapScan.
// sqlx.MapScan returns []byte for VARCHAR/TEXT columns, and json.Marshal
// encodes []byte as base64. This ensures string values are encoded as strings.
func convertRowBytes(row map[string]interface{}) {
	for k, v := range row {
		if b, ok := v.([]byte); ok {
			row[k] = string(b)
		}
	}
}

// ErrNonSelectQuery is returned when Query/QueryWithLimit is called with a non-SELECT statement.
var ErrNonSelectQuery = fmt.Errorf("only SELECT queries are allowed")

// Query executes a SELECT query with mandatory LIMIT enforcement.
// If the SQL does not contain a LIMIT clause, one is auto-appended using the
// configured DefaultLimit. The limit is capped at MaxLimit.
func (s *Session) Query(ctx context.Context, sqlStr string, args ...interface{}) (*result.QueryResult, error) {
	return s.queryWithLimit(ctx, sqlStr, 0, args...)
}

// QueryWithLimit executes a SELECT query with a caller-specified page size.
// The limit is clamped to the session's configured MaxLimit.
// If the SQL already has a LIMIT clause, the caller-specified limit is ignored
// and a warning is logged.
func (s *Session) QueryWithLimit(ctx context.Context, sqlStr string, limit int, args ...interface{}) (*result.QueryResult, error) {
	return s.queryWithLimit(ctx, sqlStr, limit, args...)
}

// queryWithLimit is the shared implementation for Query and QueryWithLimit.
func (s *Session) queryWithLimit(ctx context.Context, sqlStr string, limit int, args ...interface{}) (*result.QueryResult, error) {
	// Apply query timeout from config if no deadline is set
	if s.cfg.QueryTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.cfg.QueryTimeout)
			defer cancel()
		}
	}

	op := sqlnorm.Operation(sqlStr)

	// Reject non-SELECT statements
	if !sqlnorm.IsSELECT(op) {
		return nil, fmt.Errorf("query %s: %w", op, ErrNonSelectQuery)
	}

	// Enforce LIMIT
	hasExistingLimit := sqlnorm.HasLIMIT(sqlStr)
	appliedLimit := 0

	if hasExistingLimit {
		// SQL already has LIMIT — pass through, log if explicit limit provided
		if limit > 0 {
			s.logger.Warn("query ignores explicit limit — SQL already has LIMIT clause",
				"explicit_limit", limit,
			)
		}
	} else {
		// Auto-append LIMIT
		appliedLimit = limit
		if appliedLimit <= 0 {
			appliedLimit = s.cfg.DefaultLimit
		}
		if appliedLimit > s.cfg.MaxLimit {
			appliedLimit = s.cfg.MaxLimit
		}
		sqlStr = sqlnorm.AppendLIMIT(sqlStr, appliedLimit)
	}

	// Build warning message
	var warning string
	if hasExistingLimit {
		warning = ""
	} else {
		if limit > s.cfg.MaxLimit {
			warning = fmt.Sprintf("LIMIT capped to %d (max allowed)", appliedLimit)
		} else {
			warning = fmt.Sprintf("LIMIT %d applied automatically", appliedLimit)
		}
	}

	sqlStr = s.Rebind(sqlStr)
	start := time.Now()
	rows, err := s.db.QueryxContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}

	var resultRows [][]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err != nil {
			return nil, fmt.Errorf("query %s: %w", op, err)
		}
		convertRowBytes(row)
		rowSlice := make([]interface{}, len(columns))
		for i, col := range columns {
			rowSlice[i] = row[col]
		}
		resultRows = append(resultRows, rowSlice)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}

	duration := time.Since(start)
	hasMore := appliedLimit > 0 && len(resultRows) >= appliedLimit

	s.logger.Info("query",
		"duration_ms", duration.Milliseconds(),
		"row_count", len(resultRows),
		"warning", warning,
		"has_more", hasMore,
	)

	res := result.NewQueryResult(columns, resultRows, duration, warning)
	res.HasMore = hasMore
	return res, nil
}

// QueryWithOffset executes a SELECT with pagination (LIMIT + OFFSET).
func (s *Session) QueryWithOffset(ctx context.Context, sqlStr string, limit, offset int, args ...interface{}) (*result.QueryResult, error) {
	return s.queryWithOffset(ctx, sqlStr, limit, offset, args...)
}

// queryWithOffset is the shared paginated query implementation.
func (s *Session) queryWithOffset(ctx context.Context, sqlStr string, limit, offset int, args ...interface{}) (*result.QueryResult, error) {
	// Apply query timeout from config if no deadline is set
	if s.cfg.QueryTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.cfg.QueryTimeout)
			defer cancel()
		}
	}

	op := sqlnorm.Operation(sqlStr)

	// Reject non-SELECT statements
	if !sqlnorm.IsSELECT(op) {
		return nil, fmt.Errorf("query %s: %w", op, ErrNonSelectQuery)
	}

	// Enforce LIMIT
	hasExistingLimit := sqlnorm.HasLIMIT(sqlStr)
	appliedLimit := 0

	if hasExistingLimit {
		if limit > 0 {
			s.logger.Warn("query ignores explicit limit — SQL already has LIMIT clause",
				"explicit_limit", limit,
			)
		}
	} else {
		appliedLimit = limit
		if appliedLimit <= 0 {
			appliedLimit = s.cfg.DefaultLimit
		}
		if appliedLimit > s.cfg.MaxLimit {
			appliedLimit = s.cfg.MaxLimit
		}
		sqlStr = sqlnorm.AppendLIMIT(sqlStr, appliedLimit)
	}

	// Enforce OFFSET (only if no existing OFFSET)
	hasExistingOffset := sqlnorm.HasOFFSET(sqlStr)
	if !hasExistingOffset && offset > 0 {
		sqlStr = sqlnorm.AppendOFFSET(sqlStr, offset)
	}

	// Build warning message
	var warning string
	if hasExistingLimit {
		warning = ""
	} else {
		if limit > s.cfg.MaxLimit {
			warning = fmt.Sprintf("LIMIT capped to %d (max allowed)", appliedLimit)
		} else {
			warning = fmt.Sprintf("LIMIT %d applied automatically", appliedLimit)
		}
	}

	sqlStr = s.Rebind(sqlStr)
	start := time.Now()
	rows, err := s.db.QueryxContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}

	var resultRows [][]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err != nil {
			return nil, fmt.Errorf("query %s: %w", op, err)
		}
		convertRowBytes(row)
		rowSlice := make([]interface{}, len(columns))
		for i, col := range columns {
			rowSlice[i] = row[col]
		}
		resultRows = append(resultRows, rowSlice)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}

	duration := time.Since(start)
	hasMore := appliedLimit > 0 && len(resultRows) >= appliedLimit

	s.logger.Info("query_with_offset",
		"duration_ms", duration.Milliseconds(),
		"row_count", len(resultRows),
		"offset", offset,
		"warning", warning,
		"has_more", hasMore,
	)

	res := result.NewQueryResult(columns, resultRows, duration, warning)
	res.HasMore = hasMore
	return res, nil
}

// QueryOptions specifies optional parameters for a query.
type QueryOptions struct {
	// Limit is the maximum number of rows to return. 0 uses the configured DefaultLimit.
	Limit int
	// Offset is the number of rows to skip. 0 means no offset.
	Offset int
}

// QueryWithOptions executes a SELECT with the given options (pagination support).
func (s *Session) QueryWithOptions(ctx context.Context, sqlStr string, opts QueryOptions, args ...interface{}) (*result.QueryResult, error) {
	// Apply query timeout from config if no deadline is set
	if s.cfg.QueryTimeout > 0 {
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, s.cfg.QueryTimeout)
			defer cancel()
		}
	}

	op := sqlnorm.Operation(sqlStr)

	// Reject non-SELECT statements
	if !sqlnorm.IsSELECT(op) {
		return nil, fmt.Errorf("query %s: %w", op, ErrNonSelectQuery)
	}

	// Use ApplyCursor for LIMIT/OFFSET enforcement
	hasExistingLimit := sqlnorm.HasLIMIT(sqlStr)
	hasExistingOffset := sqlnorm.HasOFFSET(sqlStr)

	if !hasExistingLimit {
		limit := opts.Limit
		if limit <= 0 {
			limit = s.cfg.DefaultLimit
		}
		if limit > s.cfg.MaxLimit {
			limit = s.cfg.MaxLimit
		}

		// Build warning
		var warning string
		if opts.Limit > s.cfg.MaxLimit {
			warning = fmt.Sprintf("LIMIT capped to %d (max allowed)", limit)
		} else {
			warning = fmt.Sprintf("LIMIT %d applied automatically", limit)
		}

		var err error
		sqlStr, err = sqlnorm.ApplyCursor(sqlStr, limit, opts.Offset, s.cfg.MaxLimit)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", op, err)
		}

		start := time.Now()
		rows, err := s.db.QueryxContext(ctx, sqlStr, args...)
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", op, err)
		}
		defer rows.Close()

		columns, err := rows.Columns()
		if err != nil {
			return nil, fmt.Errorf("query %s: %w", op, err)
		}

		var resultRows [][]interface{}
		for rows.Next() {
			row := make(map[string]interface{})
			if err := rows.MapScan(row); err != nil {
				return nil, fmt.Errorf("query %s: %w", op, err)
			}
		convertRowBytes(row)
			rowSlice := make([]interface{}, len(columns))
			for i, col := range columns {
				rowSlice[i] = row[col]
			}
			resultRows = append(resultRows, rowSlice)
		}
		if err := rows.Err(); err != nil {
			return nil, fmt.Errorf("query %s: %w", op, err)
		}

		duration := time.Since(start)
		hasMore := limit > 0 && len(resultRows) >= limit

		s.logger.Info("query_with_options",
			"duration_ms", duration.Milliseconds(),
			"row_count", len(resultRows),
			"limit", limit,
			"offset", opts.Offset,
			"warning", warning,
			"has_more", hasMore,
		)

		res := result.NewQueryResult(columns, resultRows, duration, warning)
		res.HasMore = hasMore
		return res, nil
	}

	// SQL already has LIMIT — pass through unchanged
	if opts.Limit > 0 {
		s.logger.Warn("query ignores explicit limit — SQL already has LIMIT clause",
			"explicit_limit", opts.Limit,
		)
	}

	// If SQL has LIMIT but no OFFSET, and opts.Offset > 0, append OFFSET
	if !hasExistingOffset && opts.Offset > 0 {
		sqlStr = sqlnorm.AppendOFFSET(sqlStr, opts.Offset)
	}

	sqlStr = s.Rebind(sqlStr)
	start := time.Now()
	rows, err := s.db.QueryxContext(ctx, sqlStr, args...)
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}

	var resultRows [][]interface{}
	for rows.Next() {
		row := make(map[string]interface{})
		if err := rows.MapScan(row); err != nil {
			return nil, fmt.Errorf("query %s: %w", op, err)
		}
		convertRowBytes(row)
		rowSlice := make([]interface{}, len(columns))
		for i, col := range columns {
			rowSlice[i] = row[col]
		}
		resultRows = append(resultRows, rowSlice)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("query %s: %w", op, err)
	}

	duration := time.Since(start)
	hasMore := false // with existing LIMIT, cannot determine has_more

	s.logger.Info("query_with_options",
		"duration_ms", duration.Milliseconds(),
		"row_count", len(resultRows),
		"offset", opts.Offset,
		"warning", "",
		"has_more", hasMore,
	)

	res := result.NewQueryResult(columns, resultRows, duration, "")
	res.HasMore = hasMore
	return res, nil
}
