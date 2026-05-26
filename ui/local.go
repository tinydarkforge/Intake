package ui

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/tinydarkforge/intake/app"
	"github.com/tinydarkforge/intake/services"
)

// localEntry is one row in the filesystem pane.
type localEntry struct {
	name  string
	isDir bool
	size  int64
}

// LocalPane is the left-side filesystem browser.
type LocalPane struct {
	dir       string
	gitRoot   string
	gitStatus map[string]services.GitStatus // relative to gitRoot
	entries   []localEntry
	attached  map[string]bool // absolute path -> true
	cursor    int
	width     int
	height    int
	err       string
}

// NewLocalPane creates a LocalPane rooted at the process working directory.
func NewLocalPane() LocalPane {
	cwd, err := os.Getwd()
	if err != nil {
		cwd = "."
	}

	root := ""
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	out, err := cmd.Output()
	if err == nil {
		root = strings.TrimSpace(string(out))
	}

	m := LocalPane{
		dir:      cwd,
		gitRoot:  root,
		attached: make(map[string]bool),
	}
	m.reload()
	return m
}

func (m *LocalPane) reload() {
	if m.gitRoot != "" {
		git := services.NewGit(m.gitRoot)
		m.gitStatus, _ = git.Status(context.Background())
	}

	entries, err := os.ReadDir(m.dir)
	if err != nil {
		m.err = err.Error()
		m.entries = nil
		return
	}
	m.err = ""

	var out []localEntry

	// Always prepend ".." unless we are already at filesystem root.
	if filepath.Dir(m.dir) != m.dir {
		out = append(out, localEntry{name: "..", isDir: true})
	}

	// Separate dirs and files, then sort each group alphabetically.
	var dirs, files []localEntry
	for _, e := range entries {
		info, err := e.Info()
		var sz int64
		if err == nil && !e.IsDir() {
			sz = info.Size()
		}
		le := localEntry{name: e.Name(), isDir: e.IsDir(), size: sz}
		if e.IsDir() {
			dirs = append(dirs, le)
		} else {
			files = append(files, le)
		}
	}
	sort.Slice(dirs, func(i, j int) bool { return dirs[i].name < dirs[j].name })
	sort.Slice(files, func(i, j int) bool { return files[i].name < files[j].name })
	out = append(out, dirs...)
	out = append(out, files...)
	m.entries = out
	if m.cursor >= len(m.entries) {
		m.cursor = 0
	}
}

func (m LocalPane) Init() tea.Cmd { return nil }

func (m LocalPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height

	case app.CursorToMsg:
		if msg.Row < len(m.entries) {
			m.cursor = msg.Row
		}

	case app.IssueCreatedMsg:
		m.ClearAttached()

	case tea.KeyMsg:
		switch msg.String() {
		case "up", "k":
			if m.cursor > 0 {
				m.cursor--
			}
		case "down", "j":
			if m.cursor < len(m.entries)-1 {
				m.cursor++
			}
		case "r":
			m.reload()
		case "a":
			if m.cursor < len(m.entries) {
				e := m.entries[m.cursor]
				if e.name != ".." && !e.isDir {
					path := filepath.Join(m.dir, e.name)
					if m.attached[path] {
						delete(m.attached, path)
					} else {
						m.attached[path] = true
					}
				}
			}
		case "enter":
			if m.cursor < len(m.entries) {
				e := m.entries[m.cursor]
				if e.isDir {
					var next string
					if e.name == ".." {
						next = filepath.Dir(m.dir)
					} else {
						next = filepath.Join(m.dir, e.name)
					}
					m.dir = next
					m.cursor = 0
					m.reload()
				}
			}
		}
	}
	return m, nil
}

// Attached returns the map of attached absolute file paths.
func (m LocalPane) Attached() map[string]bool {
	return m.attached
}

// ClearAttached clears the set of attached files.
func (m *LocalPane) ClearAttached() {
	m.attached = make(map[string]bool)
}

// Dir returns the current directory.
func (m LocalPane) Dir() string { return m.dir }

// Selected returns the currently highlighted entry name, or "".
func (m LocalPane) Selected() string {
	if m.cursor < len(m.entries) {
		return m.entries[m.cursor].name
	}
	return ""
}

