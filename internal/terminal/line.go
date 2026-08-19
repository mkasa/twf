package terminal

import (
	"golang.org/x/text/width"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var escapeRegex, graphicsEscapeRegex *regexp.Regexp

func init() {
	escapeRegex = regexp.MustCompile("\x1b\\[[0-9;]*[a-zA-Z]")
	graphicsEscapeRegex = regexp.MustCompile("\x1b\\[[0-9;]*m")
}

type Line interface {
	Append(string, *Graphics) Line
	AppendRaw(string) Line
	AppendPassthrough(string) Line
	Length() int
	Text() string
}

type line struct {
	line            strings.Builder
	length          int
	maxLength       int
	defaultGraphics *Graphics
}

func NewLine(defaultGraphics *Graphics, maxLength int) Line {
	return &line{defaultGraphics: defaultGraphics, maxLength: maxLength}
}

func (l *line) appendText(s string) {
	for len(s) > 0 && l.length < l.maxLength {
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if r == utf8.RuneError || unicode.IsMark(r) || unicode.IsControl(r) {
			continue
		}
		termWidth := 1
		runeKind := width.LookupRune(r).Kind()
		if runeKind == width.EastAsianWide || runeKind == width.EastAsianFullwidth {
			termWidth = 2
		}
		if l.length+termWidth <= l.maxLength {
			l.length += termWidth
		} else {
			break
		}
		l.line.WriteString(string(r))
	}
}

func (l *line) Append(s string, graphics *Graphics) Line {
	if l.length >= l.maxLength {
		return l
	}
	if graphics != nil {
		l.line.WriteString(graphics.ToEscapeCode())
	}
	l.appendText(s)
	l.line.WriteString(resetGraphics)
	l.line.WriteString(l.defaultGraphics.ToEscapeCode())
	return l
}

func (l *line) AppendRaw(s string) Line {
	matches := escapeRegex.FindAllStringIndex(s, -1)
	prevIndex := 0
	for i := 0; i < len(matches)+1; i++ {
		piece := ""
		if i < len(matches) {
			piece = s[prevIndex:matches[i][0]]
		} else {
			piece = s[prevIndex:]
		}
		l.appendText(piece)
		if i < len(matches) {
			escapeCode := s[matches[i][0]:matches[i][1]]
			if graphicsEscapeRegex.MatchString(escapeCode) {
				l.line.WriteString(escapeCode)
			}
			prevIndex = matches[i][1]
		}
	}
	l.line.WriteString(resetGraphics)
	l.line.WriteString(l.defaultGraphics.ToEscapeCode())
	return l
}

// AppendPassthrough appends s while keeping all escape sequences (not just
// graphics ones) and combining marks intact, so terminal image protocols
// (e.g. kitty graphics placeholders emitted by imgcat3) survive rendering.
// Escape sequences contribute zero width; visible runes past the maximum
// length are dropped together with their combining marks.
func (l *line) AppendPassthrough(s string) Line {
	keptLastRune := false
	for len(s) > 0 {
		if s[0] == 0x1b {
			seq := escapeSequenceLength(s)
			l.line.WriteString(s[:seq])
			s = s[seq:]
			continue
		}
		r, size := utf8.DecodeRuneInString(s)
		s = s[size:]
		if r == utf8.RuneError && size == 1 {
			continue
		}
		if unicode.IsMark(r) {
			if keptLastRune {
				l.line.WriteRune(r)
			}
			continue
		}
		if unicode.IsControl(r) {
			continue
		}
		termWidth := 1
		runeKind := width.LookupRune(r).Kind()
		if runeKind == width.EastAsianWide || runeKind == width.EastAsianFullwidth {
			termWidth = 2
		}
		if l.length+termWidth > l.maxLength {
			keptLastRune = false
			continue
		}
		l.length += termWidth
		l.line.WriteRune(r)
		keptLastRune = true
	}
	l.line.WriteString(resetGraphics)
	l.line.WriteString(l.defaultGraphics.ToEscapeCode())
	return l
}

// escapeSequenceLength returns the byte length of the escape sequence
// starting at s[0] == ESC, including its terminator.
func escapeSequenceLength(s string) int {
	if len(s) < 2 {
		return 1
	}
	switch s[1] {
	case '[': // CSI: parameter/intermediate bytes, then a final byte in 0x40-0x7e.
		i := 2
		for i < len(s) && s[i] >= 0x20 && s[i] <= 0x3f {
			i++
		}
		if i < len(s) {
			i++
		}
		return i
	case ']', 'P', '_', '^', 'X': // OSC/DCS/APC/PM/SOS: run to ST (ESC \), or BEL for OSC.
		for i := 2; i < len(s); i++ {
			if s[i] == 0x07 && s[1] == ']' {
				return i + 1
			}
			if s[i] == 0x1b && i+1 < len(s) && s[i+1] == '\\' {
				return i + 2
			}
		}
		return len(s)
	default:
		return 2
	}
}

func (l *line) Length() int {
	return l.length
}

func (l *line) Text() string {
	return l.line.String()
}
