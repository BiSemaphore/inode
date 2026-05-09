package core

import (
	"testing"

	"github.com/shahid-io/inode/internal/model"
)

// ── filterByDistance ────────────────────────────────────────────────────

func TestFilterByDistance_EmptyInput(t *testing.T) {
	out := filterByDistance(nil, 1.0)
	if len(out) != 0 {
		t.Fatalf("expected empty output, got %d notes", len(out))
	}
}

func TestFilterByDistance_AllUnderThreshold(t *testing.T) {
	notes := []*model.Note{
		{ID: "a", Distance: 0.2},
		{ID: "b", Distance: 0.5},
		{ID: "c", Distance: 0.9},
	}
	out := filterByDistance(notes, 1.0)
	if len(out) != 3 {
		t.Fatalf("expected all 3 notes kept, got %d", len(out))
	}
}

func TestFilterByDistance_AllOverThreshold(t *testing.T) {
	notes := []*model.Note{
		{ID: "a", Distance: 1.2},
		{ID: "b", Distance: 1.5},
		{ID: "c", Distance: 1.9},
	}
	out := filterByDistance(notes, 1.0)
	if len(out) != 0 {
		t.Fatalf("expected 0 notes kept, got %d", len(out))
	}
}

func TestFilterByDistance_Mixed(t *testing.T) {
	notes := []*model.Note{
		{ID: "keep1", Distance: 0.3},
		{ID: "drop1", Distance: 1.4},
		{ID: "keep2", Distance: 0.8},
		{ID: "drop2", Distance: 1.1},
	}
	out := filterByDistance(notes, 1.0)
	if len(out) != 2 {
		t.Fatalf("expected 2 notes kept, got %d", len(out))
	}
	if out[0].ID != "keep1" || out[1].ID != "keep2" {
		t.Fatalf("expected [keep1, keep2], got [%s, %s]", out[0].ID, out[1].ID)
	}
}

func TestFilterByDistance_AtThresholdIsKept(t *testing.T) {
	notes := []*model.Note{
		{ID: "exact", Distance: 1.0},
	}
	out := filterByDistance(notes, 1.0)
	if len(out) != 1 {
		t.Fatalf("note at exact threshold should be kept (Distance <= threshold); got %d notes", len(out))
	}
}

func TestFilterByDistance_NegativeThresholdDisables(t *testing.T) {
	notes := []*model.Note{
		{ID: "a", Distance: 0.5},
		{ID: "b", Distance: 1.9},
		{ID: "c", Distance: 0.0},
	}
	out := filterByDistance(notes, -1)
	if len(out) != 3 {
		t.Fatalf("negative threshold should disable filtering; expected 3, got %d", len(out))
	}
}

func TestFilterByDistance_DoesNotMutateInput(t *testing.T) {
	n1 := &model.Note{ID: "a", Distance: 0.5}
	n2 := &model.Note{ID: "b", Distance: 1.5} // would be dropped
	n3 := &model.Note{ID: "c", Distance: 0.7}
	input := []*model.Note{n1, n2, n3}

	_ = filterByDistance(input, 1.0)

	if len(input) != 3 {
		t.Fatalf("input slice length changed: got %d, want 3", len(input))
	}
	if input[0] != n1 || input[1] != n2 || input[2] != n3 {
		t.Fatalf("input slice contents reordered or replaced")
	}
}

// ── thresholdFor (precedence: opt override beats service default) ──────

func TestThresholdFor_ZeroOptsUsesServiceDefault(t *testing.T) {
	s := &SearchService{maxDistance: 1.0}
	got := s.thresholdFor(SearchOptions{}) // MaxDistance: 0 → use default
	if got != 1.0 {
		t.Fatalf("expected service default 1.0, got %v", got)
	}
}

func TestThresholdFor_PositiveOverrideWins(t *testing.T) {
	s := &SearchService{maxDistance: 1.0}
	got := s.thresholdFor(SearchOptions{MaxDistance: 0.5})
	if got != 0.5 {
		t.Fatalf("expected caller override 0.5, got %v", got)
	}
}

func TestThresholdFor_NegativeOverrideWins(t *testing.T) {
	s := &SearchService{maxDistance: 1.0}
	got := s.thresholdFor(SearchOptions{MaxDistance: -1})
	if got != -1 {
		t.Fatalf("expected caller override -1 (disable), got %v", got)
	}
}

// ── filterByMatchedIDs ─────────────────────────────────────────────────

func TestFilterByMatchedIDs_NoIDs_ReturnsNil(t *testing.T) {
	notes := []*model.Note{{ID: "a1f4d9c2-aaaa"}}
	got := filterByMatchedIDs(notes, nil)
	if got != nil {
		t.Fatalf("expected nil for empty matched IDs, got %v", got)
	}
}

func TestFilterByMatchedIDs_ShortIDPrefixMatchesFullUUID(t *testing.T) {
	notes := []*model.Note{
		{ID: "a1f4d9c2-1111-2222-3333-444444444444"},
		{ID: "b2e5e8d3-aaaa-bbbb-cccc-dddddddddddd"},
		{ID: "c3d6f9e4-eeee-ffff-1111-222222222222"},
	}
	got := filterByMatchedIDs(notes, []string{"a1f4d9c2", "c3d6f9e4"})
	if len(got) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(got))
	}
	if got[0].ID[:8] != "a1f4d9c2" || got[1].ID[:8] != "c3d6f9e4" {
		t.Fatalf("wrong notes returned: %v %v", got[0].ID, got[1].ID)
	}
}

func TestFilterByMatchedIDs_FullUUIDAlsoMatches(t *testing.T) {
	full := "a1f4d9c2-1111-2222-3333-444444444444"
	notes := []*model.Note{{ID: full}, {ID: "b2e5e8d3-aaaa"}}
	got := filterByMatchedIDs(notes, []string{full})
	if len(got) != 1 || got[0].ID != full {
		t.Fatalf("expected full-UUID match, got %v", got)
	}
}

func TestFilterByMatchedIDs_BogusIDsIgnored(t *testing.T) {
	notes := []*model.Note{{ID: "a1f4d9c2-1111"}}
	got := filterByMatchedIDs(notes, []string{"zzzzzzzz"})
	if len(got) != 0 {
		t.Fatalf("expected 0 matches for bogus ID, got %d", len(got))
	}
}

func TestFilterByMatchedIDs_EmptyStringIDIgnored(t *testing.T) {
	// HasPrefix("anything", "") == true, so we must guard against empty strings.
	notes := []*model.Note{{ID: "a1f4d9c2-1111"}, {ID: "b2e5e8d3-aaaa"}}
	got := filterByMatchedIDs(notes, []string{""})
	if len(got) != 0 {
		t.Fatalf("expected empty-string ID to match nothing, got %d notes", len(got))
	}
}

func TestFilterByMatchedIDs_DeduplicatesViaBreak(t *testing.T) {
	// If a note matches multiple supplied IDs, it should appear only once.
	notes := []*model.Note{{ID: "a1f4d9c2-1111"}}
	got := filterByMatchedIDs(notes, []string{"a1", "a1f4", "a1f4d9c2"})
	if len(got) != 1 {
		t.Fatalf("expected single occurrence, got %d", len(got))
	}
}
