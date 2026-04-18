package main

import (
	"sort"
	"strings"
)

type scoredEntry struct {
	name  string
	score int
}

func fuzzyScore(s, q string) int {
	s = strings.ToLower(s)
	q = strings.ToLower(q)
	if q == "" {
		return 1
	}
	si := 0
	score := 0
	streak := 0
	for _, qc := range q {
		found := false
		for si < len(s) {
			if rune(s[si]) == qc {
				found = true
				streak++
				score += 10 + streak*3
				si++
				break
			}
			streak = 0
			si++
		}
		if !found {
			return 0
		}
	}
	score += max(1, 100-len(s))
	return score
}

func fuzzyFilter(names []string, query string, limit int) []string {
	var out []scoredEntry
	for _, n := range names {
		s := fuzzyScore(n, query)
		if s > 0 {
			out = append(out, scoredEntry{name: n, score: s})
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].score == out[j].score {
			return out[i].name < out[j].name
		}
		return out[i].score > out[j].score
	})
	if limit <= 0 || limit > len(out) {
		limit = len(out)
	}
	result := make([]string, 0, limit)
	for i := 0; i < limit; i++ {
		result = append(result, out[i].name)
	}
	return result
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
