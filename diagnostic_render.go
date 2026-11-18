package main

type diagnosticMark struct {
	span
	severity int
}

func diagnosticRows(b *buffer, top, bottom int) map[int][]diagnosticMark {
	if len(b.diagnostics) == 0 {
		return nil
	}
	rows := make(map[int][]diagnosticMark)
	for _, d := range b.diagnostics {
		r := d.Range
		if r.Start.Line >= bottom || r.End.Line < top {
			continue
		}
		start, startOK := b.offset(r.Start)
		end, endOK := b.offset(r.End)
		if !startOK || !endOK || start > end {
			continue
		}
		// An empty range still marks one cell, including at end of line.
		if start == end {
			end++
		}
		severity := max(1, d.Severity)
		for row := max(top, r.Start.Line); row < min(bottom, r.End.Line+1); row++ {
			s := span{max(start, b.lines[row]), min(end, b.lineEnd(row)+1)}
			if s.start < s.end {
				rows[row] = append(rows[row], diagnosticMark{s, severity})
			}
		}
	}
	return rows
}

func diagnosticAt(marks []diagnosticMark, start, end int) int {
	severity := 0
	for _, mark := range marks {
		if start < mark.end && end > mark.start && (severity == 0 || mark.severity < severity) {
			severity = mark.severity
		}
	}
	return severity
}

func diagnosticColor(severity int) terminalColor {
	switch severity {
	case 1:
		return th.red
	case 2:
		return th.yellow
	default:
		return th.aqua
	}
}

func markDiagnostic(style cellStyle, marks []diagnosticMark, start, end int) cellStyle {
	if severity := diagnosticAt(marks, start, end); severity != 0 {
		style.underline = diagnosticColor(severity)
	}
	return style
}
