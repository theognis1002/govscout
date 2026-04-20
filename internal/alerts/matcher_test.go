package alerts

import (
	"reflect"
	"testing"
	"time"

	"github.com/theognis1002/govscout/internal/db"
)

func TestParseKeywords(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"", nil},
		{"cyber", []string{"cyber"}},
		{"cyber, AI ,ml", []string{"cyber", "ai", "ml"}},
		{"  FOO  ", []string{"foo"}},
		// stray commas must not create ghost empty tokens
		{", ,", nil},
		{"a,,b", []string{"a", "b"}},
		// already-lowercase round-trip
		{"data,science", []string{"data", "science"}},
	}
	for _, tc := range tests {
		t.Run(tc.in, func(t *testing.T) {
			got := parseKeywords(tc.in)
			if !reflect.DeepEqual(got, tc.want) {
				t.Errorf("parseKeywords(%q) = %v, want %v", tc.in, got, tc.want)
			}
		})
	}
}

func TestMatchKeywords_MatchAll(t *testing.T) {
	text := "cyber security ai platform"
	tests := []struct {
		name string
		kws  []string
		want bool
	}{
		{"all present", []string{"cyber", "ai"}, true},
		{"one missing rejects (silent-regression guard)", []string{"cyber", "quantum"}, false},
		{"none present", []string{"quantum", "blockchain"}, false},
		{"single present", []string{"platform"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchKeywords(text, tc.kws, true); got != tc.want {
				t.Errorf("matchKeywords(text, %v, matchAll=true) = %v, want %v", tc.kws, got, tc.want)
			}
		})
	}
}

func TestMatchKeywords_MatchAny(t *testing.T) {
	text := "cyber security"
	tests := []struct {
		name string
		kws  []string
		want bool
	}{
		{"one present", []string{"quantum", "cyber"}, true},
		{"none present", []string{"quantum", "blockchain"}, false},
		{"all present", []string{"cyber", "security"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := matchKeywords(text, tc.kws, false); got != tc.want {
				t.Errorf("matchKeywords(text, %v, matchAll=false) = %v, want %v", tc.kws, got, tc.want)
			}
		})
	}
}

func TestMatchKeywords_EmptyKeywords(t *testing.T) {
	// Empty keyword list is a degenerate "no filter" case — must return true
	// in BOTH modes, otherwise saved searches with no include keywords would
	// silently filter out every opportunity.
	if !matchKeywords("whatever", nil, true) {
		t.Error("empty kws + matchAll=true should return true")
	}
	if !matchKeywords("whatever", nil, false) {
		t.Error("empty kws + matchAll=false should return true")
	}
	if !matchKeywords("", nil, true) {
		t.Error("empty text + empty kws should return true")
	}
}

func TestMatchKeywords_IsSubstringNotWordBoundary(t *testing.T) {
	// Current contract: matching uses strings.Contains (substring, not word
	// boundary). Locking this in so a switch to word-boundary matching is a
	// deliberate, test-failing change — not a silent behavior shift.
	if !matchKeywords("cybersecurity", []string{"cyber"}, false) {
		t.Error(`"cybersecurity" should match keyword "cyber" under substring contract`)
	}
	if !matchKeywords("unclassified", []string{"class"}, false) {
		t.Error(`"unclassified" should match keyword "class" under substring contract`)
	}
}

func TestExcludeAny(t *testing.T) {
	tests := []struct {
		name string
		text string
		kws  []string
		want bool
	}{
		{"reject on match", "award announcement", []string{"award"}, true},
		{"pass when no match", "new solicitation", []string{"award", "cancel"}, false},
		{"empty exclude list never rejects", "anything at all", nil, false},
		{"reject if any one of several hits", "request for proposal", []string{"award", "proposal"}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := excludeAny(tc.text, tc.kws); got != tc.want {
				t.Errorf("excludeAny(%q, %v) = %v, want %v", tc.text, tc.kws, got, tc.want)
			}
		})
	}
}

