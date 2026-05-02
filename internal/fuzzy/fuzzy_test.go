package fuzzy

import (
	"math"
	"testing"
)

func approx(a, b, epsilon float64) bool {
	return math.Abs(a-b) < epsilon
}

func TestHasDatePrefix(t *testing.T) {
	tests := []struct {
		name  string
		want  bool
	}{
		{"2025-08-17-redis-experiment", true},
		{"2024-01-15-project", true},
		{"not-a-date", false},
		{"redis-test", false},
		{"2025-08-17", false},
		{"2025-08-17-", true},
	}
	for _, tt := range tests {
		got := HasDatePrefix(tt.name)
		if got != tt.want {
			t.Errorf("HasDatePrefix(%q) = %v, want %v", tt.name, got, tt.want)
		}
	}
}

func TestDatePrefixBonus(t *testing.T) {
	if got := DatePrefixBonus("2025-08-17-redis"); !approx(got, 2.0, 0.001) {
		t.Errorf("DatePrefixBonus = %v, want 2.0", got)
	}
	if got := DatePrefixBonus("redis"); !approx(got, 0.0, 0.001) {
		t.Errorf("DatePrefixBonus = %v, want 0.0", got)
	}
}

func TestRecencyBonus(t *testing.T) {
	now := int64(100000)
	tests := []struct {
		mtime int64
		min   float64
		max   float64
	}{
		{now, 2.9, 3.1},         // just now
		{now - 3600, 2.0, 2.2},  // 1 hour ago
		{now - 86400, 0.5, 0.7}, // 24 hours ago
	}
	for _, tt := range tests {
		got := RecencyBonus(int64(now), tt.mtime)
		if got < tt.min || got > tt.max {
			t.Errorf("RecencyBonus(now=%d, mtime=%d) = %v, want in [%v, %v]", now, tt.mtime, got, tt.min, tt.max)
		}
	}
}

func TestMatchEmptyQuery(t *testing.T) {
	entries := []Entry{
		{Name: "2025-08-17-redis", BaseScore: 5.0},
	}
	m := New(entries)
	results := m.Match("")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if !approx(results[0].Score, 5.0, 0.001) {
		t.Errorf("empty query score = %v, want 5.0", results[0].Score)
	}
}

func TestMatchNoMatch(t *testing.T) {
	entries := []Entry{
		{Name: "redis", BaseScore: 0},
	}
	m := New(entries)
	results := m.Match("xyz")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestMatchConsecutiveBonus(t *testing.T) {
	entries := []Entry{
		{Name: "project", BaseScore: 0},
		{Name: "p_xr_oj_e_c_t", BaseScore: 0},
	}
	m := New(entries)
	results := m.Match("pro")

	consecIdx := -1
	gappyIdx := -1
	for i, r := range results {
		if r.Entry.Name == "project" {
			consecIdx = i
		} else {
			gappyIdx = i
		}
	}
	if consecIdx < 0 || gappyIdx < 0 {
		t.Fatalf("missing results: consecIdx=%d gappyIdx=%d", consecIdx, gappyIdx)
	}
	if results[consecIdx].Score <= results[gappyIdx].Score {
		t.Errorf("consecutive score %v should be > gappy score %v", results[consecIdx].Score, results[gappyIdx].Score)
	}
}

func TestMatchWordBoundaryBonus(t *testing.T) {
	entries := []Entry{
		{Name: "redis-server", BaseScore: 0},
	}
	m := New(entries)
	results := m.Match("se")
	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}
	if results[0].Positions == nil || len(results[0].Positions) != 2 {
		t.Fatalf("expected 2 positions, got %v", results[0].Positions)
	}
}

func TestMatchPositions(t *testing.T) {
	entries := []Entry{
		{Name: "2025-08-17-project", BaseScore: 0},
	}
	m := New(entries)
	results := m.Match("pro")

	if len(results) == 0 {
		t.Fatal("expected match, got none")
	}
	if len(results[0].Positions) != 3 {
		t.Fatalf("expected 3 positions, got %v", results[0].Positions)
	}
}

func TestLengthPenalty(t *testing.T) {
	short := []Entry{
		{Name: "abc", BaseScore: 0},
	}
	long := []Entry{
		{Name: "aaaaabbbbbccccc", BaseScore: 0},
	}
	shortResults := New(short).Match("a")
	longResults := New(long).Match("a")
	if len(shortResults) != 1 || len(longResults) != 1 {
		t.Fatal("expected results")
	}
	if shortResults[0].Score <= longResults[0].Score {
		t.Errorf("shorter name should score higher: short=%v long=%v", shortResults[0].Score, longResults[0].Score)
	}
}

func TestDensityBonus(t *testing.T) {
	early := []Entry{
		{Name: "abczzzzzzz", BaseScore: 0},
	}
	late := []Entry{
		{Name: "zzzzzzzabc", BaseScore: 0},
	}
	earlyResults := New(early).Match("abc")
	lateResults := New(late).Match("abc")
	if len(earlyResults) != 1 || len(lateResults) != 1 {
		t.Fatal("expected results")
	}
	if earlyResults[0].Score <= lateResults[0].Score {
		t.Errorf("early match should score higher: early=%v late=%v", earlyResults[0].Score, lateResults[0].Score)
	}
}
