package tui

import (
	"errors"
	"strings"
	"testing"

	"github.com/jasondostal/winadmin/fleet"
)

// TestBuildTable_StableColumnOrder guards against the map-iteration bug: columns
// must follow header order identically across runs, not Go's randomized map order.
func TestBuildTable_StableColumnOrder(t *testing.T) {
	results := []fleet.Result{{Target: "h1", Stdout: "Mount,Size,Used,Avail,Usepct\n/,20G,5G,15G,25%"}}
	first := strings.Join(buildTable(results, "csv", nil).cols, ",")
	for i := 0; i < 50; i++ {
		if got := strings.Join(buildTable(results, "csv", nil).cols, ","); got != first {
			t.Fatalf("column order not stable: %q vs %q", got, first)
		}
	}
	if want := "TARGET,EXIT,Mount,Size,Used,Avail,Usepct"; first != want {
		t.Errorf("cols = %q, want %q", first, want)
	}
}

func TestParseKV(t *testing.T) {
	cols, rec := parseKV("PRETTY_NAME=\"Ubuntu 24.04 LTS\"\nVERSION_ID=\"24.04\"\nnoise\nID=ubuntu")
	want := []string{"PRETTY_NAME", "VERSION_ID", "ID"}
	if len(cols) != len(want) {
		t.Fatalf("cols = %v, want %v", cols, want)
	}
	for i := range want {
		if cols[i] != want[i] {
			t.Errorf("col[%d] = %q, want %q (order must be stable)", i, cols[i], want[i])
		}
	}
	if rec["PRETTY_NAME"] != "Ubuntu 24.04 LTS" {
		t.Errorf("PRETTY_NAME = %q", rec["PRETTY_NAME"])
	}
	if rec["VERSION_ID"] != "24.04" {
		t.Errorf("VERSION_ID = %q", rec["VERSION_ID"])
	}
	if rec["ID"] != "ubuntu" {
		t.Errorf("ID = %q", rec["ID"])
	}
	if _, ok := rec["noise"]; ok {
		t.Error("a line without a separator should be skipped")
	}
}

func TestParseColumns(t *testing.T) {
	cols, recs := parseColumns("USER TTY LOGIN\nroot pts/0 Jun 20 10:00\njdoe pts/1 Jun 20 11:00")
	if got := strings.Join(cols, ","); got != "USER,TTY,LOGIN" {
		t.Errorf("cols = %q, want USER,TTY,LOGIN", got)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
	if recs[0]["USER"] != "root" || recs[1]["USER"] != "jdoe" {
		t.Errorf("USER cells = %q, %q", recs[0]["USER"], recs[1]["USER"])
	}
	// Trailing fields beyond the last header column fold into it.
	if recs[0]["LOGIN"] != "Jun 20 10:00" {
		t.Errorf("LOGIN = %q", recs[0]["LOGIN"])
	}
}

func TestParseCSV_SkipsEmptyHeaderCells(t *testing.T) {
	// wmic /format:csv shape: leading empty header cell.
	cols, recs := parseCSV(",DeviceID,FreeSpace,Size\nWIN01,C:,1000,2000\nWIN01,D:,500,4000")
	if got := strings.Join(cols, ","); got != "DeviceID,FreeSpace,Size" {
		t.Errorf("cols = %q, want DeviceID,FreeSpace,Size (empty header dropped)", got)
	}
	if len(recs) != 2 {
		t.Fatalf("want 2 records, got %d", len(recs))
	}
	if recs[0]["DeviceID"] != "C:" || recs[0]["FreeSpace"] != "1000" {
		t.Errorf("row0 = %+v", recs[0])
	}
	if _, ok := recs[0][""]; ok {
		t.Error("empty header cell should be dropped")
	}
}

func TestBuildTable_SyntheticAndOSColumns(t *testing.T) {
	results := []fleet.Result{
		{Target: "web01", Stdout: "PRETTY_NAME=\"Ubuntu\"", ExitCode: 0},
		{Target: "db01", Err: errors.New("dial timeout")},
	}
	tbl := buildTable(results, "kv", map[string]string{"web01": "Ubuntu 24.04", "db01": "RHEL 9"})

	// TARGET, OS, EXIT are synthetic and must lead.
	if tbl.cols[0] != colTarget || tbl.cols[1] != colOS || tbl.cols[2] != colExit {
		t.Fatalf("leading cols = %v", tbl.cols[:3])
	}
	if len(tbl.rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(tbl.rows))
	}
	if tbl.rows[0].get(colOS) != "Ubuntu 24.04" {
		t.Errorf("injected OS = %q", tbl.rows[0].get(colOS))
	}
	// The errored target still produces a row, carrying the error in OUTPUT.
	if got := tbl.rows[1].get(colOutput); got == "" {
		t.Error("errored target should carry an OUTPUT cell")
	}
}

