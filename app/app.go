package app

import (
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/charmbracelet/bubbles/key"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tinydarkforge/intake/types"
)

// Root is the top-level Bubble Tea model.  It owns screen routing and the NC
// two-pane layout.
type Root struct {
	screen     Screen
	activePane int // 0 = left (local), 1 = right (issues)
	width      int
	height     int
	status     string
	statusOK   bool
	statusAt   time.Time
	Cfg        types.Config // exported so main.go can set it before first render

	Local    tea.Model // left pane — LocalPane
	List     tea.Model // right pane — ListModel
	Create   tea.Model
	Detail   tea.Model
	Settings tea.Model
}

const clearDelay = 3 * time.Second

func New() Root {
	return Root{screen: ScreenNC, activePane: 1}
}

func (r Root) ActiveScreen() Screen { return r.screen }

func (r Root) Init() tea.Cmd {
	var cmds []tea.Cmd
	for _, m := range []tea.Model{r.Local, r.List, r.Create, r.Detail, r.Settings} {
		if m != nil {
			cmds = append(cmds, m.Init())
		}
	}
	return tea.Batch(cmds...)
}

// KeyInterceptor is implemented by screens that want all keystrokes when an
// input is focused, preventing global shortcuts from firing.
type KeyInterceptor interface {
	InterceptsKeys() bool
}

type tickClearMsg struct{}

func clearAfter(d time.Duration) tea.Cmd {
	return tea.Tick(d, func(_ time.Time) tea.Msg { return tickClearMsg{} })
}

func (r Root) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		r.width, r.height = msg.Width, msg.Height
		for _, m := range []*tea.Model{&r.Local, &r.List, &r.Create, &r.Detail, &r.Settings} {
			if *m != nil {
				updated, cmd := (*m).Update(msg)
				*m = updated
				cmds = append(cmds, cmd)
			}
		}

	case tea.KeyMsg:
		// Full-screen overlays that have a text input focused get all keys
		// except ctrl+c.
		if r.currentScreenInterceptsKeys() && !key.Matches(msg, Global.Quit) {
			var cmd tea.Cmd
			r, cmd = r.updateCurrent(msg)
			cmds = append(cmds, cmd)
			return r, tea.Batch(cmds...)
		}

		switch {
		case key.Matches(msg, Global.Quit):
			return r, tea.Quit

		case key.Matches(msg, Global.Back):
			if r.screen != ScreenNC {
				r.screen = ScreenNC
			}

		case key.Matches(msg, Global.Create):
			if r.screen == ScreenNC {
				r.screen = ScreenCreate

				// Gather context from Local pane if any files are attached.
				type attacher interface{ Attached() map[string]bool }
				type receiver interface{ SetContext(map[string]string) }

				if a, ok := r.Local.(attacher); ok {
					attached := a.Attached()
					if len(attached) > 0 {
						ctxMap := make(map[string]string)
						for path := range attached {
							data, err := os.ReadFile(path)
							if err == nil {
								ctxMap[path] = string(data)
							}
						}
						if rec, ok := r.Create.(receiver); ok {
							rec.SetContext(ctxMap)
						}
					}
				}
			} else {
				var cmd tea.Cmd
				r, cmd = r.updateCurrent(msg)
				cmds = append(cmds, cmd)
			}

		case key.Matches(msg, Global.Settings):
			if r.screen == ScreenNC {
				r.screen = ScreenSettings
			} else {
				var cmd tea.Cmd
				r, cmd = r.updateCurrent(msg)
				cmds = append(cmds, cmd)
			}

		default:
			if r.screen == ScreenNC {
				cmds = append(cmds, r.updateNC(msg, &r)...)
			} else {
				var cmd tea.Cmd
				r, cmd = r.updateCurrent(msg)
				cmds = append(cmds, cmd)
			}
		}

	case NavigateMsg:
		r.screen = msg.Screen

	case OpenIssueMsg:
		r.screen = ScreenDetail
		if r.Detail != nil {
			m, cmd := r.Detail.Update(msg)
			r.Detail = m
			cmds = append(cmds, cmd)
		}

	case BackMsg:
		r.screen = ScreenNC

	case StatusMsg:
		r.status = msg.Text
		r.statusOK = !msg.IsError
		r.statusAt = time.Now()
		cmds = append(cmds, clearAfter(clearDelay))

	case ErrMsg:
		r.status = msg.Err.Error()
		r.statusOK = false
		r.statusAt = time.Now()
		cmds = append(cmds, clearAfter(clearDelay))
		// Also forward to panes so they can update their own state.
		if r.screen == ScreenNC {
			for _, mp := range []*tea.Model{&r.Local, &r.List} {
				if *mp != nil {
					updated, cmd := (*mp).Update(msg)
					*mp = updated
					cmds = append(cmds, cmd)
				}
			}
		}

	case tickClearMsg:
		if time.Since(r.statusAt) >= clearDelay {
			r.status = ""
		}

	case tea.MouseMsg:
		if r.screen == ScreenNC {
			cmds = append(cmds, r.handleNCMouse(msg)...)
		}

	default:
		if r.screen == ScreenNC {
			// Forward async messages (IssuesLoadedMsg, ErrMsg, etc.) to both panes.
			for _, mp := range []*tea.Model{&r.Local, &r.List} {
				if *mp != nil {
					updated, cmd := (*mp).Update(msg)
					*mp = updated
					cmds = append(cmds, cmd)
				}
			}
		} else {
			var cmd tea.Cmd
			r, cmd = r.updateCurrent(msg)
			cmds = append(cmds, cmd)
		}
	}

	return r, tea.Batch(cmds...)
}

