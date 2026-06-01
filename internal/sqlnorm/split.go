package sqlnorm

import "strings"

// SplitStatements splits SQL text into individual statements, splitting on
// semicolons while respecting string literals, identifiers, and comments.
// Empty statements and whitespace-only fragments are omitted.
func SplitStatements(sql string) []string {
	var statements []string
	var current strings.Builder
	inSingle := false
	inDouble := false
	inBacktick := false
	inLineComment := false
	inBlockComment := false

	for i := 0; i < len(sql); i++ {
		c := sql[i]

		if inLineComment {
			if c == '\n' {
				inLineComment = false
			}
			continue
		}

		if inBlockComment {
			if c == '*' && i+1 < len(sql) && sql[i+1] == '/' {
				i++ // skip '/'
				inBlockComment = false
			}
			continue
		}

		if inSingle {
			if c == '\'' && i+1 < len(sql) && sql[i+1] == '\'' {
				// escaped single quote in string
				current.WriteByte(c)
				i++
				current.WriteByte(sql[i])
				continue
			}
			current.WriteByte(c)
			if c == '\'' {
				inSingle = false
			}
			continue
		}

		if inDouble {
			if c == '"' && i+1 < len(sql) && sql[i+1] == '"' {
				current.WriteByte(c)
				i++
				current.WriteByte(sql[i])
				continue
			}
			current.WriteByte(c)
			if c == '"' {
				inDouble = false
			}
			continue
		}

		if inBacktick {
			if c == '`' && i+1 < len(sql) && sql[i+1] == '`' {
				current.WriteByte(c)
				i++
				current.WriteByte(sql[i])
				continue
			}
			current.WriteByte(c)
			if c == '`' {
				inBacktick = false
			}
			continue
		}

		switch {
		case c == '-' && i+1 < len(sql) && sql[i+1] == '-':
			inLineComment = true
		case c == '/' && i+1 < len(sql) && sql[i+1] == '*':
			inBlockComment = true
			i++ // skip '*'
		case c == '\'':
			inSingle = true
			current.WriteByte(c)
		case c == '"':
			inDouble = true
			current.WriteByte(c)
		case c == '`':
			inBacktick = true
			current.WriteByte(c)
		case c == ';':
			stmt := strings.TrimSpace(current.String())
			current.Reset()
			if stmt != "" {
				statements = append(statements, stmt)
			}
		default:
			current.WriteByte(c)
		}
	}

	// Remaining text after the last semicolon (if any)
	stmt := strings.TrimSpace(current.String())
	if stmt != "" {
		statements = append(statements, stmt)
	}

	return statements
}
