package redact

import (
	"regexp"
)

var quoteRegex = regexp.MustCompile(`'[^']*'|"[^"]*"`)

// Error redacts quoted values inside an error message.
// Safe on nil (returns "").
func Error(err error) string {
	if err == nil {
		return ""
	}
	return quoteRegex.ReplaceAllString(err.Error(), "'[REDACTED]'")
}
