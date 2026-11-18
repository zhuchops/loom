package syntax

import (
	"strings"
	"unicode"
)

// spec describes just enough of a language for lexical highlighting.
type spec struct {
	line      []string  // line comment prefixes
	block     [2]string // block comment delimiters
	keywords  string    // space separated, compared case-insensitively
	constants string
	quotes    string // characters that start a string
	triple    bool   // """ strings span lines
	labels    bool   // "name:" at line start is a label
	directive bool   // ".name" is a directive
}

const defaultKeywords = "fn func function const let var interface type enum struct return if else for while match case switch class def from import use pub export async await new try catch except throw yield select where insert update delete package public private static mut impl mod trait extends implements"

var cStyle = spec{line: []string{"//"}, block: [2]string{"/*", "*/"}, quotes: "\"'`"}

func withSpec(base spec, f func(*spec)) spec { f(&base); return base }

var specs = map[string]spec{
	"typescript":      cStyle,
	"typescriptreact": cStyle,
	"javascript":      cStyle,
	"javascriptreact": cStyle,
	"rust":            cStyle,
	"css":             {block: [2]string{"/*", "*/"}, quotes: "\"'"},
	"html":            {block: [2]string{"<!--", "-->"}, quotes: "\"'"},
	"json":            {quotes: "\""},
	"python":          {line: []string{"#"}, quotes: "\"'", triple: true, keywords: "and as assert async await break class continue def del elif else except finally for from global if import in is lambda nonlocal not or pass raise return try while with yield match case", constants: "true false none self cls"},
	"yaml":            {line: []string{"#"}, quotes: "\"'"},
	"toml":            {line: []string{"#"}, quotes: "\"'"},
	"shellscript":     {line: []string{"#"}, quotes: "\"'`", keywords: "if then else elif fi for while until do done case esac in function select return exit export local readonly source alias unset"},
	"lua":             {line: []string{"--"}, block: [2]string{"--[[", "]]"}, quotes: "\"'", keywords: "and break do else elseif end for function goto if in local not or repeat return then until while"},
	"sql":             {line: []string{"--"}, block: [2]string{"/*", "*/"}, quotes: "\"'", keywords: "select from where insert into values update set delete create alter drop table index view join left right inner outer on group by order having limit offset union as and or not null is in like between distinct primary key foreign references default case when then else end begin commit rollback"},
	"markdown":        {block: [2]string{"<!--", "-->"}, quotes: "`", keywords: "-"},

	"c": withSpec(cStyle, func(s *spec) {
		s.keywords = "auto break case char const continue default do double else enum extern float for goto if inline int long register restrict return short signed sizeof static struct switch typedef union unsigned void volatile while bool"
		s.constants = "true false null nullptr"
	}),
	"cpp": withSpec(cStyle, func(s *spec) {
		s.keywords = "alignas alignof auto bool break case catch char class concept const consteval constexpr constinit const_cast continue co_await co_return co_yield decltype default delete do double dynamic_cast else enum explicit export extern float for friend goto if inline int long mutable namespace new noexcept operator private protected public register reinterpret_cast requires return short signed sizeof static static_assert static_cast struct switch template this throw try typedef typeid typename union unsigned using virtual void volatile while override final"
		s.constants = "true false nullptr"
	}),
	"java": withSpec(cStyle, func(s *spec) {
		s.quotes, s.triple = "\"'", true
		s.keywords = "abstract assert boolean break byte case catch char class const continue default do double else enum extends final finally float for goto if implements import instanceof int interface long native new package private protected public return short static strictfp super switch synchronized this throw throws transient try void volatile while var record sealed permits yield"
		s.constants = "true false null"
	}),
	"kotlin": withSpec(cStyle, func(s *spec) {
		s.quotes, s.triple = "\"'", true
		s.keywords = "abstract annotation as break by catch class companion const constructor continue crossinline data do else enum expect external final finally for fun get if import in infix init inline inner interface internal is lateinit noinline object open operator out override package private protected public reified return sealed set super suspend this throw try typealias val var vararg when where while"
		s.constants = "true false null"
	}),
	"scala": withSpec(cStyle, func(s *spec) {
		s.quotes, s.triple = "\"'", true
		s.keywords = "abstract case catch class def do else enum export extends final finally for given if implicit import lazy match new object override package private protected return sealed super then this throw trait try type using val var while with yield"
		s.constants = "true false null"
	}),
	"swift": withSpec(cStyle, func(s *spec) {
		s.quotes, s.triple = "\"", true
		s.keywords = "actor associatedtype async await break case catch class continue default defer deinit do else enum extension fallthrough fileprivate for func guard if import in init inout internal is let nonisolated open operator override private protocol public repeat rethrows return self static struct subscript super switch throw throws try typealias var where while"
		s.constants = "true false nil"
	}),
	"csharp": withSpec(cStyle, func(s *spec) {
		s.quotes, s.triple = "\"'", true
		s.keywords = "abstract as async await base bool break byte case catch char checked class const continue decimal default delegate do double else enum event explicit extern finally fixed float for foreach goto if implicit in int interface internal is lock long namespace new object operator out override params private protected public readonly record ref return sbyte sealed short sizeof stackalloc static string struct switch this throw try typeof uint ulong unchecked unsafe ushort using var virtual void volatile while yield"
		s.constants = "true false null"
	}),
	"php": withSpec(cStyle, func(s *spec) {
		s.line = []string{"//", "#"}
		s.quotes = "\"'"
		s.keywords = "abstract and as break case catch class clone const continue declare default do echo else elseif empty extends final finally fn for foreach function global goto if implements include include_once instanceof interface isset list match namespace new or print private protected public readonly require require_once return static switch throw trait try unset use var while xor yield"
		s.constants = "true false null"
	}),
	"dart": withSpec(cStyle, func(s *spec) {
		s.quotes, s.triple = "\"'", true
		s.keywords = "abstract as assert async await break case catch class const continue covariant default deferred do dynamic else enum export extends extension external factory final finally for get if implements import in interface is late library mixin new on operator part required rethrow return set static super switch sync this throw try typedef var void while with yield"
		s.constants = "true false null"
	}),
	"zig":        {line: []string{"//"}, quotes: "\"'", keywords: "addrspace align allowzero and anyframe anytype asm async await break callconv catch comptime const continue defer else enum errdefer error export extern fn for if inline noalias nosuspend noinline opaque or orelse packed pub resume return linksection struct suspend switch test threadlocal try union unreachable usingnamespace var volatile while", constants: "true false null undefined"},
	"go":         cStyle,
	"haskell":    {line: []string{"--"}, block: [2]string{"{-", "-}"}, quotes: "\"", keywords: "case class data default deriving do else foreign if import in infix infixl infixr instance let module newtype of then type where forall qualified as hiding", constants: "true false"},
	"elixir":     {line: []string{"#"}, quotes: "\"'", triple: true, keywords: "def defp defmodule defmacro defstruct defprotocol defimpl do end fn if else unless cond case when with for receive after try catch rescue raise import alias require use quote unquote and or not in", constants: "true false nil"},
	"ocaml":      {block: [2]string{"(*", "*)"}, quotes: "\"", keywords: "and as assert begin class constraint do done downto else end exception external for fun function functor if in include inherit initializer lazy let match method module mutable new nonrec object of open or private rec sig struct then to try type val virtual when while with", constants: "true false"},
	"ruby":       {line: []string{"#"}, quotes: "\"'`", keywords: "alias and begin break case class def defined do else elsif end ensure for if in module next not or redo rescue retry return self super then undef unless until when while yield require require_relative attr_accessor attr_reader attr_writer", constants: "true false nil"},
	"perl":       {line: []string{"#"}, quotes: "\"'`", keywords: "my our local sub package use require if elsif else unless while until for foreach return last next redo do eval print say die"},
	"r":          {line: []string{"#"}, quotes: "\"'", keywords: "function if else for while repeat break next return in", constants: "true false null na inf nan"},
	"dockerfile": {line: []string{"#"}, quotes: "\"'", keywords: "from run cmd label maintainer expose env add copy entrypoint volume user workdir arg onbuild stopsignal healthcheck shell as"},
	"makefile":   {line: []string{"#"}, quotes: "\"'", keywords: "include ifeq ifneq ifdef ifndef else endif define endef export override vpath"},
	"cmake":      {line: []string{"#"}, quotes: "\"", keywords: "if else elseif endif foreach endforeach while endwhile function endfunction macro endmacro set project include return"},
	"proto":      {line: []string{"//"}, block: [2]string{"/*", "*/"}, quotes: "\"'", keywords: "syntax package import option message enum service rpc returns repeated optional required oneof map reserved extend stream", constants: "true false"},
	"graphql":    {line: []string{"#"}, quotes: "\"", keywords: "query mutation subscription fragment on type interface union enum input scalar schema extend directive implements", constants: "true false null"},
	"terraform":  {line: []string{"#", "//"}, block: [2]string{"/*", "*/"}, quotes: "\"", keywords: "resource variable output module data provider locals terraform for in if", constants: "true false null"},
	"xml":        {block: [2]string{"<!--", "-->"}, quotes: "\"'", keywords: "-"},
	"asm": {line: []string{";", "#", "//"}, block: [2]string{"/*", "*/"}, quotes: "\"'", labels: true, directive: true,
		keywords:  "mov movzx movsx movsxd lea push pop add sub mul imul div idiv inc dec neg not and or xor shl shr sal sar rol ror cmp test jmp je jne jz jnz jg jge jl jle ja jae jb jbe js jns call ret leave nop int syscall sysenter hlt xchg cmov cmove cmovne loop rep movs stos lods scas cdq cqo cwd bt bts btr cld std adc sbb bl blx bx ldr str ldp stp adr adrp cbz cbnz b beq bne bgt blt ret svc mrs msr li la lw sw lb sb addi jal jr beqz bnez mv ecall",
		constants: "rax rbx rcx rdx rsi rdi rbp rsp r8 r9 r10 r11 r12 r13 r14 r15 eax ebx ecx edx esi edi ebp esp ax bx cx dx si di bp sp al bl cl dl ah bh ch dh xmm0 xmm1 xmm2 xmm3 x0 x1 x2 x3 x29 x30 sp lr pc zero ra"},
}

