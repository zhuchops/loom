// Package syntax tokenizes source text into coarse highlight kinds.
package syntax

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"strings"
	"unicode/utf8"
)

// Token marks the rune range [Start, End) of a source text with a kind such as
// "keyword", "string" or "function".
type Token struct {
	Start, End int
	Kind       string
}

// Highlight returns tokens in source order, or nil for languages it does not know.
func Highlight(lang, path string, text []rune) []Token {
	if len(text) > 400000 {
		return nil
	}
	if lang == "go" {
		return highlightGo(path, text)
	}
	if lang == "elixir" {
		return highlightElixir(text)
	}
	if _, ok := specs[lang]; ok {
		return highlightGeneric(text, lang)
	}
	return nil
}

func highlightGo(path string, text []rune) []Token {
	src := []byte(string(text))
	parsed := token.NewFileSet()
	tree, _ := parser.ParseFile(parsed, path, src, parser.SkipObjectResolution)
	roles := map[int]string{}
	mark := func(id *ast.Ident, kind string) {
		if id != nil {
			roles[parsed.Position(id.Pos()).Offset] = kind
		}
	}
	var markExpr func(ast.Expr, string)
	markExpr = func(expr ast.Expr, kind string) {
		switch node := expr.(type) {
		case *ast.Ident:
			mark(node, kind)
		case *ast.SelectorExpr:
			markExpr(node.X, "namespace")
			mark(node.Sel, kind)
		case *ast.StarExpr:
			markExpr(node.X, kind)
		case *ast.ArrayType:
			markExpr(node.Elt, kind)
		case *ast.MapType:
			markExpr(node.Key, kind)
			markExpr(node.Value, kind)
		case *ast.IndexExpr:
			markExpr(node.X, kind)
			markExpr(node.Index, "type")
		case *ast.IndexListExpr:
			markExpr(node.X, kind)
			for _, index := range node.Indices {
				markExpr(index, "type")
			}
		}
	}
	// Visit declarations and uses before tokenizing, so function and type colors
	// remain available while the language server is starting.
	if tree != nil {
		mark(tree.Name, "namespace")
		ast.Inspect(tree, func(node ast.Node) bool {
			switch n := node.(type) {
			case *ast.FuncDecl:
				mark(n.Name, "function")
			case *ast.TypeSpec:
				mark(n.Name, "type")
			case *ast.CallExpr:
				markExpr(n.Fun, "function")
			case *ast.CompositeLit:
				markExpr(n.Type, "type")
			case *ast.SelectorExpr:
				if roles[parsed.Position(n.Sel.Pos()).Offset] == "" {
					mark(n.Sel, "property")
				}
			case *ast.Field:
				markExpr(n.Type, "type")
			case *ast.FuncType:
				for _, list := range []*ast.FieldList{n.Params, n.Results} {
					if list != nil {
						for _, f := range list.List {
							for _, name := range f.Names {
								mark(name, "parameter")
							}
						}
					}
				}
			case *ast.GenDecl:
				if n.Tok == token.CONST {
					for _, spec := range n.Specs {
						if value, ok := spec.(*ast.ValueSpec); ok {
							for _, id := range value.Names {
								mark(id, "constant")
							}
						}
					}
				}
			}
			return true
		})
	}
	f := token.NewFileSet().AddFile(path, -1, len(src))
	var lexer scanner.Scanner
	lexer.Init(f, src, func(token.Position, string) {}, scanner.ScanComments)
	lastByte, lastRune := 0, 0
	var result []Token
	for {
		pos, tok, lit := lexer.Scan()
		if tok == token.EOF {
			return result
		}
		at := f.Offset(pos)
		kind := roles[at]
		switch {
		case tok.IsKeyword():
			kind = "keyword"
		case tok == token.STRING || tok == token.CHAR:
			kind = "string"
		case tok == token.COMMENT:
			kind = "comment"
		case tok.IsLiteral() && tok != token.IDENT:
			kind = "number"
		case tok == token.IDENT && strings.Contains("|nil|true|false|iota|", "|"+lit+"|"):
			kind = "constant"
		case tok == token.IDENT && strings.Contains("|int|string|bool|byte|rune|error|any|uint|float64|float32|int64|uint64|uintptr|", "|"+lit+"|"):
			kind = "type"
		}
		if kind == "" || at < lastByte {
			continue
		}
		if lit == "" {
			lit = tok.String()
		}
		endByte := at + len(lit)
		if endByte > len(src) {
			continue
		}
		start := lastRune + utf8.RuneCount(src[lastByte:at])
		end := start + utf8.RuneCount(src[at:endByte])
		result = append(result, Token{start, end, kind})
		lastByte, lastRune = endByte, end
	}
}
