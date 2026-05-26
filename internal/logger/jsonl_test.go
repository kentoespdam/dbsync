package logger

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSanitizeError(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{
			"Duplicate entry 'john@example.com' for key 'users.email'",
			"Duplicate entry '[REDACTED]' for key '[REDACTED]'",
		},
		{
			"near '...': syntax error",
			"near '[REDACTED]': syntax error",
		},
		{
			"normal error without quotes",
			"normal error without quotes",
		},
	}

	for _, tt := range tests {
		assert.Equal(t, tt.expected, SanitizeError(errors.New(tt.input)))
	}
}

func TestLogger(t *testing.T) {
	l, err := New("testconn", "testtable")
	assert.NoError(t, err)
	defer l.Close()
	defer os.Remove(l.file.Name())

	l.RowError(1, 101, errors.New("Duplicate entry 'val'"), "INSERT INTO...")
	l.BatchError(1, errors.New("connection lost"), "SELECT...")

	content, err := os.ReadFile(l.file.Name())
	assert.NoError(t, err)

	lines := strings.Split(strings.TrimSpace(string(content)), "\n")
	assert.Equal(t, 2, len(lines))

	var e1 Entry
	err = json.Unmarshal([]byte(lines[0]), &e1)
	assert.NoError(t, err)
	assert.Equal(t, "row_error", e1.Level)
	assert.Equal(t, "Duplicate entry '[REDACTED]'", e1.Error)
	assert.Equal(t, float64(101), e1.RowPK)

	var e2 Entry
	err = json.Unmarshal([]byte(lines[1]), &e2)
	assert.NoError(t, err)
	assert.Equal(t, "batch_error", e2.Level)
	assert.Equal(t, "connection lost", e2.Error)
}
