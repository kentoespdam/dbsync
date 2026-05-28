package tui

import (
	"strings"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/lipgloss"
	"github.com/user/dbsync/internal/storage"
)

// newTestModel builds a tablePickerModel with no DB I/O — sufficient for
// layout/measurement tests. loading=false so renderChrome/View emit real chrome.
func newTestModel(itemCount int) tablePickerModel {
	m := newTablePickerModel(storage.Connection{Name: "test"}, nil, nil, "mapping")
	m.loading = false
	items := make([]list.Item, 0, itemCount)
	for i := 0; i < itemCount; i++ {
		items = append(items, tableItem{name: "tbl_" + string(rune('a'+i%26)), isMapped: i%2 == 0, hasPK: true})
	}
	m.allItems = items
	m.list.SetItems(items)
	return m
}

func chromeHeight(m tablePickerModel) int {
	top, bottom := m.renderChrome()
	return lipgloss.Height(top) + lipgloss.Height(bottom)
}

func TestChromeHeight_BaseCase(t *testing.T) {
	m := newTestModel(50)
	got := chromeHeight(m)
	// Expected: title(1) + stats(1) + filter-with-border(3) + help(2) = 7.
	// statsLine NOT active because no filter text and unmappedOnly=false.
	want := 7
	if got != want {
		t.Errorf("chromeHeight base = %d, want %d", got, want)
	}
}

func TestChromeHeight_WithStatsLine_Filter(t *testing.T) {
	m := newTestModel(50)
	m.filterInput.SetValue("foo")
	got := chromeHeight(m)
	// Base 7 + statsLine 1 = 8
	want := 8
	if got != want {
		t.Errorf("chromeHeight with filter = %d, want %d", got, want)
	}
}

func TestChromeHeight_WithStatsLine_UnmappedOnly(t *testing.T) {
	m := newTestModel(50)
	m.unmappedOnly = true
	got := chromeHeight(m)
	want := 8 // base 7 + statsLine 1
	if got != want {
		t.Errorf("chromeHeight unmapped-only = %d, want %d", got, want)
	}
}

func TestStatsLineActive(t *testing.T) {
	tests := []struct {
		name         string
		filterValue  string
		unmappedOnly bool
		want         bool
	}{
		{"clean", "", false, false},
		{"filter only", "x", false, true},
		{"unmapped only", "", true, true},
		{"both", "x", true, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := newTestModel(0)
			m.filterInput.SetValue(tt.filterValue)
			m.unmappedOnly = tt.unmappedOnly
			if got := m.statsLineActive(); got != tt.want {
				t.Errorf("statsLineActive() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestResize_NormalHeight_ListGetsRemaining(t *testing.T) {
	m := newTestModel(50)
	m.resize(80, 30)
	chromeH := chromeHeight(m)
	// resize accounts for chromeH + 2 (blank lines between top/list/bottom).
	wantListH := 30 - chromeH - 2
	if got := m.list.Height(); got != wantListH {
		t.Errorf("list.Height() = %d, want %d (chromeH=%d)", got, wantListH, chromeH)
	}
}

func TestResize_VerySmallHeight_EnforcesMinimum(t *testing.T) {
	m := newTestModel(50)
	m.resize(80, 5) // absurdly small terminal
	if got := m.list.Height(); got < 3 {
		t.Errorf("list.Height() = %d, want >= 3 (minimum)", got)
	}
}

func TestResize_FilterInputWidth(t *testing.T) {
	m := newTestModel(0)
	m.resize(80, 30)
	if got := m.filterInput.Width; got != 76 {
		t.Errorf("filterInput.Width = %d, want 76 (=80-4)", got)
	}
}

// TestView_TotalHeightFitsTerminal is the regression test for the bug
// reported in dbsync-881: when the DB has many tables, the rendered View
// must not exceed terminal height (otherwise the terminal native-scrolls
// and the filter at the top gets pushed off-screen).
func TestView_TotalHeightFitsTerminal(t *testing.T) {
	cases := []struct {
		name   string
		width  int
		height int
		// activate stats line (filter text) — this was the trigger for the
		// original bug: chrome grew by 1 row and total view exceeded height.
		withFilter bool
	}{
		{"normal 80x40", 80, 40, false},
		{"normal 80x40 with filter", 80, 40, true},
		{"small 80x20", 80, 20, false},
		{"small 80x20 with filter", 80, 20, true},
		{"tight 80x15 with filter", 80, 15, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := newTestModel(200)
			if c.withFilter {
				m.filterInput.SetValue("tbl")
				m.applyFilter()
			}
			m.resize(c.width, c.height)
			view := m.View()
			got := lipgloss.Height(view)
			if got > c.height {
				t.Errorf("View() height = %d, exceeds terminal height %d\nfirst lines:\n%s",
					got, c.height, headLines(view, 5))
			}
		})
	}
}

func headLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	if len(lines) > n {
		lines = lines[:n]
	}
	return strings.Join(lines, "\n")
}
