package editor

import "github.com/zhuchops/loom/internal/syntax"

// highlightSyntax turns syntax tokens into styled spans using the active theme.
func highlightSyntax(b *buffer, lang string) []highlight {
	var result []highlight
	for _, t := range syntax.Highlight(lang, b.path, b.text) {
		if hex := semanticColor(t.Kind); hex != 0 {
			result = append(result, highlight{span{t.Start, t.End}, tokenStyle(t.Kind, hex)})
		}
	}
	return result
}

// italicComments follows the italic_comments setting.
var italicComments bool

func tokenStyle(kind string, hex int32) cellStyle {
	style := baseStyle.Foreground(rgb(hex))
	style.italic = kind == "comment" && italicComments
	return style
}
