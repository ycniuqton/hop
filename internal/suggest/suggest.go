// Package suggest finds "did you mean" candidates by edit distance.
package suggest

// Matcher picks the closest candidate within a maximum edit distance.
type Matcher struct {
	MaxDistance int
}

// DidYouMean is the policy from spec section 6: Levenshtein distance ≤ 2.
var DidYouMean = Matcher{MaxDistance: 2}

// Closest returns the nearest candidate, or "" when none is close enough.
func (m Matcher) Closest(word string, candidates []string) string {
	best, bestDist := "", m.MaxDistance+1
	for _, c := range candidates {
		if c == word {
			continue
		}
		if d := m.distance(word, c); d < bestDist {
			best, bestDist = c, d
		}
	}
	return best
}

func (m Matcher) distance(a, b string) int {
	ra, rb := []rune(a), []rune(b)
	prev := make([]int, len(rb)+1)
	cur := make([]int, len(rb)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = i
		for j := 1; j <= len(rb); j++ {
			cost := 1
			if ra[i-1] == rb[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(rb)]
}
