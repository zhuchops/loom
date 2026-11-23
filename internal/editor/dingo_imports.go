package editor

import (
	"context"
	"os/exec"
	"regexp"
	"slices"
	"strings"
	"sync"
	"time"
	"unicode"
)

// dingo-lsp answers from the Go file it last generated, so a package that is
// not imported yet, or a line just typed, gets no completions. For .dingo
// files Loom fills that gap itself: after `fmt.` it lists the exports of a
// standard package of that name, and accepting one adds the import to the
// file being edited.

var stdPackages struct {
	once  sync.Once
	byEnd map[string][]string
}

// stdImportPaths lists the standard library packages whose last element is name.
func stdImportPaths(name string) []string {
	stdPackages.once.Do(func() {
		stdPackages.byEnd = map[string][]string{}
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		out, err := exec.CommandContext(ctx, "go", "list", "std").Output()
		if err != nil {
			return
		}
		for _, path := range strings.Fields(string(out)) {
			if path == "C" || strings.HasPrefix(path, "cmd/") || strings.HasPrefix(path, "vendor/") ||
				strings.Contains(path, "internal") {
				continue
			}
			end := path[strings.LastIndex(path, "/")+1:]
			stdPackages.byEnd[end] = append(stdPackages.byEnd[end], path)
		}
		for _, paths := range stdPackages.byEnd {
			slices.Sort(paths)
		}
	})
	return stdPackages.byEnd[name]
}

var packageExports struct {
	sync.Mutex
	items map[string][]completionItem
}

// exportsOf lists the exported names of a standard package, from `go doc`.
func exportsOf(path string) []completionItem {
	packageExports.Lock()
	defer packageExports.Unlock()
	if items, ok := packageExports.items[path]; ok {
		return items
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, "go", "doc", "-short", path).Output()
	if err != nil {
		return nil
	}
	var items []completionItem
	for _, line := range strings.Split(string(out), "\n") {
		keyword, rest, ok := strings.Cut(line, " ")
		if !ok || strings.HasPrefix(line, " ") {
			continue
		}
		name := rest
		if i := strings.IndexAny(name, "([ ="); i >= 0 {
			name = name[:i]
		}
		if name == "" || !unicode.IsUpper([]rune(name)[0]) {
			continue
		}
		kind := map[string]int{"func": 3, "type": 7, "const": 21, "var": 6}[keyword]
		if kind == 0 {
			continue
		}
		if kind == 7 && strings.Contains(rest, " struct") {
			kind = 22
		}
		detail := rest
		if keyword != "func" {
			detail = keyword + " " + rest
		}
		items = append(items, completionItem{Label: name, Kind: kind, Detail: detail, FilterText: name, importPath: path})
	}
	if packageExports.items == nil {
		packageExports.items = map[string][]completionItem{}
	}
	packageExports.items[path] = items
	return items
}

var importText = regexp.MustCompile(`^\s*(?:import\s*\(?\s*)?(?:[\w.]+\s+)?"([^"\s]+)"\s*\)?\s*$`)

// importedPath is the package a server's edit imports, or "" when the text
// is anything else than an import line (or a one-line block).
func importedPath(text string) string {
	if m := importText.FindStringSubmatch(text); m != nil {
		return m[1]
	}
	return ""
}

// dingoKeywords are offered as completions in .dingo files.
var dingoKeywords = strings.Fields("break case chan const continue default defer else enum fallthrough for func go goto guard if import in interface let map match package range return select struct switch type var where")

// stdPackageNames lists the standard packages not yet imported whose name
// starts with prefix, as completion items that add the import when accepted.
func stdPackageNames(prefix, source string) []completionItem {
	stdImportPaths("") // load the list
	var out []completionItem
	for name, paths := range stdPackages.byEnd {
		if !strings.HasPrefix(name, prefix) {
			continue
		}
		for _, path := range paths {
			if !strings.Contains(source, `"`+path+`"`) {
				out = append(out, completionItem{Label: name, Kind: 9, Detail: "package " + path, FilterText: name, importPath: path})
			}
		}
	}
	slices.SortFunc(out, func(a, b completionItem) int { return strings.Compare(a.Label+a.Detail, b.Label+b.Detail) })
	return out
}

// bareWordCompletions adds what dingo-lsp lacks for a word that is not after
// a dot: keywords, and standard packages that are not imported yet.
func bareWordCompletions(text []rune, cursor int, have []completionItem) []completionItem {
	start := cursor
	for start > 0 && isWordRune(text[start-1]) {
		start--
	}
	if start == cursor || start > 0 && text[start-1] == '.' {
		return nil
	}
	prefix := strings.ToLower(string(text[start:cursor]))
	seen := map[string]bool{}
	for _, item := range have {
		seen[item.Label] = true
	}
	var out []completionItem
	for _, word := range dingoKeywords {
		if strings.HasPrefix(word, prefix) && !seen[word] {
			out = append(out, completionItem{Label: word, Kind: 14, Detail: "keyword", FilterText: word})
		}
	}
	if len(prefix) >= 2 {
		for _, item := range stdPackageNames(prefix, string(text)) {
			if !seen[item.Label] {
				out = append(out, item)
			}
		}
	}
	return out
}

func isWordRune(r rune) bool { return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r) }

// packageBeforeDot finds `name` in `name.pre|`, where | is the cursor.
func packageBeforeDot(text []rune, cursor int) (string, bool) {
	if cursor > len(text) {
		return "", false
	}
	start := cursor
	for start > 0 && isWordRune(text[start-1]) {
		start--
	}
	if start == 0 || text[start-1] != '.' {
		return "", false
	}
	end := start - 1
	from := end
	for from > 0 && isWordRune(text[from-1]) {
		from--
	}
	if from == end || !unicode.IsLower(text[from]) || from > 0 && text[from-1] == '.' {
		return "", false
	}
	return string(text[from:end]), true
}

// unimportedCompletions offers the exports of the standard packages named
// like the word before the dot, when the file does not import them yet.
func unimportedCompletions(text []rune, cursor int) []completionItem {
	name, ok := packageBeforeDot(text, cursor)
	if !ok {
		return nil
	}
	source := string(text)
	var out []completionItem
	for _, path := range stdImportPaths(name) {
		if strings.Contains(source, `"`+path+`"`) {
			continue
		}
		out = append(out, exportsOf(path)...)
	}
	return out
}

// importEdit adds `import "path"` to the file: into an import block when
// there is one, after the last single import, or under the package line.
func importEdit(b *buffer, path string) (edit, bool) {
	if strings.Contains(string(b.text), `"`+path+`"`) {
		return edit{}, false
	}
	var block, last, pkg = -1, -1, -1
	for r := range b.lines {
		line := strings.TrimSpace(string(b.text[b.lines[r]:b.lineEnd(r)]))
		switch {
		case strings.HasPrefix(line, "package ") && pkg < 0:
			pkg = r
		case line == "import (" || strings.HasPrefix(line, "import ("):
			if block < 0 {
				block = r
			}
		case strings.HasPrefix(line, "import "):
			last = r
		}
	}
	at := func(r int) int { return b.lineEnd(r) }
	switch {
	case block >= 0:
		return edit{span{at(block), at(block)}, "\n\t\"" + path + "\""}, true
	case last >= 0:
		return edit{span{at(last), at(last)}, "\nimport \"" + path + "\""}, true
	case pkg >= 0:
		return edit{span{at(pkg), at(pkg)}, "\n\nimport \"" + path + "\""}, true
	}
	return edit{span{0, 0}, "import \"" + path + "\"\n\n"}, true
}
