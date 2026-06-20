package tui

import (
	"encoding/csv"
	"sort"
	"strconv"
	"strings"

	"github.com/jasondostal/winadmin/fleet"
)

// The gather table is the "it's just data" layer: raw per-target command output
// parsed into a columnar form you can sort or group by any column, spreadsheet-
// style. Synthetic columns (TARGET, EXIT, and — when a registry is joined in —
// OS) are always available alongside whatever the query's output parses into.

const (
	colTarget = "TARGET"
	colExit   = "EXIT"
	colOutput = "OUTPUT"
	colOS     = "OS"
)

// gatherRow is one record: named cells. A single target can yield many rows
// (e.g. df lists many filesystems), each tagged with its TARGET cell.
type gatherRow struct {
	cells map[string]string
}

func (r gatherRow) get(col string) string { return r.cells[col] }

// gatherTable is the parsed, columnar form of a gather run.
type gatherTable struct {
	cols []string // column names in display order
	rows []gatherRow
}

// buildTable turns raw per-target results into a columnar table using the parse
// mode. osByTarget (may be nil) injects an OS column from the registry.
func buildTable(results []fleet.Result, parse string, osByTarget map[string]string) gatherTable {
	cols := []string{colTarget}
	seen := map[string]bool{colTarget: true}
	add := func(name string) {
		if name != "" && !seen[name] {
			seen[name] = true
			cols = append(cols, name)
		}
	}
	if osByTarget != nil {
		add(colOS)
	}
	add(colExit)

	var rows []gatherRow
	for _, res := range results {
		base := map[string]string{colTarget: res.Target, colExit: itoa(res.ExitCode)}
		if osByTarget != nil {
			base[colOS] = osByTarget[res.Target]
		}
		recCols, recs := parseRecords(res, parse)
		for _, c := range recCols { // header order — deterministic, not map order
			add(c)
		}
		for _, rec := range recs {
			cells := make(map[string]string, len(base)+len(rec))
			for k, v := range base {
				cells[k] = v
			}
			for k, v := range rec {
				cells[k] = v
			}
			rows = append(rows, gatherRow{cells: cells})
		}
	}
	return gatherTable{cols: cols, rows: rows}
}

// parseRecords extracts the (ordered) column names and zero or more records from
// one target's output per the parse mode. A transport error becomes a single
// OUTPUT cell carrying it.
func parseRecords(res fleet.Result, parse string) ([]string, []map[string]string) {
	if res.Err != nil {
		return []string{colOutput}, []map[string]string{{colOutput: "ERROR: " + res.Err.Error()}}
	}
	switch parse {
	case "kv":
		cols, rec := parseKV(res.Stdout)
		if len(cols) > 0 {
			return cols, []map[string]string{rec}
		}
	case "columns":
		if cols, recs := parseColumns(res.Stdout); len(recs) > 0 {
			return cols, recs
		}
	case "csv":
		if cols, recs := parseCSV(res.Stdout); len(recs) > 0 {
			return cols, recs
		}
	}
	// Default / fallback: the whole output as one OUTPUT cell.
	return []string{colOutput}, []map[string]string{{colOutput: strings.Join(splitLines(res.Stdout), " ")}}
}

// parseKV reads KEY=VALUE / KEY: VALUE lines (os-release, `wmic … /format:list`)
// into a single record, stripping surrounding quotes from values. Returns keys
// in the order they appear.
func parseKV(s string) ([]string, map[string]string) {
	var cols []string
	rec := map[string]string{}
	for _, line := range splitLines(s) {
		i := strings.IndexAny(line, "=:")
		if i <= 0 {
			continue
		}
		k := strings.TrimSpace(line[:i])
		v := strings.Trim(strings.TrimSpace(line[i+1:]), `"'`)
		if k == "" {
			continue
		}
		if _, seen := rec[k]; !seen {
			cols = append(cols, k)
		}
		rec[k] = v
	}
	return cols, rec
}

// parseColumns reads whitespace-delimited output with a header line (df, who,
// `wmic … get`) into one record per data line. Trailing fields beyond the last
// header column are joined into it.
func parseColumns(s string) ([]string, []map[string]string) {
	lines := splitLines(s)
	if len(lines) < 2 {
		return nil, nil
	}
	header := strings.Fields(lines[0])
	if len(header) == 0 {
		return nil, nil
	}
	var recs []map[string]string
	for _, line := range lines[1:] {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		rec := make(map[string]string, len(header))
		for i, h := range header {
			switch {
			case i == len(header)-1 && len(f) > len(header):
				rec[h] = strings.Join(f[i:], " ")
			case i < len(f):
				rec[h] = f[i]
			}
		}
		recs = append(recs, rec)
	}
	return header, recs
}

