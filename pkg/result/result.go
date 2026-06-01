// Package result provides structured types for database operation results,
// designed for easy serialization (JSON) and Agent consumption.
package result

import (
	"time"
)

// ExecResult holds the outcome of an Exec call (DDL/DML).
type ExecResult struct {
	LastInsertID int64  `json:"last_insert_id,omitempty"`
	RowsAffected int64  `json:"rows_affected"`
	DurationMs   int64  `json:"duration_ms"`
	Error        string `json:"error,omitempty"`
}

// NewExecResult creates an ExecResult with the given parameters.
func NewExecResult(lastInsertID, rowsAffected int64, duration time.Duration) *ExecResult {
	return &ExecResult{
		LastInsertID: lastInsertID,
		RowsAffected: rowsAffected,
		DurationMs:   duration.Milliseconds(),
	}
}

// QueryResult holds the outcome of a Query call.
type QueryResult struct {
	Columns    []string        `json:"columns"`
	Rows       [][]interface{} `json:"rows"`
	RowCount   int             `json:"row_count"`
	DurationMs int64           `json:"duration_ms"`
	Warning    string          `json:"warning,omitempty"`
	HasMore    bool            `json:"has_more"`
	Error      string          `json:"error,omitempty"`
}

// NewQueryResult creates a QueryResult with the given parameters.
func NewQueryResult(columns []string, rows [][]interface{}, duration time.Duration, warning string) *QueryResult {
	return &QueryResult{
		Columns:    columns,
		Rows:       rows,
		RowCount:   len(rows),
		DurationMs: duration.Milliseconds(),
		Warning:    warning,
	}
}

// StreamRow represents a single row from a streaming query.
type StreamRow struct {
	Row   map[string]interface{} `json:"row"`
	Index int64                  `json:"index"`
	Error string                 `json:"error,omitempty"`
}

// StreamResult is an iterator for streaming query results.
// Callers iterate via Next() and collect rows with Scan().
type StreamResult struct {
	columns []string
	err     error
	closed  bool
	done    chan struct{}
	rows    chan StreamRow
	current StreamRow
	index   int64
	prodDone chan struct{} // closed by producer goroutine when it exits
}

// NewStreamResult creates a new StreamResult with the given channel.
func NewStreamResult(columns []string, rows chan StreamRow) *StreamResult {
	return &StreamResult{
		columns:  columns,
		rows:     rows,
		done:     make(chan struct{}),
		prodDone: make(chan struct{}),
	}
}

// Columns returns the column names for the stream.
func (s *StreamResult) Columns() []string { return s.columns }

// Next advances the iterator to the next row. Returns false when exhausted.
func (s *StreamResult) Next() bool {
	if s.closed {
		return false
	}
	row, ok := <-s.rows
	if !ok {
		s.closed = true
		return false
	}
	s.current = row
	s.index++
	return true
}

// Scan returns the current row data.
func (s *StreamResult) Scan() map[string]interface{} {
	return s.current.Row
}

// Err returns the first error encountered during streaming.
func (s *StreamResult) Err() error {
	if s.err != nil {
		return s.err
	}
	if s.current.Error != "" {
		return &StreamError{Msg: s.current.Error}
	}
	return nil
}

// Close stops the streaming query and releases resources.
// After Close returns, the producer goroutine may still be cleaning up.
// Call Wait() before closing the database connection to ensure the
// goroutine has fully completed.
func (s *StreamResult) Close() error {
	if s.closed {
		return nil
	}
	s.closed = true
	close(s.done)
	return nil
}

// SetColumns sets the column names for the stream (used during initialization).
func (s *StreamResult) SetColumns(cols []string) {
	s.columns = cols
}

// SetError sets an error on the stream result (used by the producer goroutine).
func (s *StreamResult) SetError(err error) {
	s.err = err
}

// Done returns a channel that is closed when the stream is cancelled or completes.
func (s *StreamResult) Done() <-chan struct{} {
	return s.done
}

// Wait blocks until the producer goroutine has fully completed.
// Call this before closing the database connection if the stream
// was stopped early, to ensure all cleanup is done.
func (s *StreamResult) Wait() {
	<-s.prodDone
}

// SetProducerDone signals that the producer goroutine has finished.
// Called by the producer goroutine as its final cleanup step.
func (s *StreamResult) SetProducerDone() {
	close(s.prodDone)
}
type StreamError struct {
	Msg string
}

func (e *StreamError) Error() string {
	return "stream error: " + e.Msg
}
