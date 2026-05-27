package redact

import (
	"errors"
	"testing"
)

func TestError(t *testing.T) {
	tests := []struct {
		name string
		err  error
		want string
	}{
		{"nil", nil, ""},
		{"no quotes", errors.New("something went wrong"), "something went wrong"},
		{"single quotes", errors.New("Duplicate entry 'john@x.com' for key 'users.email'"), "Duplicate entry '[REDACTED]' for key '[REDACTED]'"},
		{"double quotes", errors.New(`Table "invoices" not found`), "Table '[REDACTED]' not found"},
		{"mixed quotes", errors.New(`near "foo": syntax error at 'bar'`), "near '[REDACTED]': syntax error at '[REDACTED]'"},
		{"empty quotes", errors.New("error at ''"), "error at '[REDACTED]'"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Error(tt.err); got != tt.want {
				t.Errorf("Error() = %q, want %q", got, tt.want)
			}
		})
	}
}
