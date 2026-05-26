package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/lipgloss"

	"github.com/tinydarkforge/intake/app"
	"github.com/tinydarkforge/intake/components"
)

func (m CreateModel) View() string {
	var lines []string

	switch m.step {
	case stepChooseTemplate:
		lines = m.viewTemplates()
	case stepTitle:
		lines = m.viewTitle()
	case stepBrief:
		lines = m.viewBrief()
	case stepAgentTurns:
		lines = m.viewAgentTurns()
	case stepPreview:
		lines = m.viewPreview()
	case stepRefine:
		lines = m.viewRefine()
	case stepCreating:
		lines = []string{
			"",
			fmt.Sprintf("  %s  Creating issue…", m.spinner.View()),
		}
	case stepDone:
		lines = []string{
			"",
			app.StyleSuccess.Render("  ✓ Issue created: ") + app.StyleBlue.Render(m.createdURL),
			"",
		}
		if m.branchName != "" {
			lines = append(lines, app.StyleSuccess.Render("  ✓ Branch created: ")+app.StyleBlue.Render(m.branchName), "")
		} else {
			lines = append(lines, app.StyleDim.Render("  Press b to create a local feature branch"), "")
		}
		lines = append(lines, app.StyleDim.Render("  Press c to create another or esc to return."))
	}

	if m.statusText != "" {
		var s string
		if m.statusErr {
			s = app.StyleError.Render("  ✗ " + m.statusText)
		} else {
			s = app.StyleSuccess.Render("  ✓ " + m.statusText)
		}
		lines = append(lines, "", s)
	}

	footer := components.FooterHint(m.width,
		app.Global.Back, app.Global.Quit,
	)
	lines = append(lines, footer)
	return strings.Join(lines, "\n")
}

func (m CreateModel) viewTemplates() []string {
	lines := []string{
		"",
		app.StyleBold.Render("  Choose a template"),
		"",
	}
	for i, t := range m.templates {
		line := fmt.Sprintf("  %s", t.DisplayName())
		if i == m.tmplIdx {
			line = app.StyleListItemSelected.Width(m.width - 4).Render(line)
		} else {
			line = app.StyleListItem.Render(line)
		}
		lines = append(lines, line)
	}
	lines = append(lines, "", app.StyleDim.Render("  j/k to move · enter to select"))
	return lines
}

func (m CreateModel) viewTitle() []string {
	tmpl := m.templates[m.tmplIdx]
	return []string{
		"",
		app.StyleBold.Render(fmt.Sprintf("  New %s", tmpl.DisplayName())),
		"",
		app.StyleDim.Render("  Title  ") + app.StyleDim.Render("(optional — agent infers from context if blank)"),
		"  " + m.titleInput.View(),
		"",
		app.StyleDim.Render("  enter / tab to continue"),
	}
}

func (m CreateModel) viewBrief() []string {
	brief := strings.TrimSpace(m.briefInput.Value())
	hint := "  ctrl+s to start  ·  paste freely — more context = better draft"
	if len(brief) > 150 {
		hint = app.StyleSuccess.Render("  ✓ rich context detected — agent will draft immediately on ctrl+s")
	}
	return []string{
		"",
		app.StyleBold.Render("  Context  ") + app.StyleDim.Render("(paste anything: error logs, Slack threads, PR descriptions…)"),
		"",
		"  " + m.briefInput.View(),
		"",
		app.StyleDim.Render(hint),
	}
}

func (m CreateModel) viewAgentTurns() []string {
	lines := []string{"", app.StyleBold.Render("  ◆ intake is thinking…"), ""}
	if m.agent == nil {
		lines = append(lines, fmt.Sprintf("  %s", m.spinner.View()))
		return lines
	}

	for _, t := range m.agent.History {
		lines = append(lines,
			app.StyleAccent.Render("  Q: ")+t.Question,
			app.StyleDim.Render("  A: ")+t.Answer,
			"",
		)
	}

	nextIdx := len(m.agent.History)
	if nextIdx < len(m.questions) {
		lines = append(lines,
			app.StyleAccent.Render("  Q: ")+m.questions[nextIdx],
			"  "+m.answerIn.View(),
			"",
			app.StyleDim.Render("  enter to answer · type 'skip' to finalize now"),
		)
	} else {
		lines = append(lines, fmt.Sprintf("  %s  Thinking…", m.spinner.View()))
	}
	return lines
}

func (m CreateModel) viewPreview() []string {
	lines := []string{
		"",
		app.StyleBold.Render("  Preview"),
		"",
		app.StyleAccent.Render("  Title: ") + m.draft.Title,
		"",
	}
	body := components.RenderMarkdown(m.draft.Body, m.width-6)
	box := app.BorderNormal.Width(m.width - 6).Render(body)
	lines = append(lines, box, "")

	if len(m.draft.Labels) > 0 {
		var labelParts []string
		for _, l := range m.draft.Labels {
			labelParts = append(labelParts, app.StyleLabel.Render(l))
		}
		labelStr := strings.Join(labelParts, " ")
		lines = append(lines, lipgloss.JoinHorizontal(lipgloss.Top, "  Labels: ", labelStr), "")
	}

	lines = append(lines,
		app.StyleDim.Render("  y  create issue   f  refine   v  edit   r  regenerate   esc  cancel"),
	)
	return lines
}

func (m CreateModel) viewRefine() []string {
	return []string{
		"",
		app.StyleBold.Render("  Refine Draft"),
		"",
		app.StyleDim.Render("  Instruction  ") + app.StyleDim.Render("(e.g. 'more detail', 'fix typo', 'add logs')"),
		"  " + m.refinementInput.View(),
		"",
		app.StyleDim.Render("  enter to submit · esc back to preview"),
	}
}

var _ = lipgloss.NewStyle
