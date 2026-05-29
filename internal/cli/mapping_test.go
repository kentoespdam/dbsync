package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

// bd-13c
func TestParseValueMapShorthand(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    map[string]string
		wantErr bool
	}{
		{
			name:    "Single valid pair",
			input:   "Draft=DRAFT",
			want:    map[string]string{"Draft": "DRAFT"},
			wantErr: false,
		},
		{
			name:    "Multiple valid pairs",
			input:   "Draft=DRAFT,Ditampilkan=PUBLISHED",
			want:    map[string]string{"Draft": "DRAFT", "Ditampilkan": "PUBLISHED"},
			wantErr: false,
		},
		{
			name:    "Whitespace trimming",
			input:   "  Draft  =   DRAFT  ,  Ditampilkan  =   PUBLISHED  ",
			want:    map[string]string{"Draft": "DRAFT", "Ditampilkan": "PUBLISHED"},
			wantErr: false,
		},
		{
			name:    "Value containing '='",
			input:   "Draft==DRAFT",
			want:    map[string]string{"Draft": "=DRAFT"},
			wantErr: false,
		},
		{
			name:    "Value containing multiple '=' and spaces",
			input:   "Draft =  =DRAFT=123",
			want:    map[string]string{"Draft": "=DRAFT=123"},
			wantErr: false,
		},
		{
			name:    "Empty shorthand error",
			input:   "   ",
			wantErr: true,
		},
		{
			name:    "Missing '=' error",
			input:   "DraftDRAFT",
			wantErr: true,
		},
		{
			name:    "Empty key error",
			input:   "=DRAFT",
			wantErr: true,
		},
		{
			name:    "Empty value error",
			input:   "Draft=",
			wantErr: true,
		},
		{
			name:    "Empty key with whitespace error",
			input:   "   =DRAFT",
			wantErr: true,
		},
		{
			name:    "Empty value with whitespace error",
			input:   "Draft=   ",
			wantErr: true,
		},
		{
			name:    "Duplicate key error",
			input:   "Draft=DRAFT,Draft=PUBLISHED",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseValueMapShorthand(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseValueMapShorthand() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseValueMapShorthand() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// bd-13c
func TestParseValueMapFile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "dbsync-test")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// 1. Valid JSON file
	validPath := filepath.Join(tmpDir, "valid.json")
	validData := `{"Draft": "DRAFT", "Ditampilkan": "PUBLISHED"}`
	if err := os.WriteFile(validPath, []byte(validData), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// 2. Invalid JSON file
	invalidPath := filepath.Join(tmpDir, "invalid.json")
	invalidData := `{invalid json}`
	if err := os.WriteFile(invalidPath, []byte(invalidData), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// 3. Empty key JSON file
	emptyKeyPath := filepath.Join(tmpDir, "empty_key.json")
	emptyKeyData := `{"": "DRAFT"}`
	if err := os.WriteFile(emptyKeyPath, []byte(emptyKeyData), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	// 4. Empty value JSON file
	emptyValPath := filepath.Join(tmpDir, "empty_val.json")
	emptyValData := `{"Draft": "  "}`
	if err := os.WriteFile(emptyValPath, []byte(emptyValData), 0644); err != nil {
		t.Fatalf("failed to write temp file: %v", err)
	}

	tests := []struct {
		name    string
		path    string
		want    map[string]string
		wantErr bool
	}{
		{
			name:    "Valid JSON file",
			path:    validPath,
			want:    map[string]string{"Draft": "DRAFT", "Ditampilkan": "PUBLISHED"},
			wantErr: false,
		},
		{
			name:    "Invalid JSON file error",
			path:    invalidPath,
			wantErr: true,
		},
		{
			name:    "Non-existent file error",
			path:    filepath.Join(tmpDir, "missing.json"),
			wantErr: true,
		},
		{
			name:    "Empty key JSON file error",
			path:    emptyKeyPath,
			wantErr: true,
		},
		{
			name:    "Empty value JSON file error",
			path:    emptyValPath,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseValueMapFile(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseValueMapFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseValueMapFile() got = %v, want %v", got, tt.want)
			}
		})
	}
}

// bd-13c
func TestDeterministicJSON(t *testing.T) {
	vmap := map[string]string{
		"z": "last",
		"a": "first",
		"m": "middle",
	}

	data, err := json.Marshal(vmap)
	if err != nil {
		t.Fatalf("failed to marshal map: %v", err)
	}

	got := string(data)
	want := `{"a":"first","m":"middle","z":"last"}`
	if got != want {
		t.Errorf("json.Marshal() did not sort keys canonically: got %q, want %q", got, want)
	}
}
