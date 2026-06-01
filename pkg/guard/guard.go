// Package guard provides dangerous operation policy enforcement.
package guard

import "errors"

// Policy defines how dangerous operations are handled.
type Policy int

const (
	// PolicyBlock dangerous operations with an error (default).
	PolicyBlock Policy = iota
	// PolicyWarn and allow dangerous operations.
	PolicyWarn
	// PolicyAllow dangerous operations without warning.
	PolicyAllow
)

// String returns the string representation of the policy.
func (p Policy) String() string {
	switch p {
	case PolicyBlock:
		return "block"
	case PolicyWarn:
		return "warn"
	case PolicyAllow:
		return "allow"
	default:
		return "unknown"
	}
}

// ErrDangerousOp is returned when a dangerous operation is blocked.
var ErrDangerousOp = errors.New("dangerous operation blocked")

// DangerousOps lists SQL operations considered dangerous by default.
var DangerousOps = []string{
	"DROP",
	"TRUNCATE",
}

// IsDangerousOp checks if the given SQL statement is a dangerous operation.
// It performs a simple prefix match against the dangerous ops list.
func IsDangerousOp(sql string) bool {
	for _, op := range DangerousOps {
		if matchesOp(sql, op) {
			return true
		}
	}
	return false
}

// matchesOp checks if sql starts with op (case-insensitive, word-boundary).
func matchesOp(sql, op string) bool {
	if len(sql) < len(op) {
		return false
	}
	for i := 0; i < len(op); i++ {
		c := sql[i]
		if c >= 'a' && c <= 'z' {
			c -= 32
		}
		if c != op[i] {
			return false
		}
	}
	if len(sql) > len(op) {
		next := sql[len(op)]
		return next == ' ' || next == '\t' || next == '(' || next == '\n'
	}
	return true
}

// Check evaluates the policy for the given SQL statement.
// Returns ErrDangerousOp if blocked, nil if allowed or warned.
func Check(policy Policy, sql string) error {
	if policy == PolicyAllow {
		return nil
	}
	if !IsDangerousOp(sql) {
		return nil
	}
	if policy == PolicyBlock {
		return ErrDangerousOp
	}
	return nil
}
