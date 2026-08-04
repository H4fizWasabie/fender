package skills

import (
	"sort"
	"strings"
)

// stopwords are filtered from matching tokens (small hardcoded set).
var stopwords = map[string]bool{
	"the": true, "and": true, "for": true, "with": true, "when": true,
	"use": true, "you": true, "your": true, "that": true, "this": true,
	"what": true, "want": true, "from": true, "have": true, "has": true,
	"will": true, "can": true, "are": true, "was": true, "were": true,
	"into": true, "over": true, "under": true, "about": true, "than": true,
	"which": true, "where": true, "there": true, "here": true, "then": true,
	"them": true, "they": true, "does": true, "doing": true, "made": true,
	"make": true, "like": true, "just": true, "also": true, "should": true,
}

// words returns significant tokens: len > 3, not stopwords.
func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,;:!?\"'()[]{}<>/\\")
		if len(w) > 3 && !stopwords[w] {
			out[w] = true
		}
	}
	return out
}

// Match scores the message against every model-invokable skill description
// by significant-word overlap. Score >= 2 → candidate; top MatchTopN by
// (score desc, name asc); total body ≤ BodyBudget.
func (r *Registry) Match(message string) []Skill {
	msg := words(message)
	type cand struct {
		s     Skill
		score int
	}
	var cands []cand
	for _, s := range r.all {
		if !s.ModelInvokable {
			continue
		}
		desc := words(s.Description)
		score := 0
		for w := range msg {
			if desc[w] {
				score++
			}
		}
		if score >= 2 {
			cands = append(cands, cand{s, score})
		}
	}
	sort.Slice(cands, func(i, j int) bool {
		if cands[i].score != cands[j].score {
			return cands[i].score > cands[j].score
		}
		return cands[i].s.Name < cands[j].s.Name
	})
	var out []Skill
	used := 0
	for i, c := range cands {
		if i >= MatchTopN || used+len(c.s.Body) > BodyBudget {
			break
		}
		out = append(out, c.s)
		used += len(c.s.Body)
	}
	return out
}
