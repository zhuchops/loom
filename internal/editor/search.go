package editor

import (
	"fmt"
	"regexp"
	"sort"
	"unicode/utf8"
)

// searchOptions are the toggles next to the find field. With all of them off a
// query is plain text that ignores case.
type searchOptions struct {
	caseSensitive, wholeWord, regex bool
}

type searchResult struct {
	spans  []span  // rune offsets, in order, never empty
	groups [][]int // byte offsets of each match's submatches in text
	re     *regexp.Regexp
	text   string
	err    error
}

type searchCache struct {
	b       *buffer
	version int
	query   string
	opts    searchOptions
	result  searchResult
	valid   bool
}

const maxMatches = 20000

func compileSearch(query string, o searchOptions) (*regexp.Regexp, error) {
	pattern := query
	if !o.regex {
		pattern = regexp.QuoteMeta(query)
	}
	if o.wholeWord {
		pattern = `\b(?:` + pattern + `)\b`
	}
	flags := "(?m"
	if !o.caseSensitive {
		flags += "i"
	}
	return regexp.Compile(flags + ")" + pattern)
}

// matches lists every match of the current query in b. The result is reused
// until the text, the query or the options change.
func (e *editor) matches(b *buffer) searchResult {
	c := &e.searchCache
	if c.valid && c.b == b && c.version == b.version && c.query == e.query && c.opts == e.searchOpts {
		return c.result
	}
	res := searchResult{}
	if e.query != "" {
		re, err := compileSearch(e.query, e.searchOpts)
		if err != nil {
			res.err = err
		} else {
			res.re, res.text = re, string(b.text)
			lastByte, lastRune := 0, 0
			for _, m := range re.FindAllStringSubmatchIndex(res.text, maxMatches) {
				if m[0] == m[1] {
					continue
				}
				start := lastRune + utf8.RuneCountInString(res.text[lastByte:m[0]])
				end := start + utf8.RuneCountInString(res.text[m[0]:m[1]])
				res.spans = append(res.spans, span{start, end})
				res.groups = append(res.groups, m)
				lastByte, lastRune = m[1], end
			}
		}
	}
	*c = searchCache{b, b.version, e.query, e.searchOpts, res, true}
	return res
}

// matchRange returns the matches that touch [from, to).
func (r searchResult) matchRange(from, to int) []span {
	i := sort.Search(len(r.spans), func(i int) bool { return r.spans[i].end > from })
	j := i
	for j < len(r.spans) && r.spans[j].start < to {
		j++
	}
	return r.spans[i:j]
}

// at returns the index of the first match starting at or after offset, wrapping
// to the first match, or -1 when there are none.
func (r searchResult) at(offset int) int {
	if len(r.spans) == 0 {
		return -1
	}
	i := sort.Search(len(r.spans), func(i int) bool { return r.spans[i].start >= offset })
	if i == len(r.spans) {
		return 0
	}
	return i
}

// replacement is the text that replaces match i. Regex replacements expand $1.
func (e *editor) replacement(r searchResult, i int, template string) string {
	if !e.searchOpts.regex || r.re == nil {
		return template
	}
	return string(r.re.ExpandString(nil, template, r.text, r.groups[i]))
}

// find moves to the next or previous match, wrapping around the buffer.
func (e *editor) find(reverse bool) {
	if e.query == "" {
		return
	}
	b := e.b
	res := e.matches(b)
	if res.err != nil {
		e.notify("Invalid pattern: " + res.err.Error())
		return
	}
	if len(res.spans) == 0 {
		e.notify(fmt.Sprintf("No match for %q", e.query))
		return
	}
	var target span
	if reverse {
		i := sort.Search(len(res.spans), func(i int) bool { return res.spans[i].start >= b.cursor })
		target = res.spans[(i-1+len(res.spans))%len(res.spans)]
	} else {
		i := sort.Search(len(res.spans), func(i int) bool { return res.spans[i].start > b.cursor })
		target = res.spans[i%len(res.spans)]
	}
	e.searchVisible = true
	b.cursor = target.start
	e.clampModalCursor()
}
