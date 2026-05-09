package model

import "testing"

func TestIsValidCategory_KnownCategoriesAccepted(t *testing.T) {
	for _, c := range Categories {
		if !IsValidCategory(c.Name) {
			t.Errorf("expected predefined category %q to be valid", c.Name)
		}
	}
}

func TestIsValidCategory_CaseInsensitive(t *testing.T) {
	for _, in := range []string{"Credentials", "CREDENTIALS", "  credentials  "} {
		if !IsValidCategory(in) {
			t.Errorf("expected %q to be valid (case/whitespace tolerant)", in)
		}
	}
}

func TestIsValidCategory_UnknownRejected(t *testing.T) {
	for _, in := range []string{"", "credential", "command-line", "todo", "foo", "code"} {
		if IsValidCategory(in) {
			t.Errorf("expected %q to be rejected", in)
		}
	}
}

func TestFallbackCategory_IsItselfValid(t *testing.T) {
	// Sanity check: the fallback target must be a real category, otherwise
	// the classifier would be writing invalid values into the DB.
	if !IsValidCategory(FallbackCategory) {
		t.Fatalf("FallbackCategory %q is not in Categories", FallbackCategory)
	}
}
