package terminal

type View interface {
	Position(int, int) Position
	HasBorder() bool
	ShouldRender() bool
	Render(Position) []Line
	GetCommands() map[string]Command
}

// EventCapturer can be implemented by views that need raw input, such as a
// text prompt. Events for which CaptureEvent returns true are not looked up
// in the keybindings.
type EventCapturer interface {
	CaptureEvent(Event) bool
}

type Command func(helper TerminalHelper, args ...interface{}) error

type TerminalHelper interface {
	ExecuteInTerminal(string) (string, error)
}

type Position struct {
	Top  int
	Left int

	Rows int
	Cols int
}

func (p *Position) Shrink(i int) Position {
	if p.Rows <= 0 || p.Cols <= 0 {
		return *p
	}
	return Position{
		Top:  p.Top + i,
		Left: p.Left + i,
		Rows: p.Rows - 2*i,
		Cols: p.Cols - 2*i,
	}
}
