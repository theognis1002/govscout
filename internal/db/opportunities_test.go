package db

import (
	"reflect"
	"strings"
	"testing"
)

func TestAddLikeSearch(t *testing.T) {
	tests := []struct {
		name       string
		search     string
		wantClause string
		wantParams []any
	}{
		{
			name:       "empty search is no-op",
			search:     "",
			wantClause: "",
			wantParams: nil,
		},
		{
			name:       "plain term wraps with wildcards across three columns",
			search:     "cyber",
			wantClause: `(title LIKE ? ESCAPE '\' OR solicitation_number LIKE ? ESCAPE '\' OR department LIKE ? ESCAPE '\')`,
			wantParams: []any{"%cyber%", "%cyber%", "%cyber%"},
		},
		{
			name:       "percent literal is escaped so it matches %, not any-char",
			search:     "50%",
			wantClause: `(title LIKE ? ESCAPE '\' OR solicitation_number LIKE ? ESCAPE '\' OR department LIKE ? ESCAPE '\')`,
			wantParams: []any{`%50\%%`, `%50\%%`, `%50\%%`},
		},
		{
			name:       "underscore literal is escaped so it matches _, not any-single-char",
			search:     "a_b",
			wantClause: `(title LIKE ? ESCAPE '\' OR solicitation_number LIKE ? ESCAPE '\' OR department LIKE ? ESCAPE '\')`,
			wantParams: []any{`%a\_b%`, `%a\_b%`, `%a\_b%`},
		},
		{
			name:       "backslash is escaped first so escape chars stay literal",
			search:     `a\b`,
			wantClause: `(title LIKE ? ESCAPE '\' OR solicitation_number LIKE ? ESCAPE '\' OR department LIKE ? ESCAPE '\')`,
			wantParams: []any{`%a\\b%`, `%a\\b%`, `%a\\b%`},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var qb QueryBuilder
			qb.addLikeSearch(tc.search)
			if tc.wantClause == "" {
				if len(qb.clauses) != 0 {
					t.Fatalf("expected no clauses, got %v", qb.clauses)
				}
				if len(qb.params) != 0 {
					t.Fatalf("expected no params, got %v", qb.params)
				}
				return
			}
			if len(qb.clauses) != 1 || qb.clauses[0] != tc.wantClause {
				t.Errorf("clause = %q, want %q", qb.clauses, tc.wantClause)
			}
			if !reflect.DeepEqual(qb.params, tc.wantParams) {
				t.Errorf("params = %v, want %v", qb.params, tc.wantParams)
			}
		})
	}
}

func TestAddIn(t *testing.T) {
	tests := []struct {
		name       string
		column     string
		csv        string
		wantClause string
		wantParams []any
	}{
		{
			name:       "empty string is no-op",
			csv:        "",
			wantClause: "",
			wantParams: nil,
		},
		{
			name:       "single value produces IN (?)",
			column:     "naics_code",
			csv:        "541511",
			wantClause: "naics_code IN (?)",
			wantParams: []any{"541511"},
		},
		{
			name:       "multiple values produce N placeholders in order",
			column:     "naics_code",
			csv:        "541511,541512,541519",
			wantClause: "naics_code IN (?,?,?)",
			wantParams: []any{"541511", "541512", "541519"},
		},
		{
			name:       "whitespace is trimmed and empty tokens dropped",
			column:     "opp_type",
			csv:        " a , , b ,",
			wantClause: "opp_type IN (?,?)",
			wantParams: []any{"a", "b"},
		},
		{
			name:       "all-empty tokens produce no clause",
			column:     "opp_type",
			csv:        ", ,, ",
			wantClause: "",
			wantParams: nil,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var qb QueryBuilder
			qb.addIn(tc.column, tc.csv)
			if tc.wantClause == "" {
				if len(qb.clauses) != 0 {
					t.Fatalf("expected no clauses, got %v", qb.clauses)
				}
				if len(qb.params) != 0 {
					t.Fatalf("expected no params, got %v", qb.params)
				}
				return
			}
			if len(qb.clauses) != 1 || qb.clauses[0] != tc.wantClause {
				t.Errorf("clause = %q, want %q", qb.clauses, tc.wantClause)
			}
			if !reflect.DeepEqual(qb.params, tc.wantParams) {
				t.Errorf("params = %v, want %v", qb.params, tc.wantParams)
			}
		})
	}
}

