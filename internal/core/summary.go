package core

import (
	"context"
	"fmt"
	"strings"
	"unicode"
)

// Summaries of sensitive notes are stored and shown in plaintext (inode list,
// get sources, note get, MCP), so they must never contain the secret itself.
// The rules here decide whether a summary leaks content and what to show
// instead. They look for secret-looking tokens rather than any shared word:
// most notes are sensitive by default (defaults.sensitive = true), and a
// summary like "kubectl rollout restart runbook" must survive.

// minWindow is the length of the substrings compared between a secret-looking
// token and a summary. It also catches truncated copies of long secrets.
const minWindow = 8

// tokenDelims splits note content into candidate tokens. '@', '.', '-' and
// '_' are kept inside tokens so passwords, emails and key prefixes such as
// sk_test_ stay whole.
func tokenDelims(r rune) bool {
	if unicode.IsSpace(r) {
		return true
	}
	return strings.ContainsRune("=:'\"`,;()<>[]{}|/?&#", r)
}

// secretTokens returns the tokens of content that look like secret material:
// at least 8 characters containing a digit or a symbol (anything other than
// a letter, '.' or '-'). In the credentials category the bar is lower:
// 6 characters with a digit or symbol, or any token of 12 or more characters.
func secretTokens(content, category string) []string {
	var out []string
	for _, tok := range strings.FieldsFunc(content, tokenDelims) {
		tok = strings.Trim(tok, ".!")
		if isSecretLike(tok, category == "credentials") {
			out = append(out, tok)
		}
	}
	return out
}

func isSecretLike(tok string, credentials bool) bool {
	marked := false
	for _, r := range tok {
		if unicode.IsDigit(r) || (!unicode.IsLetter(r) && r != '.' && r != '-') {
			marked = true
			break
		}
	}
	n := len([]rune(tok))
	if credentials {
		return (n >= 6 && marked) || n >= 12
	}
	return n >= minWindow && marked
}

// summaryLeaks reports whether summary contains any secret-looking token of
// content, or any 8-character window of one (case-insensitive).
func summaryLeaks(summary, content, category string) bool {
	s := strings.ToLower(summary)
	for _, tok := range secretTokens(content, category) {
		t := []rune(strings.ToLower(tok))
		if len(t) <= minWindow {
			if strings.Contains(s, string(t)) {
				return true
			}
			continue
		}
		for i := 0; i+minWindow <= len(t); i++ {
			if strings.Contains(s, string(t[i:i+minWindow])) {
				return true
			}
		}
	}
	return false
}

// neutralSummary is shown for a sensitive note when no safe summary exists:
// the category plus any tags that are not themselves secret-looking.
func neutralSummary(category string, tags []string) string {
	if category == "" {
		category = "item"
	}
	var safe []string
	for _, t := range tags {
		if !isSecretLike(t, true) {
			safe = append(safe, t)
		}
	}
	if len(safe) == 0 {
		return category + " (sensitive)"
	}
	return category + " (sensitive): " + strings.Join(safe, ", ")
}

// safeSummary returns summary unless the note is sensitive and the summary is
// empty or leaks content, in which case it returns the neutral form.
func safeSummary(summary, content, category string, tags []string, sensitive bool) string {
	if !sensitive {
		return summary
	}
	if strings.TrimSpace(summary) == "" || summaryLeaks(summary, content, category) {
		return neutralSummary(category, tags)
	}
	return summary
}

// ScrubSensitiveSummaries validates summaries of sensitive notes saved before
// summaries were checked (#53) and replaces the ones that leak content. Each
// note is checked once; the database records it. Returns how many summaries
// were replaced.
func (s *NoteService) ScrubSensitiveSummaries(ctx context.Context) (int, error) {
	notes, err := s.db.UncheckedSensitive(ctx)
	if err != nil {
		return 0, fmt.Errorf("list unchecked summaries: %w", err)
	}
	if len(notes) == 0 {
		return 0, nil
	}

	key, err := s.keyMgr.DeriveKey()
	if err != nil {
		return 0, fmt.Errorf("derive key: %w", err)
	}

	replaced := 0
	for _, n := range notes {
		content := n.ContentPlain
		if len(n.ContentEnc) > 0 {
			plain, err := Decrypt(key, n.ContentEnc, []byte("note"))
			if err != nil {
				// Without the content the summary cannot be judged, so fall back
				// to the neutral form rather than keep a possibly leaking one.
				content = ""
				n.Summary = ""
			} else {
				content = string(plain)
			}
		}
		summary := safeSummary(n.Summary, content, n.Category, n.Tags, true)
		if summary != n.Summary {
			replaced++
		}
		if err := s.db.SetSummary(ctx, n.ID, summary); err != nil {
			return replaced, fmt.Errorf("update summary of %s: %w", n.ID, err)
		}
	}
	return replaced, nil
}
