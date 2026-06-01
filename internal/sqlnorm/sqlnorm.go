// Package sqlnorm provides SQL normalization and safety analysis utilities.
package sqlnorm

import "strings"

// Operation returns the first SQL operation keyword (SELECT, INSERT, etc.).
func Operation(sql string) string {
	s := strings.TrimSpace(sql)
	if len(s) == 0 {
		return ""
	}
	idx := strings.IndexAny(s, " \t\n\r(")
	if idx < 0 {
		return strings.ToUpper(s)
	}
	return strings.ToUpper(s[:idx])
}

// IsSELECT checks if the operation is a read-only query.
func IsSELECT(op string) bool {
	return op == "SELECT" || op == "WITH"
}

// HasWHERE checks if a SQL statement contains a WHERE clause.
// It strips string literals before checking to avoid false positives.
func HasWHERE(sql string) bool {
	cleaned := stripQuoted(sql)
	return matchKeyword(cleaned, "WHERE")
}

// HasLIMIT checks if a SQL statement already contains a LIMIT clause.
func HasLIMIT(sql string) bool {
	cleaned := stripQuoted(sql)
	return matchKeyword(cleaned, "LIMIT")
}

// HasOFFSET checks if a SQL statement already contains an OFFSET clause.
func HasOFFSET(sql string) bool {
	cleaned := stripQuoted(sql)
	return matchKeyword(cleaned, "OFFSET")
}

// AppendOFFSET appends an OFFSET clause to a SQL statement, before any trailing
// semicolon. Should be called after AppendLIMIT so the result is
// "... LIMIT n OFFSET m".
func AppendOFFSET(sql string, offset int) string {
	s := strings.TrimSpace(sql)
	hasSemi := strings.HasSuffix(s, ";")
	if hasSemi {
		s = s[:len(s)-1]
	}
	return s + " OFFSET " + itoa(offset) + suffix(hasSemi)
}

// AppendLIMIT appends a LIMIT clause to a SQL statement, before any trailing
// semicolon. Returns the original SQL if it already has a LIMIT clause.
func AppendLIMIT(sql string, limit int) string {
	s := strings.TrimSpace(sql)
	hasSemi := strings.HasSuffix(s, ";")
	if hasSemi {
		s = s[:len(s)-1]
	}
	return s + " LIMIT " + itoa(limit) + suffix(hasSemi)
}

func suffix(hasSemi bool) string {
	if hasSemi {
		return ";"
	}
	return ""
}

// itoa is a simple int to string conversion (avoiding strconv import).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var buf [12]byte
	neg := false
	if n < 0 {
		neg = true
		n = -n
	}
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// stripQuoted replaces content inside single and double quotes with spaces.
func stripQuoted(s string) string {
	var buf strings.Builder
	buf.Grow(len(s))
	inSingle := false
	inDouble := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inSingle {
			if c == '\'' {
				inSingle = false
			}
			buf.WriteByte(' ')
			continue
		}
		if inDouble {
			if c == '"' {
				inDouble = false
			}
			buf.WriteByte(' ')
			continue
		}
		if c == '\'' {
			inSingle = true
			buf.WriteByte(' ')
			continue
		}
		if c == '"' {
			inDouble = true
			buf.WriteByte(' ')
			continue
		}
		buf.WriteByte(c)
	}
	return buf.String()
}

// matchKeyword checks if a SQL string contains the given keyword as a standalone word.
func matchKeyword(sql, keyword string) bool {
	upper := strings.ToUpper(sql)
	idx := strings.Index(upper, keyword)
	for idx >= 0 {
		if idx == 0 || isDelimiter(upper[idx-1]) {
			end := idx + len(keyword)
			if end >= len(upper) || isDelimiter(upper[end]) {
				return true
			}
		}
		next := idx + 1
		if next >= len(upper) {
			break
		}
		idx = strings.Index(upper[next:], keyword)
		if idx >= 0 {
			idx += next
		}
	}
	return false
}

// isDelimiter checks if a byte is a word delimiter in SQL.
func isDelimiter(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' ||
		c == '(' || c == ')' || c == ',' || c == ';' ||
		c == '=' || c == '<' || c == '>' || c == '!' ||
		c == '+' || c == '-' || c == '*' || c == '/' ||
		c == '\'' || c == '"'
}

// RequiresWHERE checks if the operation requires a WHERE clause.
func RequiresWHERE(op string) bool {
	return op == "UPDATE" || op == "DELETE"
}