func TestBuildTable_NoRegistryOmitsOS(t *testing.T) {
	tbl := buildTable([]fleet.Result{{Target: "h1", Stdout: "x"}}, "", nil)
	for _, c := range tbl.cols {
		if c == colOS {
			t.Fatal("OS column should not appear without a registry join")
		}
	}
}

func TestCellLess_NumericAware(t *testing.T) {
	if !cellLess("5G", "20G") {
		t.Error("5G should sort before 20G numerically")
	}
	if cellLess("20G", "5G") {
		t.Error("20G should not sort before 5G")
	}
	if !cellLess("9", "10") {
		t.Error("9 should sort before 10 (numeric, not lexical)")
	}
	if !cellLess("12", "apple") {
		t.Error("numbers should sort before free text")
	}
}

func TestSortRows_Desc(t *testing.T) {
	rows := []gatherRow{
		{cells: map[string]string{"size": "5G"}},
		{cells: map[string]string{"size": "20G"}},
		{cells: map[string]string{"size": "1G"}},
	}
	sortRows(rows, "size", true)
	if rows[0].get("size") != "20G" || rows[2].get("size") != "1G" {
		t.Errorf("desc order wrong: %v", []string{rows[0].get("size"), rows[1].get("size"), rows[2].get("size")})
	}
}

func TestGroupRows(t *testing.T) {
	rows := []gatherRow{
		{cells: map[string]string{"os": "Ubuntu", "h": "a"}},
		{cells: map[string]string{"os": "RHEL", "h": "b"}},
		{cells: map[string]string{"os": "Ubuntu", "h": "c"}},
		{cells: map[string]string{"os": "", "h": "d"}},
	}
	groups := groupRows(rows, "os")
	if len(groups) != 3 {
		t.Fatalf("want 3 groups, got %d", len(groups))
	}
	// Sorted by key: "RHEL", "Ubuntu", "—" (empty).
	if groups[0].key != "RHEL" || groups[1].key != "Ubuntu" || groups[2].key != "—" {
		t.Errorf("group order = %q, %q, %q", groups[0].key, groups[1].key, groups[2].key)
	}
	if len(groups[1].rows) != 2 {
		t.Errorf("Ubuntu group should have 2 rows, got %d", len(groups[1].rows))
	}
}

func TestHumanizeCell(t *testing.T) {
	cases := []struct{ col, in, want string }{
		{"FreeSpace", "123273396224", "114.8G"},
		{"Size", "135771664384", "126.4G"},
		{"FreeSpace", "", ""},                    // empty (E: CD-ROM) stays empty
		{"Avail", "20G", "20G"},                  // already human → untouched
		{"EXIT", "123273396224", "123273396224"}, // non-byte column → untouched
		{"DeviceID", "C:", "C:"},
	}
	for _, c := range cases {
		if got := humanizeCell(c.col, c.in); got != c.want {
			t.Errorf("humanizeCell(%q,%q) = %q, want %q", c.col, c.in, got, c.want)
		}
	}
}

func TestHumanizeIsDisplayOnly(t *testing.T) {
	// The raw cell (used for sort/filter) must be untouched; only display humanizes.
	results := []fleet.Result{{Target: "win", Stdout: "Node,Size\nwin,135771664384"}}
	tbl := buildTable(results, "csv", nil)
	if got := tbl.rows[0].get("Size"); got != "135771664384" {
		t.Errorf("raw Size cell = %q, want raw bytes (sort key must stay numeric)", got)
	}
}

func TestFilterRows(t *testing.T) {
	rows := []gatherRow{
		{cells: map[string]string{"a": "hello", "b": "world"}},
		{cells: map[string]string{"a": "foo", "b": "bar"}},
	}
	if got := filterRows(rows, "WORLD"); len(got) != 1 {
		t.Errorf("case-insensitive filter want 1, got %d", len(got))
	}
	if got := filterRows(rows, ""); len(got) != 2 {
		t.Errorf("empty filter should keep all, got %d", len(got))
	}
}
