package views

import (
	"github.com/wvanlint/twf/internal/config"
	"github.com/wvanlint/twf/internal/state"
	term "github.com/wvanlint/twf/internal/terminal"
)

type statusView struct {
	config *config.TwfConfig
	state  *state.State

	// Filter prompt.
	editing bool
	input   []rune
	message string
}

func NewStatusView(config *config.TwfConfig, state *state.State) term.View {
	return &statusView{
		config: config,
		state:  state,
	}
}

func (v *statusView) Position(totalRows int, totalCols int) term.Position {
	return term.Position{
		Top:  totalRows,
		Left: 1,
		Rows: 1,
		Cols: totalCols,
	}
}

func (v *statusView) HasBorder() bool {
	return false
}

func (v *statusView) ShouldRender() bool {
	return true
}

func (v *statusView) Render(p term.Position) []term.Line {
	line := term.NewLine(&term.Graphics{}, p.Cols)
	if v.editing {
		line.Append("Filter: "+string(v.input), nil)
		line.Append(" ", &term.Graphics{Reverse: true})
		if v.message != "" {
			line.Append("  "+v.message, nil)
		} else {
			line.Append("  (enter: apply, empty: show all, esc: cancel)", nil)
		}
	} else if filter := v.state.Root.Filter(); filter != "" {
		line.Append("Filter: "+filter, nil)
	} else {
		line.Append("", &term.Graphics{})
	}
	return []term.Line{line}
}

func (v *statusView) GetCommands() map[string]term.Command {
	return map[string]term.Command{
		"filter:edit": v.editFilter,
	}
}

// editFilter opens the filter prompt, starting from the current filter so
// that a mistyped one can be corrected.
func (v *statusView) editFilter(helper term.TerminalHelper, args ...interface{}) error {
	v.editing = true
	v.input = []rune(v.state.Root.Filter())
	v.message = ""
	return nil
}

func (v *statusView) CaptureEvent(event term.Event) bool {
	if !v.editing {
		return false
	}
	v.message = ""
	switch event.Symbol {
	case term.Rune:
		v.input = append(v.input, event.Value)
	case term.Del, term.CtrlH:
		if len(v.input) > 0 {
			v.input = v.input[:len(v.input)-1]
		}
	case term.CtrlU:
		v.input = nil
	case term.Enter:
		if err := v.state.SetFilter(string(v.input)); err != nil {
			v.message = err.Error()
		} else {
			v.editing = false
		}
	case term.Escape, term.CtrlC:
		v.editing = false
	}
	return true
}