// commentSyntax returns the line-comment delimiters used by the comment toggle.
// CommentSyntax returns the delimiters the comment toggle uses for lang.
func CommentSyntax(lang string) (open, close string, ok bool) {
	switch lang {
	case "html", "markdown", "xml":
		return "<!--", "-->", true
	case "css":
		return "/*", "*/", true
	case "ocaml":
		return "(*", "*)", true
	case "haskell", "lua", "sql":
		return "--", "", true
	case "asm":
		return ";", "", true
	}
	if spec, found := specs[lang]; found && len(spec.line) > 0 && lang != "json" {
		return spec.line[0], "", true
	}
	return "", "", false
}

func highlightGeneric(r []rune, lang string) []Token {
	spec := specs[lang]
	keywordList := spec.keywords
	if keywordList == "" {
		keywordList = defaultKeywords
	}
	set := func(list string) map[string]bool {
		m := map[string]bool{}
		for _, w := range strings.Fields(list) {
			m[w] = true
		}
		return m
	}
	keywords, constants := set(keywordList), set("true false null none nil undefined "+spec.constants)
	if lang == "asm" {
		constants = set(spec.constants)
	}
	hasAt := func(i int, s string) bool {
		if s == "" || i+len(s) > len(r) {
			return false
		}
		for k, c := range s {
			if r[i+k] != c {
				return false
			}
		}
		return true
	}
	isWord := func(ch rune) bool {
		return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch) || unicode.IsMark(ch)
	}
	var result []Token
	var previous string
	for i := 0; i < len(r); {
		start := i
		ch := r[i]
		i++
		var kind string
		matched := false
		for _, prefix := range spec.line {
			if hasAt(start, prefix) {
				kind, matched = "comment", true
				for i < len(r) && r[i] != '\n' {
					i++
				}
				break
			}
		}
		switch {
		case matched:
		case hasAt(start, spec.block[0]):
			kind = "comment"
			i = start + len(spec.block[0])
			for i < len(r) && !hasAt(i, spec.block[1]) {
				i++
			}
			i = min(len(r), i+len(spec.block[1]))
		case strings.ContainsRune(spec.quotes, ch):
			kind = "string"
			if spec.triple && ch != '`' && start+2 < len(r) && r[start+1] == ch && r[start+2] == ch {
				i = start + 3
				for i < len(r) && !(r[i] == ch && hasAt(i, string([]rune{ch, ch, ch}))) {
					i++
				}
				i = min(len(r), i+3)
				break
			}
			for i < len(r) {
				if r[i] == '\\' {
					i = min(len(r), i+2)
					continue
				}
				if r[i] == '\n' && ch != '`' {
					break // an unterminated string ends with its line
				}
				end := r[i] == ch
				i++
				if end {
					break
				}
			}
			if i < len(r) && r[i] == ':' && lang == "json" {
				kind = "property"
			}
		case spec.directive && ch == '.' && i < len(r) && unicode.IsLetter(r[i]):
			for i < len(r) && isWord(r[i]) {
				i++
			}
			kind = "keyword"
		case isWord(ch):
			for i < len(r) && isWord(r[i]) {
				i++
			}
			word := string(r[start:i])
			lower := strings.ToLower(word)
			next := i
			for next < len(r) && (r[next] == ' ' || r[next] == '\t') {
				next++
			}
			switch {
			case unicode.IsDigit(ch):
				kind = "number"
			case spec.labels && i < len(r) && r[i] == ':':
				kind = "function"
			case constants[lower]:
				kind = "constant"
			case keywords[lower]:
				kind = "keyword"
			case lang != "asm" && strings.Contains("|class|type|interface|struct|enum|trait|record|object|", "|"+previous+"|"):
				kind = "type"
			case next < len(r) && r[next] == '(' && lang != "asm":
				kind = "function"
			case next < len(r) && (r[next] == ':' || r[next] == '=') && (lang == "yaml" || lang == "toml" || lang == "css"):
				kind = "property"
			case unicode.IsUpper(ch) && lang != "asm":
				kind = "type"
			}
			previous = lower
		case lang == "markdown" && ch == '#' && (start == 0 || r[start-1] == '\n'):
			kind = "function"
			for i < len(r) && r[i] != '\n' {
				i++
			}
		}
		if kind != "" {
			result = append(result, Token{start, i, kind})
		}
	}
	return result
}