// updateNC handles key events in ScreenNC mode.
func (r *Root) updateNC(msg tea.KeyMsg, out *Root) []tea.Cmd {
	var cmds []tea.Cmd
	switch msg.String() {
	case "tab", "right", "left":
		out.activePane = 1 - out.activePane
	default:
		// Route to active pane.
		if out.activePane == 0 && out.Local != nil {
			m, cmd := out.Local.Update(msg)
			out.Local = m
			cmds = append(cmds, cmd)
		} else if out.activePane == 1 && out.List != nil {
			m, cmd := out.List.Update(msg)
			out.List = m
			cmds = append(cmds, cmd)
			// Check if List fired an OpenIssueMsg by watching for it via Cmd.
			// The Cmd will be processed in the next Update cycle naturally.
		}
	}
	return cmds
}

func (r Root) currentScreenInterceptsKeys() bool {
	var m tea.Model
	switch r.screen {
	case ScreenNC:
		return false // NC panes do their own key handling
	case ScreenCreate:
		m = r.Create
	case ScreenDetail:
		m = r.Detail
	case ScreenSettings:
		m = r.Settings
	}
	if ki, ok := m.(KeyInterceptor); ok {
		return ki.InterceptsKeys()
	}
	return false
}

func (r Root) updateCurrent(msg tea.Msg) (Root, tea.Cmd) {
	var (
		m   tea.Model
		cmd tea.Cmd
	)
	switch r.screen {
	case ScreenNC:
		// NC pane routing is handled separately; nothing to do here.
	case ScreenCreate:
		if r.Create != nil {
			m, cmd = r.Create.Update(msg)
			r.Create = m
		}
	case ScreenDetail:
		if r.Detail != nil {
			m, cmd = r.Detail.Update(msg)
			r.Detail = m
		}
	case ScreenSettings:
		if r.Settings != nil {
			m, cmd = r.Settings.Update(msg)
			r.Settings = m
		}
	}
	return r, cmd
}

// ---- View ----------------------------------------------------------------------

func (r Root) View() string {
	if r.width == 0 {
		return "loading…"
	}
	switch r.screen {
	case ScreenCreate:
		if r.Create != nil {
			return r.renderFullScreen(r.Create.View())
		}
	case ScreenDetail:
		if r.Detail != nil {
			return r.renderFullScreen(r.Detail.View())
		}
	case ScreenSettings:
		if r.Settings != nil {
			return r.renderFullScreen(r.Settings.View())
		}
	}
	return r.renderNCFrame()
}

