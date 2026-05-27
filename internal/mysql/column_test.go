package mysql

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestColumn_IsBool(t *testing.T) {
	tests := []struct {
		columnType string
		want       bool
	}{
		{"tinyint(1)", true},
		{"TINYINT(1)", true},
		{"tinyint(4)", false},
		{"int", false},
		{"varchar(255)", false},
	}
	for _, tt := range tests {
		t.Run(tt.columnType, func(t *testing.T) {
			c := Column{ColumnType: tt.columnType}
			assert.Equal(t, tt.want, c.IsBool())
		})
	}
}

func TestColumn_EnumValues(t *testing.T) {
	tests := []struct {
		columnType string
		want       []string
	}{
		{"enum('a','b','c')", []string{"a", "b", "c"}},
		{"ENUM('X','Y')", []string{"X", "Y"}},
		{"enum('active','inactive','deleted')", []string{"active", "inactive", "deleted"}},
		{"varchar(255)", nil},
		{"int", nil},
	}
	for _, tt := range tests {
		t.Run(tt.columnType, func(t *testing.T) {
			c := Column{ColumnType: tt.columnType}
			assert.Equal(t, tt.want, c.EnumValues())
		})
	}
}
