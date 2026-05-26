package mysql

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRedactError(t *testing.T) {
	password := "secret123"
	err := fmt.Errorf("could not connect to user:%s@tcp(localhost:3306)", password)
	redacted := redactError(err, password)
	assert.Contains(t, redacted.Error(), "***")
	assert.NotContains(t, redacted.Error(), password)

	err2 := fmt.Errorf("other error")
	assert.Equal(t, err2, redactError(err2, password))
}