// renderFullScreen wraps an overlay content string inside the outer NC border.
func (r Root) renderFullScreen(content string) string {
	W := r.width
	H := r.height

	// ─── top bar ───────────────────────────────────────────────────────
	topBar := r.renderTopBar(W)

	// ─── inner height ──────────────────────────────────────────────────
	// rows consumed: topBar(1) + topBar-border(2) + fnBar(1) + fnBar-border(1)
	innerH := H - 5
	if innerH < 1 {
		innerH = 1
	}
	innerW := W - 2 // exclude ║ ║

	bStyle := lipgloss.NewStyle().Foreground(NCBorder)

	// Pad/trim content lines.
	lines := strings.Split(content, "\n")
	var rows []string
	for _, l := range lines {
		if len(rows) >= innerH {
			break
		}
		lw := lipgloss.Width(l)
		if lw < innerW {
			l = l + strings.Repeat(" ", innerW-lw)
		}
		rows = append(rows, bStyle.Render("║")+l+bStyle.Render("║"))
	}
	for len(rows) < innerH {
		rows = append(rows, bStyle.Render("║")+strings.Repeat(" ", innerW)+bStyle.Render("║"))
	}

	// ─── fn bar ────────────────────────────────────────────────────────
	fnBarRow := fnBar(W, r.screen, r.activePane)

	divider := bStyle.Render("╠" + strings.Repeat("═", W-2) + "╣")
	bottom := bStyle.Render("╚" + strings.Repeat("═", W-2) + "╝")

	var sb strings.Builder
	sb.WriteString(topBar)
	sb.WriteByte('\n')
	sb.WriteString(divider)
	sb.WriteByte('\n')
	for _, row := range rows {
		sb.WriteString(row)
		sb.WriteByte('\n')
	}
	sb.WriteString(divider)
	sb.WriteByte('\n')
	sb.WriteString(fnBarRow)
	sb.WriteByte('\n')
	sb.WriteString(bottom)
	return sb.String()
}

// renderNCFrame draws the full two-pane NC UI.
func (r Root) renderNCFrame() string {
	W := r.width
	H := r.height

	// Inner widths (each excludes its own ║ characters).
	// Frame chars per row: left ║ + divider ║ + right ║ = 3 chars.
	leftW := (W - 3) / 2
	rightW := W - 3 - leftW

	// Rows consumed: top border(1) + header(1) + header border(1) +
	//   col-header(1) + col-header border(1) + bottom panel border(1) +
	//   status(1) + status border(1) + fn bar(1) + fn bar border(1) = 10
	// (But we reuse the bottom panel border as top status border.)
	panelH := H - 9
	if panelH < 1 {
		panelH = 1
	}

	// ─── content lines from each pane ──────────────────────────────────
	leftLines := getLinesFromModel(r.Local, leftW, panelH)
	rightLines := getLinesFromModel(r.List, rightW, panelH)
	leftLines = padLinesToHeight(leftLines, panelH, leftW)
	rightLines = padLinesToHeight(rightLines, panelH, rightW)

	// ─── styles ────────────────────────────────────────────────────────
	activeBorder := lipgloss.NewStyle().Foreground(NCBorder)
	dimBorder := lipgloss.NewStyle().Foreground(NCBorderDim)

	leftActive := r.activePane == 0
	rightActive := r.activePane == 1

	leftBorderStyle := dimBorder
	rightBorderStyle := dimBorder
	if leftActive {
		leftBorderStyle = activeBorder
	}
	if rightActive {
		rightBorderStyle = activeBorder
	}

	leftTitleBg := NCTitleDim
	rightTitleBg := NCTitleDim
	if leftActive {
		leftTitleBg = NCTitleActive
	}
	if rightActive {
		rightTitleBg = NCTitleActive
	}

	// ─── top border ────────────────────────────────────────────────────
	// ╔══════════════════════════════════════════════════════════════════╗
	topBar := r.renderTopBar(W)

	// ─── panel title row ───────────────────────────────────────────────
	// ╠══════════════════╦═══════════════════════════════════════════════╣
	// ║ Local            ║ GitHub Issues (open — N)                     ║
	titleSep := activeBorder.Render("╠") +
		leftBorderStyle.Render(strings.Repeat("═", leftW)) +
		activeBorder.Render("╦") +
		rightBorderStyle.Render(strings.Repeat("═", rightW)) +
		activeBorder.Render("╣")

	leftTitleContent := titleText("Local", leftW, leftTitleBg)
	rightTitleContent := titleText(r.issuesTitle(), rightW, rightTitleBg)
	titleRow := leftBorderStyle.Render("║") + leftTitleContent +
		activeBorder.Render("║") + rightTitleContent +
		rightBorderStyle.Render("║")

	// ─── column header row ─────────────────────────────────────────────
	// ╠══════════════════╬═══════════════════════════════════════════════╣
	// ║ Name       Size  ║  #      Title                      Labels    ║
	colHdrSep := activeBorder.Render("╠") +
		leftBorderStyle.Render(strings.Repeat("═", leftW)) +
		activeBorder.Render("╬") +
		rightBorderStyle.Render(strings.Repeat("═", rightW)) +
		activeBorder.Render("╣")

	leftColHdr := colHeader(fmt.Sprintf("%-*s%s", leftW-8, "Name", "Size    "), leftW)
	rightColHdr := colHeader(fmt.Sprintf("%-7s%-*s%s", "#", rightW-18, "Title", "Labels    "), rightW)
	colHdrRow := leftBorderStyle.Render("║") + leftColHdr +
		activeBorder.Render("║") + rightColHdr +
		rightBorderStyle.Render("║")

	colHdrSep2 := activeBorder.Render("╠") +
		leftBorderStyle.Render(strings.Repeat("═", leftW)) +
		activeBorder.Render("╬") +
		rightBorderStyle.Render(strings.Repeat("═", rightW)) +
		activeBorder.Render("╣")

	// ─── panel content rows ────────────────────────────────────────────
	var contentRows strings.Builder
	for i := 0; i < panelH; i++ {
		lLine := leftLines[i]
		rLine := rightLines[i]
		row := leftBorderStyle.Render("║") + lLine +
			activeBorder.Render("║") + rLine +
			rightBorderStyle.Render("║")
		contentRows.WriteString(row)
		contentRows.WriteByte('\n')
	}

	// ─── status row ────────────────────────────────────────────────────
	// ╠══════════════════╩═══════════════════════════════════════════════╣
	// ║ /path/to/dir  ◆  issue info                                     ║
	statusSep := activeBorder.Render("╠") +
		leftBorderStyle.Render(strings.Repeat("═", leftW)) +
		activeBorder.Render("╩") +
		rightBorderStyle.Render(strings.Repeat("═", rightW)) +
		activeBorder.Render("╣")

	statusContent := r.buildStatusContent(W - 2)
	statusRow := activeBorder.Render("║") + statusContent + activeBorder.Render("║")

	// ─── fn bar ────────────────────────────────────────────────────────
	fnBarSep := activeBorder.Render("╠" + strings.Repeat("═", W-2) + "╣")
	fnBarRow := fnBar(W, r.screen, r.activePane)
	bottom := activeBorder.Render("╚" + strings.Repeat("═", W-2) + "╝")

	var sb strings.Builder
	sb.WriteString(topBar)
	sb.WriteByte('\n')
	sb.WriteString(titleSep)
	sb.WriteByte('\n')
	sb.WriteString(titleRow)
	sb.WriteByte('\n')
	sb.WriteString(colHdrSep)
	sb.WriteByte('\n')
	sb.WriteString(colHdrRow)
	sb.WriteByte('\n')
	sb.WriteString(colHdrSep2)
	sb.WriteByte('\n')
	sb.WriteString(contentRows.String())
	sb.WriteString(statusSep)
	sb.WriteByte('\n')
	sb.WriteString(statusRow)
	sb.WriteByte('\n')
	sb.WriteString(fnBarSep)
	sb.WriteByte('\n')
	sb.WriteString(fnBarRow)
	sb.WriteByte('\n')
	sb.WriteString(bottom)
	return sb.String()
}