// parseCSV reads comma-separated output with a header row into one record per
// data row. It handles both bare wmic CSV and PowerShell's `ConvertTo-Csv`
// (every field double-quoted, embedded commas) by parsing with encoding/csv;
// empty header cells (wmic emits a leading comma; the Node column) are dropped.
// splitLines first strips the \r / \r\r that wmic and WinRM leave behind.
func parseCSV(s string) ([]string, []map[string]string) {
	lines := splitLines(s)
	if len(lines) < 2 {
		return nil, nil
	}
	r := csv.NewReader(strings.NewReader(strings.Join(lines, "\n")))
	r.FieldsPerRecord = -1 // tolerate ragged rows
	r.TrimLeadingSpace = true
	rows, err := r.ReadAll()
	if err != nil || len(rows) < 2 {
		return nil, nil
	}
	header := rows[0]
	var cols []string
	for _, h := range header {
		if h != "" {
			cols = append(cols, h)
		}
	}
	var recs []map[string]string
	for _, f := range rows[1:] {
		rec := make(map[string]string, len(header))
		for i, h := range header {
			if h == "" {
				continue
			}
			if i < len(f) {
				rec[h] = f[i]
			}
		}
		if len(rec) > 0 {
			recs = append(recs, rec)
		}
	}
	return cols, recs
}

// sortRows orders rows by a column, numeric-aware (so "20G" > "5G" and exit
// codes sort as numbers, not strings). Stable, so equal keys keep input order.
func sortRows(rows []gatherRow, col string, desc bool) {
	sort.SliceStable(rows, func(i, j int) bool {
		if desc {
			return cellLess(rows[j].get(col), rows[i].get(col))
		}
		return cellLess(rows[i].get(col), rows[j].get(col))
	})
}

func cellLess(a, b string) bool {
	an, aok := parseNum(a)
	bn, bok := parseNum(b)
	if aok && bok && an != bn {
		return an < bn
	}
	if aok != bok {
		return aok // numbers sort before free text
	}
	return strings.ToLower(a) < strings.ToLower(b)
}

// parseNum reads a leading number with an optional human-size suffix (K/M/G/T,
// 1024-based) or a trailing %, so disk/size columns sort by magnitude.
func parseNum(s string) (float64, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, false
	}
	mult := 1.0
	switch s[len(s)-1] {
	case '%':
		s = s[:len(s)-1]
	case 'K', 'k':
		mult, s = 1<<10, s[:len(s)-1]
	case 'M', 'm':
		mult, s = 1<<20, s[:len(s)-1]
	case 'G', 'g':
		mult, s = 1<<30, s[:len(s)-1]
	case 'T', 't':
		mult, s = 1<<40, s[:len(s)-1]
	}
	f, err := strconv.ParseFloat(strings.TrimSpace(s), 64)
	if err != nil {
		return 0, false
	}
	return f * mult, true
}

// group is an ordered bucket of rows sharing a value in the group column.
type group struct {
	key  string
	rows []gatherRow
}

// groupRows buckets rows by a column, returning groups ordered numeric-aware by
// key. Rows within each group keep their (already-sorted) order.
func groupRows(rows []gatherRow, col string) []group {
	idx := map[string]int{}
	var groups []group
	for _, r := range rows {
		k := r.get(col)
		if k == "" {
			k = "—"
		}
		if i, ok := idx[k]; ok {
			groups[i].rows = append(groups[i].rows, r)
			continue
		}
		idx[k] = len(groups)
		groups = append(groups, group{key: k, rows: []gatherRow{r}})
	}
	sort.SliceStable(groups, func(i, j int) bool { return cellLess(groups[i].key, groups[j].key) })
	return groups
}

// filterRows keeps rows where any cell contains the (lowercased) query.
func filterRows(rows []gatherRow, q string) []gatherRow {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return rows
	}
	out := make([]gatherRow, 0, len(rows))
	for _, r := range rows {
		for _, v := range r.cells {
			if strings.Contains(strings.ToLower(v), q) {
				out = append(out, r)
				break
			}
		}
	}
	return out
}