// View satisfies tea.Model — returns the plain content without any border.
func (m LocalPane) View() string {
	return strings.Join(m.NCLines(m.width, m.height), "\n")
}

// NCLines returns exactly h content lines, each exactly w visible characters wide.
// This is called by the NC frame renderer.
func (m LocalPane) NCLines(w, h int) []string {
	if m.err != "" {
		line := truncPad("  error: "+m.err, w)
		lines := []string{styleLocalLine(line, false)}
		for len(lines) < h {
			lines = append(lines, blankLine(w))
		}
		return lines
	}

	// Column widths: attachment(4) + gap(1) + git(2) + name + gap(1) + size(8)
	const sizeW = 8
	const attachW = 4
	const gitW = 2
	nameW := w - sizeW - attachW - gitW - 2 // 2 for the space separators
	if nameW < 4 {
		nameW = 4
	}

	var lines []string
	for i, e := range m.entries {
		if len(lines) >= h {
			break
		}

		path := filepath.Join(m.dir, e.name)
		var attachPart string
		if m.attached[path] {
			attachPart = app.StyleAccent.Render(" [x]")
		} else if !e.isDir && e.name != ".." {
			attachPart = " [ ]"
		} else {
			attachPart = "    "
		}

		gitPart := "  "
		if m.gitRoot != "" && !e.isDir && e.name != ".." {
			rel, err := filepath.Rel(m.gitRoot, path)
			if err == nil {
				if s, ok := m.gitStatus[rel]; ok {
					switch s {
					case services.GitModified:
						gitPart = app.StyleWarning.Render("M ")
					case services.GitAdded:
						gitPart = app.StyleSuccess.Render("A ")
					case services.GitUntracked:
						gitPart = app.StyleDim.Render("? ")
					case services.GitDeleted:
						gitPart = app.StyleError.Render("D ")
					case services.GitRenamed:
						gitPart = app.StyleBlue.Render("R ")
					}
				}
			}
		}

		var displayName string
		if e.isDir {
			displayName = e.name + "/"
		} else {
			displayName = e.name
		}
		namePart := truncPad(displayName, nameW)

		var sizePart string
		if e.isDir {
			sizePart = truncPad("", sizeW)
		} else {
			sizePart = truncPad(formatSize(e.size), sizeW)
		}

		raw := attachPart + " " + gitPart + namePart + " " + sizePart
		selected := i == m.cursor
		lines = append(lines, styleLocalLine(raw, selected))
	}

	// Pad to h lines.
	for len(lines) < h {
		lines = append(lines, blankLine(w))
	}
	return lines
}

// styleLocalLine applies NC colours to a pre-padded line.
func styleLocalLine(line string, selected bool) string {
	if selected {
		return lipgloss.NewStyle().
			Background(app.NCSelected).
			Foreground(app.NCSelFg).
			Render(line)
	}
	return lipgloss.NewStyle().
		Background(app.NCBg).
		Foreground(lipgloss.Color("#FFFFFF")).
		Render(line)
}

// blankLine returns a blank line styled with NC background, w chars wide.
func blankLine(w int) string {
	return lipgloss.NewStyle().
		Background(app.NCBg).
		Render(strings.Repeat(" ", w))
}

// formatSize renders a file size in a compact human-readable form.
func formatSize(n int64) string {
	switch {
	case n >= 1<<30:
		return fmt.Sprintf("%.1fG", float64(n)/(1<<30))
	case n >= 1<<20:
		return fmt.Sprintf("%.1fM", float64(n)/(1<<20))
	case n >= 1<<10:
		return fmt.Sprintf("%.1fK", float64(n)/(1<<10))
	default:
		return fmt.Sprintf("%dB", n)
	}
}

// truncPad truncates s to at most w visible columns (appending "…" if needed)
// and then right-pads with spaces to exactly w visible columns.
func truncPad(s string, w int) string {
	if w <= 0 {
		return ""
	}
	sw := lipgloss.Width(s)
	if sw >= w {
		r := []rune(s)
		for lipgloss.Width(string(r)) >= w {
			if len(r) == 0 {
				break
			}
			r = r[:len(r)-1]
		}
		return string(r) + "…"
	}
	return s + strings.Repeat(" ", w-sw)
}
