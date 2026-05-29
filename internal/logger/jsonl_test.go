package logger

import (
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
)

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
	assert.Equal(t, "Duplicate entry 'val'", e1.Error)
	assert.Equal(t, float64(101), e1.RowPK)

	var e2 Entry
	err = json.Unmarshal([]byte(lines[1]), &e2)
	assert.NoError(t, err)
	assert.Equal(t, "batch_error", e2.Level)
	assert.Equal(t, "connection lost", e2.Error)
}