// renderTopBar renders the ╔...╗ / ║ intake ◆ repo ◆ model ║ top section.
func (r Root) renderTopBar(W int) string {
	bStyle := lipgloss.NewStyle().Foreground(NCBorder)
	topBorder := bStyle.Render("╔" + strings.Repeat("═", W-2) + "╗")

	title := fmt.Sprintf(" intake  %s  %s ", r.Cfg.Repo, r.Cfg.Model)
	tw := lipgloss.Width(title)
	if tw < W-2 {
		title = title + strings.Repeat(" ", W-2-tw)
	} else if tw > W-2 {
		rr := []rune(title)
		for lipgloss.Width(string(rr)) >= W-2 && len(rr) > 0 {
			rr = rr[:len(rr)-1]
		}
		title = string(rr)
	}

	titleRow := bStyle.Render("║") +
		lipgloss.NewStyle().
			Background(NCBg).
			Foreground(NCStatusFg).
			Bold(true).
			Render(title) +
		bStyle.Render("║")

	return topBorder + "\n" + titleRow
}

// issuesTitle builds the right panel header text.
func (r Root) issuesTitle() string {
	// Attempt to read state from the ListModel via type assertion.
	type stater interface{ State() string }
	type counter interface{ Count() int }

	state := "open"
	count := 0
	if lm, ok := r.List.(stater); ok {
		state = lm.State()
	}
	if lm, ok := r.List.(counter); ok {
		count = lm.Count()
	}
	return fmt.Sprintf("GitHub Issues (%s — %d)", state, count)
}

