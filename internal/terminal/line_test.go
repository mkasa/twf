package terminal

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLineAppend(t *testing.T) {
	line := NewLine(
		&Graphics{Reverse: true},
		2,
	)
	line.Append("b", &Graphics{Bold: true})
	assert.Equal(t, 1, line.Length())
	line.Append("la", &Graphics{})
	assert.Equal(t, 2, line.Length())
	assert.Equal(t, "\x1b[1mb\x1b[m\x1b[7m\x1b[ml\x1b[m\x1b[7m", line.Text())
}

func TestLineAppendRaw(t *testing.T) {
	line := NewLine(
		&Graphics{Reverse: true},
		2,
	)

	line.AppendRaw("\x1b[44mabc")
	assert.Equal(t, 2, line.Length())
	assert.Equal(t, "\x1b[44mab\x1b[m\x1b[7m", line.Text())
}

func TestLineAppendPassthrough(t *testing.T) {
	line := NewLine(
		&Graphics{Reverse: true},
		2,
	)

	// A kitty graphics APC escape passes through with zero width, and
	// placeholder runes keep their combining diacritics.
	line.AppendPassthrough("\x1b_Ga=T,i=1;QUJD\x1b\\\x1b[38;2;0;0;1m\U0010EEEE̅̅\U0010EEEE̅̍\U0010EEEE̅̎")
	assert.Equal(t, 2, line.Length())
	assert.Equal(
		t,
		"\x1b_Ga=T,i=1;QUJD\x1b\\\x1b[38;2;0;0;1m\U0010EEEE̅̅\U0010EEEE̅̍\x1b[m\x1b[7m",
		line.Text(),
	)
}

func TestEscapeSequenceLength(t *testing.T) {
	assert.Equal(t, 5, escapeSequenceLength("\x1b[44mabc"))
	assert.Equal(t, 7, escapeSequenceLength("\x1b]0;ab\x07c"))
	assert.Equal(t, 9, escapeSequenceLength("\x1b_Gm=0;\x1b\\x"))
	assert.Equal(t, 2, escapeSequenceLength("\x1b7abc"))
	assert.Equal(t, 4, escapeSequenceLength("\x1b_Gm"))
}

func TestLineNarrowUnicode(t *testing.T) {
	line := NewLine(
		&Graphics{Reverse: true},
		2,
	)

	line.Append("›", nil)
	assert.Equal(t, 1, line.Length())
}

func TestLineWideUnicode(t *testing.T) {
	line := NewLine(
		&Graphics{Reverse: true},
		3,
	)

	line.Append("😊", nil)
	assert.Equal(t, 2, line.Length())
	line.Append("😊", nil)
	assert.Equal(t, 2, line.Length())
}
