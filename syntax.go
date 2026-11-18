package main

import (
	"go/ast"
	"go/parser"
	"go/scanner"
	"go/token"
	"strings"
	"unicode/utf8"
)

func highlightSyntax(b *buffer, lang string) []highlight {
	if len(b.text) > 400000 {
		return nil
	}
	switch lang {
	case "go":
		return highlightGo(b)
	}
	if _, ok := syntaxSpecs[lang]; ok {
		return highlightGeneric(b.text, lang)
	}
	return nil
}

func syntaxHighlight(start, end int, hex int32) highlight {
	return highlight{span{start, end}, baseStyle.Foreground(rgb(hex))}
}

func highlightGo(b *buffer) []highlight {
	src := []byte(string(b.text))
	parsed := token.NewFileSet()
	tree, _ := parser.ParseFile(parsed, b.path, src, parser.SkipObjectResolution)
	roles := map[int]int32{}
	mark := func(id *ast.Ident, kind string) {
		if id != nil {
			roles[parsed.Position(id.Pos()).Offset] = semanticColor(kind)
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
				if roles[parsed.Position(n.Sel.Pos()).Offset] == 0 {
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
	f := token.NewFileSet().AddFile(b.path, -1, len(src))
	var lexer scanner.Scanner
	lexer.Init(f, src, func(token.Position, string) {}, scanner.ScanComments)
	lastByte, lastRune := 0, 0
	var result []highlight
	for {
		pos, tok, lit := lexer.Scan()
		if tok == token.EOF {
			return result
		}
		at := f.Offset(pos)
		hex := roles[at]
		switch {
		case tok.IsKeyword():
			hex = semanticColor("keyword")
		case tok == token.STRING || tok == token.CHAR:
			hex = semanticColor("string")
		case tok == token.COMMENT:
			hex = semanticColor("comment")
		case tok.IsLiteral() && tok != token.IDENT:
			hex = semanticColor("number")
		case tok == token.IDENT && strings.Contains("|nil|true|false|iota|", "|"+lit+"|"):
			hex = semanticColor("constant")
		case tok == token.IDENT && strings.Contains("|int|string|bool|byte|rune|error|any|uint|float64|float32|int64|uint64|uintptr|", "|"+lit+"|"):
			hex = semanticColor("type")
		}
		if hex == 0 || at < lastByte {
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
		result = append(result, syntaxHighlight(start, end, hex))
		lastByte, lastRune = endByte, end
	}
}
