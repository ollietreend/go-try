package fuzzy

import (
	"math"
	"strings"
)

const (
	charMatch    = 1.0
	wordBoundary = 1.0
	dateBonus    = 2.0
	recencyScale = 3.0
	lengthPenaltyScale = 10.0
)

var wordBoundaryReplacer = strings.NewReplacer(
	"-", " ", "_", " ", ".", " ",
)

func isWordStart(name string, pos int) bool {
	if pos == 0 {
		return true
	}
	b := name[pos-1]
	return b == '-' || b == '_' || b == '.' || b == ' '
}

var sqrtTable [64]float64

func init() {
	for i := 0; i < 64; i++ {
		sqrtTable[i] = math.Sqrt(float64(i + 1))
	}
}

func proximityBonus(gap int) float64 {
	if gap < 64 {
		return 2.0 / sqrtTable[gap]
	}
	return 2.0 / math.Sqrt(float64(gap+1))
}

type Entry struct {
	Path      string
	Name      string
	Mtime     int64
	IsSymlink bool
	BaseScore float64
}

type MatchResult struct {
	Entry     Entry
	Score     float64
	Positions []int
}

type Matcher struct {
	entries []indexedEntry
}

type indexedEntry struct {
	entry   Entry
	lower   string
}

func New(entries []Entry) *Matcher {
	idx := make([]indexedEntry, len(entries))
	for i, e := range entries {
		idx[i] = indexedEntry{
			entry: e,
			lower: strings.ToLower(e.Name),
		}
	}
	return &Matcher{entries: idx}
}

func (m *Matcher) Match(query string) []MatchResult {
	queryLower := strings.ToLower(query)
	queryRunes := []rune(queryLower)

	results := make([]MatchResult, 0)

	for _, ie := range m.entries {
		nameRunes := []rune(ie.lower)

		if len(queryRunes) == 0 {
			results = append(results, MatchResult{
				Entry:     ie.entry,
				Score:     ie.entry.BaseScore,
				Positions: nil,
			})
			continue
		}

		score, positions := calculateMatch(ie.entry.BaseScore, nameRunes, queryRunes, len(ie.entry.Name))
		if positions == nil {
			continue
		}
		results = append(results, MatchResult{
			Entry:     ie.entry,
			Score:     score,
			Positions: positions,
		})
	}

	return results
}

func calculateMatch(baseScore float64, nameRunes, queryRunes []rune, nameLen int) (float64, []int) {
	positions := make([]int, 0, len(queryRunes))
	score := baseScore

	p := 0
	lastPos := -1

	for _, qc := range queryRunes {
		found := -1
		for i := p; i < len(nameRunes); i++ {
			if nameRunes[i] == qc {
				found = i
				break
			}
		}
		if found < 0 {
			return 0, nil
		}

		positions = append(positions, found)
		score += charMatch

		if isWordStart(string(nameRunes), found) {
			score += wordBoundary
		}

		if lastPos >= 0 {
			gap := found - lastPos - 1
			score += proximityBonus(gap)
		}

		lastPos = found
		p = found + 1
	}

	if lastPos >= 0 {
		score *= float64(len(queryRunes)) / float64(lastPos+1)
	}
	score *= lengthPenaltyScale / (float64(nameLen) + lengthPenaltyScale)

	return score, positions
}

func HasDatePrefix(name string) bool {
	if len(name) < 11 {
		return false
	}
	for i := 0; i < 4; i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	if name[4] != '-' {
		return false
	}
	for i := 5; i < 7; i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	if name[7] != '-' {
		return false
	}
	for i := 8; i < 10; i++ {
		if name[i] < '0' || name[i] > '9' {
			return false
		}
	}
	if name[10] != '-' {
		return false
	}
	return true
}

func DatePrefixBonus(name string) float64 {
	if HasDatePrefix(name) {
		return dateBonus
	}
	return 0
}

func RecencyBonus(nowUnix int64, mtime int64) float64 {
	hours := float64(nowUnix-mtime) / 3600.0
	return recencyScale / math.Sqrt(hours+1)
}