func TestAddDateGteLte(t *testing.T) {
	t.Run("gte empty is no-op", func(t *testing.T) {
		var qb QueryBuilder
		qb.addDateGte("posted_date", "")
		if len(qb.clauses) != 0 || len(qb.params) != 0 {
			t.Fatalf("expected no-op, got clauses=%v params=%v", qb.clauses, qb.params)
		}
	})

	t.Run("gte converts MM/DD/YYYY to sortable YYYYMMDD", func(t *testing.T) {
		var qb QueryBuilder
		qb.addDateGte("posted_date", "01/31/2026")
		wantClause := "substr(posted_date,7,4)||substr(posted_date,1,2)||substr(posted_date,4,2) >= ?"
		if len(qb.clauses) != 1 || qb.clauses[0] != wantClause {
			t.Errorf("clause = %q, want %q", qb.clauses, wantClause)
		}
		if !reflect.DeepEqual(qb.params, []any{"20260131"}) {
			t.Errorf("params = %v, want [20260131]", qb.params)
		}
	})

	t.Run("lte uses <= operator with same sortable form", func(t *testing.T) {
		var qb QueryBuilder
		qb.addDateLte("response_deadline", "12/25/2025")
		wantClause := "substr(response_deadline,7,4)||substr(response_deadline,1,2)||substr(response_deadline,4,2) <= ?"
		if qb.clauses[0] != wantClause {
			t.Errorf("clause = %q, want %q", qb.clauses[0], wantClause)
		}
		if !reflect.DeepEqual(qb.params, []any{"20251225"}) {
			t.Errorf("params = %v, want [20251225]", qb.params)
		}
	})
}

func TestMmddyyyyToYyyymmdd(t *testing.T) {
	tests := []struct {
		in, want string
	}{
		{"01/31/2026", "20260131"},
		{"12/25/2025", "20251225"},
		{"06/07/2000", "20000607"},
		// Invalid input passes through unchanged — callers treat it as already-sortable.
		{"2026-01-31", "2026-01-31"},
		{"", ""},
		{"not-a-date", "not-a-date"},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := mmddyyyyToYyyymmdd(tc.in)
			if got != tc.want {
				t.Errorf("mmddyyyyToYyyymmdd(%q) = %q, want %q", tc.in, got, tc.want)
			}
		})
	}
}

func TestSplitCSV(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"a", []string{"a"}},
		{"a,b,c", []string{"a", "b", "c"}},
		{" a , b , c ", []string{"a", "b", "c"}},
		{",,", nil},
		{"a,,b", []string{"a", "b"}},
		{"  ,  ", nil},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := splitCSV(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("splitCSV(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestWhereSQL_Composition(t *testing.T) {
	// Verifies that multiple clauses compose with " AND " into a single WHERE.
	// This is the composition contract the whole query layer depends on.
	var qb QueryBuilder
	qb.addIn("naics_code", "541511,541512")
	qb.addIn("opp_type", "k")
	qb.addLiteral("active = 1")

	got := qb.whereSQL()
	want := "WHERE naics_code IN (?,?) AND opp_type IN (?) AND active = 1"
	if got != want {
		t.Errorf("whereSQL = %q, want %q", got, want)
	}
	if !reflect.DeepEqual(qb.params, []any{"541511", "541512", "k"}) {
		t.Errorf("params = %v, want [541511 541512 k]", qb.params)
	}

	// Count placeholder slots match params slot count.
	if strings.Count(got, "?") != len(qb.params) {
		t.Errorf("placeholder count %d != param count %d in %q",
			strings.Count(got, "?"), len(qb.params), got)
	}
}

func TestWhereSQL_Empty(t *testing.T) {
	var qb QueryBuilder
	if got := qb.whereSQL(); got != "" {
		t.Errorf("whereSQL on empty builder = %q, want empty", got)
	}
}