func TestBuildFilters_ResponseDeadline(t *testing.T) {
	// buildFilters currently reads time.Now() directly. We test with a tight
	// tolerance: the "From" date must equal today's MM/DD/YYYY (produced by
	// buildFilters and by the test at nearly the same instant), and "To" must
	// match now.AddDate(...) computed the same way.

	ptr := func(s string) *string { return &s }
	search := db.SavedSearchRow{ResponseDeadline: ptr("3m")}

	before := time.Now()
	f := buildFilters(search, 100)
	after := time.Now()

	if f.ResponseDeadline != "3m" {
		t.Errorf("ResponseDeadline = %q, want 3m", f.ResponseDeadline)
	}
	// From should be "today" in MM/DD/YYYY. Allow either before or after in
	// case the test crosses a day boundary mid-run.
	validFrom := map[string]bool{
		before.Format("01/02/2006"): true,
		after.Format("01/02/2006"):  true,
	}
	if !validFrom[f.ResponseDeadlineFrom] {
		t.Errorf("ResponseDeadlineFrom = %q, want today (%q)", f.ResponseDeadlineFrom, before.Format("01/02/2006"))
	}
	validTo := map[string]bool{
		before.AddDate(0, 3, 0).Format("01/02/2006"): true,
		after.AddDate(0, 3, 0).Format("01/02/2006"):  true,
	}
	if !validTo[f.ResponseDeadlineTo] {
		t.Errorf("ResponseDeadlineTo = %q, want now+3m", f.ResponseDeadlineTo)
	}
}

func TestBuildFilters_ResponseDeadline_Windows(t *testing.T) {
	// Verifies the switch arms: 1m/3m/6m/12m each produce a distinct window
	// offset. Guard: if someone reorders or removes a case, the corresponding
	// window becomes empty and this test catches it.
	ptr := func(s string) *string { return &s }
	cases := []struct {
		code   string
		addY   int
		addM   int
		addD   int
		wantTo func(time.Time) string
	}{
		{"1m", 0, 1, 0, func(n time.Time) string { return n.AddDate(0, 1, 0).Format("01/02/2006") }},
		{"3m", 0, 3, 0, func(n time.Time) string { return n.AddDate(0, 3, 0).Format("01/02/2006") }},
		{"6m", 0, 6, 0, func(n time.Time) string { return n.AddDate(0, 6, 0).Format("01/02/2006") }},
		{"12m", 1, 0, 0, func(n time.Time) string { return n.AddDate(1, 0, 0).Format("01/02/2006") }},
	}
	for _, tc := range cases {
		t.Run(tc.code, func(t *testing.T) {
			before := time.Now()
			f := buildFilters(db.SavedSearchRow{ResponseDeadline: ptr(tc.code)}, 10)
			after := time.Now()

			valid := map[string]bool{tc.wantTo(before): true, tc.wantTo(after): true}
			if !valid[f.ResponseDeadlineTo] {
				t.Errorf("%s: ResponseDeadlineTo = %q, want %q", tc.code, f.ResponseDeadlineTo, tc.wantTo(before))
			}
			if f.ResponseDeadlineFrom == "" {
				t.Errorf("%s: ResponseDeadlineFrom should be set", tc.code)
			}
		})
	}
}

func TestBuildFilters_NoResponseDeadline(t *testing.T) {
	// No deadline set → neither From nor To should be populated.
	f := buildFilters(db.SavedSearchRow{}, 10)
	if f.ResponseDeadlineFrom != "" || f.ResponseDeadlineTo != "" {
		t.Errorf("expected empty deadline range, got From=%q To=%q",
			f.ResponseDeadlineFrom, f.ResponseDeadlineTo)
	}
}

func TestBuildFilters_CopiesScalarFields(t *testing.T) {
	// Guards the dereferencing of *string fields onto ListFilters.
	ptr := func(s string) *string { return &s }
	search := db.SavedSearchRow{
		NAICSCode:  ptr("541511,541512"),
		OppType:    ptr("k"),
		SetAside:   ptr("SBA"),
		State:      ptr("VA"),
		Department: ptr("DOD"),
		ActiveOnly: true,
	}
	f := buildFilters(search, 42)
	if f.NAICSCode != "541511,541512" || f.OppType != "k" || f.SetAside != "SBA" ||
		f.State != "VA" || f.Department != "DOD" || !f.ActiveOnly || f.Limit != 42 {
		t.Errorf("buildFilters copy mismatch: %+v", f)
	}
}