// buildStatusContent renders the single status bar row content (no borders),
// exactly w visible characters wide.
func (r Root) buildStatusContent(w int) string {
	// Collect the current directory from the local pane if it exposes Dir().
	type direr interface{ Dir() string }
	dir := ""
	if r.Local != nil {
		if d, ok := r.Local.(direr); ok {
			dir = d.Dir()
		}
	}

	issueInfo := r.selectedIssueText()

	var text string
	if r.status != "" {
		if r.statusOK {
			text = " ✓ " + r.status
		} else {
			text = " ✗ " + r.status
		}
	} else {
		var parts []string
		if dir != "" {
			parts = append(parts, dir)
		}
		if issueInfo != "" {
			parts = append(parts, issueInfo)
		}
		if len(parts) > 0 {
			text = " " + strings.Join(parts, "  ◆  ")
		} else {
			text = " "
		}
	}

	tw := lipgloss.Width(text)
	if tw < w {
		text = text + strings.Repeat(" ", w-tw)
	} else if tw > w {
		rr := []rune(text)
		for lipgloss.Width(string(rr)) >= w && len(rr) > 0 {
			rr = rr[:len(rr)-1]
		}
		text = string(rr)
	}

	return lipgloss.NewStyle().
		Background(NCStatus).
		Foreground(NCStatusFg).
		Render(text)
}

// selectedIssueText returns a short description of the highlighted issue if
// the List model exposes SelectedIssueText().
func (r Root) selectedIssueText() string {
	type texter interface{ SelectedIssueText() string }
	if r.List != nil {
		if t, ok := r.List.(texter); ok {
			return t.SelectedIssueText()
		}
	}
	return ""
}

// titleText renders the text inside a panel title bar, padded to w.
func titleText(s string, w int, bg lipgloss.Color) string {
	s = " " + s
	sw := lipgloss.Width(s)
	if sw < w {
		s = s + strings.Repeat(" ", w-sw)
	} else if sw > w {
		rr := []rune(s)
		for lipgloss.Width(string(rr)) >= w && len(rr) > 0 {
			rr = rr[:len(rr)-1]
		}
		s = string(rr)
	}
	return lipgloss.NewStyle().
		Background(bg).
		Foreground(lipgloss.Color("#FFFFFF")).
		Bold(true).
		Render(s)
}

// colHeader renders a column header bar, padded to w.
func colHeader(s string, w int) string {
	s = " " + s
	sw := lipgloss.Width(s)
	if sw < w {
		s = s + strings.Repeat(" ", w-sw)
	} else if sw > w {
		rr := []rune(s)
		for lipgloss.Width(string(rr)) >= w && len(rr) > 0 {
			rr = rr[:len(rr)-1]
		}
		s = string(rr)
	}
	return lipgloss.NewStyle().
		Background(NCColHeader).
		Foreground(lipgloss.Color("#000000")).
		Bold(true).
		Render(s)
}

// fnBar renders the bottom shortcut hint bar.
func fnBar(width int, screen Screen, _ int) string {
	type hint struct{ key, label string }

	var hints []hint
	switch screen {
	case ScreenDetail:
		hints = []hint{
			{"↑/↓", "scroll"}, {"e", "edit"}, {"c", "comment"},
			{"a", "assign"}, {"x", "close"}, {"o", "reopen"}, {"esc", "back"},
		}
	case ScreenCreate:
		hints = []hint{
			{"tab", "next"}, {"shift+tab", "prev"},
			{"ctrl+s", "submit"}, {"esc", "cancel"},
		}
	case ScreenSettings:
		hints = []hint{
			{"tab", "next field"}, {"ctrl+s", "save"}, {"esc", "back"},
		}
	default: // ScreenNC
		hints = []hint{
			{"↑/↓", "nav"}, {"enter", "open"}, {"o", "toggle"},
			{"r", "refresh"}, {"tab", "switch pane"},
			{"a", "attach ctx"}, {"c", "create"}, {"s", "settings"}, {"q", "quit"},
		}
	}

	keyStyle := lipgloss.NewStyle().
		Background(NCFnBg).
		Foreground(NCFnFg).
		Bold(true).
		Padding(0, 1)
	descStyle := lipgloss.NewStyle().
		Background(NCBg).
		Foreground(NCFnDescFg).
		Padding(0, 1)

	var parts []string
	for _, h := range hints {
		parts = append(parts, keyStyle.Render(h.key)+descStyle.Render(h.label))
	}

	bar := strings.Join(parts, "")
	bw := lipgloss.Width(bar)
	inner := width - 2
	if bw < inner {
		bar = bar + lipgloss.NewStyle().Background(NCBg).Render(strings.Repeat(" ", inner-bw))
	} else if bw > inner {
		// Trim hints from the right until it fits.
		for len(parts) > 1 && bw > inner {
			parts = parts[:len(parts)-1]
			bar = strings.Join(parts, "")
			bw = lipgloss.Width(bar)
		}
		if bw < inner {
			bar = bar + lipgloss.NewStyle().Background(NCBg).Render(strings.Repeat(" ", inner-bw))
		}
	}

	bStyle := lipgloss.NewStyle().Foreground(NCBorder)
	return bStyle.Render("║") + bar + bStyle.Render("║")
}

