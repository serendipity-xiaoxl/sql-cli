package result

import (
	"encoding/json"
	"testing"
	"time"
)

func TestNewExecResult(t *testing.T) {
	r := NewExecResult(1, 5, 100*time.Millisecond)

	if r.LastInsertID != 1 {
		t.Errorf("LastInsertID = %d, want 1", r.LastInsertID)
	}
	if r.RowsAffected != 5 {
		t.Errorf("RowsAffected = %d, want 5", r.RowsAffected)
	}
	if r.DurationMs != 100 {
		t.Errorf("DurationMs = %d, want 100", r.DurationMs)
	}
}

func TestExecResultJSON(t *testing.T) {
	r := NewExecResult(42, 7, 200*time.Millisecond)
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ExecResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.LastInsertID != 42 {
		t.Errorf("LastInsertID = %d, want 42", decoded.LastInsertID)
	}
	if decoded.RowsAffected != 7 {
		t.Errorf("RowsAffected = %d, want 7", decoded.RowsAffected)
	}
	if decoded.DurationMs != 200 {
		t.Errorf("DurationMs = %d, want 200", decoded.DurationMs)
	}
}

func TestExecResultJSONEmpty(t *testing.T) {
	r := &ExecResult{}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded ExecResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.LastInsertID != 0 {
		t.Errorf("LastInsertID = %d, want 0 (zero)", decoded.LastInsertID)
	}
	if decoded.RowsAffected != 0 {
		t.Errorf("RowsAffected = %d, want 0", decoded.RowsAffected)
	}
	if decoded.Error != "" {
		t.Errorf("Error = %q, want empty", decoded.Error)
	}
}

func TestExecResultJSONOmitLastInsertID(t *testing.T) {
	r := &ExecResult{RowsAffected: 3, DurationMs: 50}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	// last_insert_id should be omitted when zero (omitempty)
	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if _, exists := raw["last_insert_id"]; exists {
		t.Errorf("expected last_insert_id to be omitted, got %v", raw["last_insert_id"])
	}
}

func TestNewQueryResult(t *testing.T) {
	columns := []string{"id", "name"}
	rows := [][]interface{}{
		{1, "Alice"},
		{2, "Bob"},
	}
	r := NewQueryResult(columns, rows, 150*time.Millisecond, "LIMIT 100 applied automatically")

	if len(r.Columns) != 2 {
		t.Errorf("len(Columns) = %d, want 2", len(r.Columns))
	}
	if r.RowCount != 2 {
		t.Errorf("RowCount = %d, want 2", r.RowCount)
	}
	if r.DurationMs != 150 {
		t.Errorf("DurationMs = %d, want 150", r.DurationMs)
	}
	if r.Warning != "LIMIT 100 applied automatically" {
		t.Errorf("Warning = %q, want %q", r.Warning, "LIMIT 100 applied automatically")
	}
}

func TestQueryResultJSONRoundTrip(t *testing.T) {
	columns := []string{"id", "name", "email"}
	rows := [][]interface{}{
		{1, "Alice", "alice@example.com"},
		{2, "Bob", "bob@example.com"},
	}
	r := NewQueryResult(columns, rows, 50*time.Millisecond, "LIMIT 100 applied")
	r.HasMore = true

	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var decoded QueryResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}

	if decoded.RowCount != 2 {
		t.Errorf("RowCount = %d, want 2", decoded.RowCount)
	}
	if decoded.DurationMs != 50 {
		t.Errorf("DurationMs = %d, want 50", decoded.DurationMs)
	}
	if !decoded.HasMore {
		t.Errorf("HasMore = false, want true")
	}
	if decoded.Warning != "LIMIT 100 applied" {
		t.Errorf("Warning = %q, want %q", decoded.Warning, "LIMIT 100 applied")
	}
}

func TestQueryResultJSONWarningOmitted(t *testing.T) {
	r := NewQueryResult([]string{"id"}, [][]interface{}{{1}}, time.Millisecond, "")
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if _, exists := raw["warning"]; exists {
		t.Errorf("expected warning to be omitted when empty, got %v", raw["warning"])
	}
}

func TestQueryResultJSONErrorOmitted(t *testing.T) {
	r := &QueryResult{Columns: []string{"id"}, Rows: [][]interface{}{{1}}}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("Marshal error: %v", err)
	}

	var raw map[string]interface{}
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatalf("Unmarshal error: %v", err)
	}
	if _, exists := raw["error"]; exists {
		t.Errorf("expected error to be omitted when empty, got %v", raw["error"])
	}
}

