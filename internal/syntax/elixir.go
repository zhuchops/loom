package syntax

import (
	"strings"
	"unicode"
)

var elixirKeywords = wordSet("do end fn if else unless cond case when with for receive after try catch rescue raise reraise throw " +
	"import alias require use quote unquote unquote_splicing and or not in super " +
	"def defp defmodule defmacro defmacrop defstruct defprotocol defimpl defdelegate defguard defguardp defexception defoverridable defcallback")

var elixirConstants = wordSet("true false nil")

// Words after which the next name is a function being defined.
var elixirDefiners = wordSet("def defp defmacro defmacrop defdelegate defguard defguardp")

func wordSet(list string) map[string]bool {
	m := map[string]bool{}
	for _, w := range strings.Fields(list) {
		m[w] = true
	}
	return m
}

func elixirWord(c rune) bool { return c == '_' || unicode.IsLetter(c) || unicode.IsDigit(c) }

// closingDelimiter is the rune that ends a sigil opened with open.
func closingDelimiter(open rune) rune {
	switch open {
	case '(':
		return ')'
	case '[':
		return ']'
	case '{':
		return '}'
	case '<':
		return '>'
	}
	return open
}

// highlightElixir colors Elixir source. It is a lexer, not a parser, but it
// knows the parts the generic one misses: atoms, sigils, heredocs, module
// attributes, keyword keys, `?` and `!` in names and pipelines.
func highlightElixir(r []rune) []Token {
	var out []Token
	n := len(r)
	at := func(i int) rune {
		if i >= 0 && i < n {
			return r[i]
		}
		return 0
	}
	hasAt := func(i int, s string) bool {
		for k, c := range s {
			if at(i+k) != c {
				return false
			}
		}
		return true
	}
	add := func(start, end int, kind string) { out = append(out, Token{start, end, kind}) }
	// quoted scans from just after an opening quote to just past its closing one.
	quoted := func(i int, quote rune) int {
		for i < n {
			switch r[i] {
			case '\\':
				i += 2
				continue
			case quote:
				return i + 1
			}
			i++
		}
		return n
	}
	// heredoc scans from just after an opening triple quote.
	heredoc := func(i int, quote rune) int {
		closer := string([]rune{quote, quote, quote})
		for i < n && !hasAt(i, closer) {
			if r[i] == '\\' {
				i++
			}
			i++
		}
		return min(n, i+3)
	}
	// followsPipe reports whether the token at i comes right after |> .
	followsPipe := func(i int) bool {
		for i > 0 && (r[i-1] == ' ' || r[i-1] == '\t' || r[i-1] == '\n') {
			i--
		}
		return i >= 2 && r[i-1] == '>' && r[i-2] == '|'
	}
	var previousWord string
	for i := 0; i < n; {
		start, ch := i, r[i]
		switch {
		case ch == '#':
			for i < n && r[i] != '\n' {
				i++
			}
			add(start, i, "comment")
		case ch == '"' || ch == '\'':
			if hasAt(i, string([]rune{ch, ch, ch})) {
				// A heredoc right after @doc or @moduledoc is documentation.
				kind := "string"
				if previousWord == "@doc" || previousWord == "@moduledoc" || previousWord == "@typedoc" {
					kind = "comment"
				}
				i = heredoc(i+3, ch)
				add(start, i, kind)
				break
			}
			i = quoted(i+1, ch)
			if at(i) == ':' && at(i+1) != ':' {
				i++
				add(start, i, "property") // "quoted key": value
				break
			}
			add(start, i, "string")
		case ch == '~' && unicode.IsLetter(at(i+1)):
			// A sigil: ~r/…/i, ~s"…", ~H"""…""" and the like.
			j := i + 2
			for unicode.IsUpper(at(i+1)) && unicode.IsUpper(at(j)) {
				j++ // upper-case sigils may have long names
			}
			open := at(j)
			if !strings.ContainsRune("/|\"'([{<", open) {
				i++
				break
			}
			if (open == '"' || open == '\'') && hasAt(j, string([]rune{open, open, open})) {
				i = heredoc(j+3, open)
			} else {
				close := closingDelimiter(open)
				i = j + 1
				for i < n && r[i] != close {
					if r[i] == '\\' {
						i++
					}
					i++
				}
				i = min(n, i+1)
			}
			for unicode.IsLetter(at(i)) {
				i++ // modifiers
			}
			kind := "string"
			if r[start+1] == 'r' || r[start+1] == 'R' {
				kind = "regexp"
			}
			add(start, i, kind)
		case ch == '@' && (unicode.IsLetter(at(i+1)) || at(i+1) == '_'):
			i++
			for elixirWord(at(i)) {
				i++
			}
			previousWord = string(r[start:i])
			add(start, i, "decorator")
			continue
		case ch == ':' && at(i-1) != ':' && at(i+1) == '"':
			i = quoted(i+2, '"')
			add(start, i, "constant")
		case ch == ':' && at(i-1) != ':' && (unicode.IsLetter(at(i+1)) || at(i+1) == '_'):
			i++
			for elixirWord(at(i)) {
				i++
			}
			if at(i) == '?' || at(i) == '!' {
				i++
			}
			add(start, i, "constant")
		case ch == '?' && !elixirWord(at(i-1)) && at(i+1) != 0 && at(i+1) != ' ' && at(i+1) != '\n' && !elixirWord(at(i+2)):
			i += 2 // a character literal such as ?a
			add(start, i, "number")
		case unicode.IsDigit(ch):
			for unicode.IsLetter(at(i)) || unicode.IsDigit(at(i)) || at(i) == '_' ||
				(at(i) == '.' && unicode.IsDigit(at(i+1))) {
				i++
			}
			add(start, i, "number")
		case unicode.IsLetter(ch) || ch == '_':
			i++
			for elixirWord(at(i)) {
				i++
			}
			if (at(i) == '?' || at(i) == '!') && at(i+1) != '=' {
				i++
			}
			word := string(r[start:i])
			previousDot := at(start-1) == '.'
			kind := ""
			switch {
			case at(i) == ':' && at(i+1) != ':':
				kind = "property" // name: value, do:
				i++
			case unicode.IsUpper(ch):
				kind = "type"
			case elixirConstants[word] && !previousDot:
				kind = "constant"
			case elixirKeywords[word] && !previousDot:
				kind = "keyword"
			case elixirDefiners[previousWord]:
				kind = "function"
			case at(i) == '(' || followsPipe(start):
				kind = "function"
			}
			if kind == "property" {
				previousWord = ""
			} else {
				previousWord = word
			}
			if kind != "" {
				add(start, i, kind)
			}
			continue
		default:
			i++
		}
		if ch != ' ' && ch != '\t' && ch != '\n' {
			previousWord = ""
		}
	}
	return out
}
