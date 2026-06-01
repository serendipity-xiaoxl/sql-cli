// Package sanitize provides utilities for sanitizing SQL parameters in logs
// to prevent sensitive data (passwords, secrets) from being exposed.
package sanitize

import "strings"

// Params sanitizes SQL query parameters for safe logging.
// Each value is checked; if it looks like a credential or sensitive value,
// it is replaced with "[REDACTED]".
func Params(args []interface{}) []interface{} {
	result := make([]interface{}, len(args))
	for i, arg := range args {
		result[i] = sanitizeValue(arg)
	}
	return result
}

// sanitizeValue checks a single value for sensitive content.
func sanitizeValue(v interface{}) interface{} {
	s, ok := v.(string)
	if !ok {
		return v
	}
	if looksSensitive(s) {
		return "[REDACTED]"
	}
	return v
}

// looksSensitive checks if a string value appears to be sensitive data.
func looksSensitive(s string) bool {
	if len(s) == 0 {
		return false
	}
	// Check for common credential patterns
	truncated := s
	if len(truncated) > 32 {
		truncated = truncated[:32]
	}
	lower := strings.ToLower(truncated)

	sensitivePrefixes := []string{
		"sk-", "sk_",
		"pk-", "pk_",
		"AKIA", "ASIA",
	}
	for _, prefix := range sensitivePrefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}

	// Heuristic: long random-looking strings
	if len(s) > 20 {
		// Check for high entropy — predominantly alphanumeric
		alphaNum := 0
		for _, c := range s {
			if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') {
				alphaNum++
			}
		}
		if alphaNum > len(s)*80/100 {
			return true
		}
	}

	// Common sensitive field names in SQL params
	sensitiveKeywords := []string{
		"password", "secret", "token", "key", "credential",
		"passwd", "pwd", "auth",
	}
	for _, kw := range sensitiveKeywords {
		if strings.Contains(lower, kw) {
			return true
		}
	}

	return false
}

// SQL sanitizes a SQL string for safe logging by masking literal values.
func SQL(sql string) string {
	// Basic replacement: replace string literals with placeholders
	result := sql
	result = replaceQuoted(result, '\'')
	result = replaceQuoted(result, '"')
	return result
}

// replaceQuoted replaces content between matching quote characters.
func replaceQuoted(s string, quote rune) string {
	var sb strings.Builder
	sb.Grow(len(s))
	inQuote := false
	for _, c := range s {
		if c == quote {
			if inQuote {
				sb.WriteRune(c)
				inQuote = false
			} else {
				sb.WriteRune(c)
				inQuote = true
			}
			continue
		}
		if inQuote {
			sb.WriteRune('*')
		} else {
			sb.WriteRune(c)
		}
	}
	return sb.String()
}