func TestStreamResultLifecycle(t *testing.T) {
	columns := []string{"id", "val"}
	rows := make(chan StreamRow, 3)
	sr := NewStreamResult(columns, rows)

	// Send rows
	rows <- StreamRow{Row: map[string]interface{}{"id": 1, "val": "a"}, Index: 0}
	rows <- StreamRow{Row: map[string]interface{}{"id": 2, "val": "b"}, Index: 1}
	close(rows)

	// Read all
	var results []map[string]interface{}
	for sr.Next() {
		results = append(results, sr.Scan())
	}
	if sr.Err() != nil {
		t.Errorf("Err() = %v, want nil", sr.Err())
	}
	if len(results) != 2 {
		t.Errorf("got %d rows, want 2", len(results))
	}
	if results[0]["id"] != 1 || results[1]["val"] != "b" {
		t.Errorf("unexpected row data: %v", results)
	}
}

func TestStreamResultClose(t *testing.T) {
	columns := []string{"id"}
	rows := make(chan StreamRow, 1)
	sr := NewStreamResult(columns, rows)

	rows <- StreamRow{Row: map[string]interface{}{"id": 1}, Index: 0}

	// Read one row
	sr.Next()
	_ = sr.Scan()

	// Close the stream
	sr.Close()

	if sr.Next() {
		t.Errorf("expected Next() to return false after close")
	}
	// Close is idempotent
	if err := sr.Close(); err != nil {
		t.Errorf("Close() error = %v, want nil (idempotent)", err)
	}
}

func TestStreamResultColumns(t *testing.T) {
	sr := NewStreamResult([]string{"id", "name"}, make(chan StreamRow, 1))
	cols := sr.Columns()
	if len(cols) != 2 || cols[0] != "id" || cols[1] != "name" {
		t.Errorf("Columns() = %v, want [id name]", cols)
	}
}

func TestStreamResultSetColumns(t *testing.T) {
	sr := NewStreamResult(nil, make(chan StreamRow, 1))
	sr.SetColumns([]string{"a", "b"})
	cols := sr.Columns()
	if len(cols) != 2 || cols[0] != "a" || cols[1] != "b" {
		t.Errorf("after SetColumns, Columns() = %v, want [a b]", cols)
	}
}

func TestStreamResultError(t *testing.T) {
	sr := NewStreamResult(nil, make(chan StreamRow, 1))
	sr.SetError(assertionErr("test error"))
	if sr.Err() == nil || sr.Err().Error() != "test error" {
		t.Errorf("Err() = %v, want 'test error'", sr.Err())
	}
}

func TestStreamResultNoError(t *testing.T) {
	sr := NewStreamResult(nil, make(chan StreamRow, 1))
	close(sr.rows)
	for sr.Next() {
	}
	if sr.Err() != nil {
		t.Errorf("Err() = %v, want nil", sr.Err())
	}
}

func TestStreamResultDone(t *testing.T) {
	sr := NewStreamResult(nil, make(chan StreamRow, 1))
	done := sr.Done()
	if done == nil {
		t.Fatal("Done() returned nil")
	}
	// Close should trigger the done channel
	sr.Close()
	select {
	case <-done:
	default:
		t.Error("Done() channel not closed after Close()")
	}
}

func TestStreamErrorError(t *testing.T) {
	e := &StreamError{Msg: "stream failed"}
	if e.Error() != "stream error: stream failed" {
		t.Errorf("Error() = %q, want 'stream error: stream failed'", e.Error())
	}
}

func TestErrChecksCurrentRowError(t *testing.T) {
	rows := make(chan StreamRow, 1)
	sr := NewStreamResult(nil, rows)
	rows <- StreamRow{Error: "row level error"}
	close(rows)

	sr.Next()
	if sr.Err() == nil || sr.Err().Error() != "stream error: row level error" {
		t.Errorf("Err() = %v, want 'stream error: row level error'", sr.Err())
	}
}

type assertionErr string

func (e assertionErr) Error() string { return string(e) }

func TestStreamResultSetProducerDoneAndWait(t *testing.T) {
	sr := NewStreamResult(nil, make(chan StreamRow, 1))
	
	// Start a goroutine that will call SetProducerDone when done
	go func() {
		sr.SetProducerDone()
	}()
	
	// Wait should unblock when SetProducerDone is called
	sr.Wait()
}

func TestStreamResultWaitAlreadyDone(t *testing.T) {
	sr := NewStreamResult(nil, make(chan StreamRow, 1))
	sr.SetProducerDone()
	// Second Wait should unblock immediately (already done)
	sr.Wait()
}