// ---- Mouse handling ------------------------------------------------------------

// contentStartRow is the number of terminal rows above the pane content area in
// the NC frame: topBorder(1) + titleRow(1) + titleSep(1) + titleRow(1) +
// colHdrSep(1) + colHdrRow(1) + colHdrSep2(1) = 7.
const contentStartRow = 7

func (r *Root) handleNCMouse(msg tea.MouseMsg) []tea.Cmd {
	var cmds []tea.Cmd

	leftW := (r.width - 3) / 2

	// Determine which pane the pointer is in by X coordinate.
	inLeft := msg.X >= 1 && msg.X <= leftW
	inRight := msg.X >= leftW+2 && msg.X < r.width-1

	switch msg.Action {
	case tea.MouseActionPress:
		if msg.Button == tea.MouseButtonLeft {
			// Switch active pane.
			if inLeft {
				r.activePane = 0
			} else if inRight {
				r.activePane = 1
			}
			// Move cursor to clicked row.
			contentRow := msg.Y - contentStartRow
			if contentRow >= 0 {
				if inLeft && r.Local != nil {
					m, cmd := r.Local.Update(CursorToMsg{Row: contentRow})
					r.Local = m
					cmds = append(cmds, cmd)
				} else if inRight && r.List != nil {
					m, cmd := r.List.Update(CursorToMsg{Row: contentRow})
					r.List = m
					cmds = append(cmds, cmd)
				}
			}
		}

	case tea.MouseActionRelease:
		// nothing

	case tea.MouseActionMotion:
		// nothing
	}

	// Scroll wheel — navigate active pane regardless of pointer position.
	var syntheticKey tea.KeyType
	if msg.Button == tea.MouseButtonWheelUp {
		syntheticKey = tea.KeyUp
	} else if msg.Button == tea.MouseButtonWheelDown {
		syntheticKey = tea.KeyDown
	}
	if syntheticKey != 0 {
		kmsg := tea.KeyMsg{Type: syntheticKey}
		if r.activePane == 0 && r.Local != nil {
			m, cmd := r.Local.Update(kmsg)
			r.Local = m
			cmds = append(cmds, cmd)
		} else if r.activePane == 1 && r.List != nil {
			m, cmd := r.List.Update(kmsg)
			r.List = m
			cmds = append(cmds, cmd)
		}
	}

	return cmds
}

// ---- NCLiner interface ---------------------------------------------------------

// NCLiner is implemented by pane models that can produce fixed-width content
// lines for the NC frame renderer.
type NCLiner interface {
	NCLines(w, h int) []string
}

// getLinesFromModel calls NCLines if available, otherwise splits View().
func getLinesFromModel(m tea.Model, w, h int) []string {
	if m == nil {
		return nil
	}
	if nl, ok := m.(NCLiner); ok {
		return nl.NCLines(w, h)
	}
	return strings.Split(m.View(), "\n")
}

// padLinesToHeight pads a slice of lines to exactly h entries.  Lines that are
// too short are padded with spaces; excess lines are dropped.
func padLinesToHeight(lines []string, h, w int) []string {
	out := make([]string, h)
	for i := 0; i < h; i++ {
		if i < len(lines) {
			out[i] = lines[i]
		} else {
			out[i] = lipgloss.NewStyle().Background(NCBg).Render(strings.Repeat(" ", w))
		}
	}
	return out
}
