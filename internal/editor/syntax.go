package editor

import "github.com/zhuchops/loom/internal/syntax"

// highlightSyntax turns syntax tokens into styled spans using the active theme.
func highlightSyntax(b *buffer, lang string) []highlight {
	var result []highlight
	for _, t := range syntax.Highlight(lang, b.path, b.text) {
		if hex := semanticColor(t.Kind); hex != 0 {
			result = append(result, highlight{span{t.Start, t.End}, baseStyle.Foreground(rgb(hex))})
		}
	}
	return result
}
