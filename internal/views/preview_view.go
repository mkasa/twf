package views

import (
	"fmt"
	"math"
	"os"
	"os/exec"
	"strings"

	"github.com/wvanlint/twf/internal/config"
	"github.com/wvanlint/twf/internal/state"
	term "github.com/wvanlint/twf/internal/terminal"
)

type previewView struct {
	config      *config.TwfConfig
	state       *state.State
	lastPath    string
	lastCols    int
	lastRows    int
	lastPreview []string
	lastRaw     bool
	scroll      int
	noLines     int
}

func NewPreviewView(config *config.TwfConfig, state *state.State) term.View {
	return &previewView{
		config: config,
		state:  state,
	}
}

func (v *previewView) Position(totalRows int, totalCols int) term.Position {
	return term.Position{
		Top:  1,
		Left: int(math.Ceil(float64(totalCols)/2.0)) + 1,
		Rows: totalRows - 1,
		Cols: int(math.Floor(float64(totalCols) / 2.0)),
	}
}

func (v *previewView) HasBorder() bool {
	return true
}

func (v *previewView) ShouldRender() bool {
	return v.config.Preview.Enabled && !v.state.Cursor.IsDir()
}

func (v *previewView) Render(p term.Position) []term.Line {
	if v.lastPath != v.state.Cursor.AbsPath || v.lastCols != p.Cols || v.lastRows != p.Rows {
		if v.lastPath != v.state.Cursor.AbsPath {
			v.scroll = 0
		}
		v.lastPath = v.state.Cursor.AbsPath
		v.lastCols = p.Cols
		v.lastRows = p.Rows

		preview, err := getPreview(v.config.Preview.PreviewCommand, v.state.Cursor.AbsPath, p.Cols, p.Rows)
		if err != nil {
			preview = err.Error()
		}
		v.lastRaw = hasGraphicsPrefix(preview)
		if !v.lastRaw {
			preview = strings.ReplaceAll(preview, "\t", "    ")
		}
		v.lastPreview = strings.Split(preview, "\n")
	}

	lines := v.lastPreview
	if v.scroll > len(lines)-p.Rows {
		if len(lines) < p.Rows {
			v.scroll = 0
		} else {
			v.scroll = len(lines) - p.Rows
		}
	}

	termLines := []term.Line{}
	for i := v.scroll; i-v.scroll < p.Rows && i < len(lines); i++ {
		termLine := term.NewLine(&term.Graphics{}, p.Cols)
		if v.lastRaw {
			termLine.AppendPassthrough(lines[i])
		} else {
			termLine.AppendRaw(lines[i])
		}
		termLines = append(termLines, termLine)
	}
	return termLines
}

// hasGraphicsPrefix reports whether the preview output starts with a terminal
// graphics escape: kitty graphics (APC G), iTerm2 inline image (OSC 1337), or
// sixel (DCS). Such output must be passed through without escape filtering.
func hasGraphicsPrefix(s string) bool {
	return strings.HasPrefix(s, "\x1b_G") ||
		strings.HasPrefix(s, "\x1b]1337;") ||
		strings.HasPrefix(s, "\x1bP")
}

func getPreview(cmdTemplate string, path string, cols int, rows int) (string, error) {
	escapedPath := "\"" + strings.ReplaceAll(path, "\"", "\\\"") + "\""
	cmd := strings.ReplaceAll(cmdTemplate, "{}", escapedPath)
	var stdout, stderr strings.Builder
	preview := exec.Command("bash", "-c", cmd)
	preview.Env = append(os.Environ(),
		fmt.Sprintf("TWF_PREVIEW_COLUMNS=%d", cols),
		fmt.Sprintf("TWF_PREVIEW_LINES=%d", rows),
	)
	preview.Stdout = &stdout
	preview.Stderr = &stderr
	err := preview.Run()
	if err != nil {
		return stdout.String(), fmt.Errorf("%w %s", err, stderr.String())
	} else {
		return stdout.String(), nil
	}
}

func (v *previewView) GetCommands() map[string]term.Command {
	return map[string]term.Command{
		"preview:down": v.down,
		"preview:up":   v.up,
	}
}

func (v *previewView) up(helper term.TerminalHelper, args ...interface{}) error {
	if v.scroll > 0 {
		v.scroll -= 1
	}
	return nil
}

func (v *previewView) down(helper term.TerminalHelper, args ...interface{}) error {
	v.scroll += 1
	return nil
}
