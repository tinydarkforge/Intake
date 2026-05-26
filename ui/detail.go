package ui

import (
	"context"
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/textarea"
	"github.com/charmbracelet/bubbles/textinput"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/tinydarkforge/intake/app"
	"github.com/tinydarkforge/intake/components"
	"github.com/tinydarkforge/intake/services"
	"github.com/tinydarkforge/intake/types"
)

type detailMode int

const (
	detailModeView detailMode = iota
	detailModeComment
	detailModeEdit
)

type DetailModel struct {
	gh        *services.GitHub
	issue     *types.Issue
	vp        viewport.Model
	mode      detailMode
	commentIn textinput.Model
	editTitle textinput.Model
	editBody  textarea.Model
	status    string
	statusErr bool
	width     int
	height    int
}

func NewDetail(gh *services.GitHub) DetailModel {
	ci := textinput.New()
	ci.Placeholder = "Add a comment…"

	et := textinput.New()
	et.Placeholder = "Issue title…"
	et.CharLimit = 200

	eb := textarea.New()
	eb.Placeholder = "Issue body…"

	vp := viewport.New(80, 20)
	return DetailModel{gh: gh, vp: vp, commentIn: ci, editTitle: et, editBody: eb}
}

func (m DetailModel) SetIssue(issue *types.Issue) DetailModel {
	m.issue = issue
	m.vp.SetContent(renderIssueBody(issue, m.width-4))
	return m
}

func (m DetailModel) Init() tea.Cmd { return nil }

func (m DetailModel) InterceptsKeys() bool {
	return m.mode == detailModeComment || m.mode == detailModeEdit
}

func (m DetailModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.vp.Width = m.width - 4
		m.vp.Height = m.height - 8
		if m.issue != nil {
			m.vp.SetContent(renderIssueBody(m.issue, m.width-4))
		}

	case app.OpenIssueMsg:
		m.issue = nil // clear stale content while loading
		m.status = ""
		return m, m.fetchCmd(msg.Number)

	case app.IssueActionDoneMsg:
		m.status = fmt.Sprintf("%s done on #%d", msg.Action, msg.Number)
		m.statusErr = false
		return m, m.reloadCmd()

	case app.IssueLoadedMsg:
		m.issue = msg.Issue
		m.vp.SetContent(renderIssueBody(m.issue, m.width-4))

	case app.ErrMsg:
		m.status = msg.Err.Error()
		m.statusErr = true

	case app.IssueEditedMsg:
		m.status = fmt.Sprintf("#%d saved", msg.Number)
		m.statusErr = false
		m.mode = detailModeView
		return m, m.fetchCmd(msg.Number)

	case tea.KeyMsg:
		switch m.mode {
		case detailModeView:
			cmd := m.handleViewKey(msg)
			if cmd != nil {
				return m, cmd
			}
			var vpCmd tea.Cmd
			m.vp, vpCmd = m.vp.Update(msg)
			return m, vpCmd
		case detailModeComment:
			cmd := m.handleCommentKey(msg)
			if cmd != nil {
				return m, cmd
			}
			var ciCmd tea.Cmd
			m.commentIn, ciCmd = m.commentIn.Update(msg)
			return m, ciCmd
		case detailModeEdit:
			cmd := m.handleEditKey(msg)
			if cmd != nil {
				return m, cmd
			}
			// tab cycles between title and body
			if msg.String() == "tab" {
				if m.editTitle.Focused() {
					m.editTitle.Blur()
					m.editBody.Focus()
				} else {
					m.editBody.Blur()
					m.editTitle.Focus()
				}
				return m, nil
			}
			if m.editTitle.Focused() {
				var c tea.Cmd
				m.editTitle, c = m.editTitle.Update(msg)
				return m, c
			}
			var c tea.Cmd
			m.editBody, c = m.editBody.Update(msg)
			return m, c
		}
	default:
		if m.mode == detailModeView {
			var vpCmd tea.Cmd
			m.vp, vpCmd = m.vp.Update(msg)
			return m, vpCmd
		}
	}
	return m, nil
}

func (m *DetailModel) handleViewKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, app.Detail.Edit):
		if m.issue == nil {
			return nil
		}
		m.editTitle.SetValue(m.issue.Title)
		m.editBody.SetValue(m.issue.Body)
		m.editBody.SetWidth(m.width - 6)
		m.editBody.SetHeight(m.height - 12)
		m.mode = detailModeEdit
		m.editTitle.Focus()
	case key.Matches(msg, app.Detail.Comment):
		m.mode = detailModeComment
		m.commentIn.Focus()
	case key.Matches(msg, app.Detail.Assign):
		if m.issue == nil {
			return nil
		}
		num := m.issue.Number
		return func() tea.Msg {
			if err := m.gh.AssignMe(context.Background(), num); err != nil {
				return app.ErrMsg{Err: err}
			}
			return app.IssueActionDoneMsg{Action: "assigned", Number: num}
		}
	case key.Matches(msg, app.Detail.Close):
		if m.issue == nil || m.issue.State == "closed" {
			return nil
		}
		num := m.issue.Number
		return func() tea.Msg {
			if err := m.gh.Close(context.Background(), num); err != nil {
				return app.ErrMsg{Err: err}
			}
			return app.IssueActionDoneMsg{Action: "closed", Number: num}
		}
	case key.Matches(msg, app.Detail.Reopen):
		if m.issue == nil || m.issue.State == "open" {
			return nil
		}
		num := m.issue.Number
		return func() tea.Msg {
			if err := m.gh.Reopen(context.Background(), num); err != nil {
				return app.ErrMsg{Err: err}
			}
			return app.IssueActionDoneMsg{Action: "reopened", Number: num}
		}
	}
	return nil
}

func (m *DetailModel) handleCommentKey(msg tea.KeyMsg) tea.Cmd {
	switch msg.String() {
	case "esc":
		m.mode = detailModeView
		m.commentIn.Reset()
	case "enter":
		body := strings.TrimSpace(m.commentIn.Value())
		if body == "" {
			return nil
		}
		num := m.issue.Number
		m.commentIn.Reset()
		m.mode = detailModeView
		return func() tea.Msg {
			if err := m.gh.Comment(context.Background(), num, body); err != nil {
				return app.ErrMsg{Err: err}
			}
			return app.IssueActionDoneMsg{Action: "commented", Number: num}
		}
	}
	return nil
}

func (m *DetailModel) fetchCmd(number int) tea.Cmd {
	gh := m.gh
	return func() tea.Msg {
		issue, err := gh.View(context.Background(), number)
		if err != nil {
			return app.ErrMsg{Err: err}
		}
		return app.IssueLoadedMsg{Issue: issue}
	}
}

func (m *DetailModel) handleEditKey(msg tea.KeyMsg) tea.Cmd {
	switch {
	case key.Matches(msg, app.Detail.Save):
		if m.issue == nil {
			return nil
		}
		title := strings.TrimSpace(m.editTitle.Value())
		body := m.editBody.Value()
		if title == "" {
			m.status = "title cannot be empty"
			m.statusErr = true
			return nil
		}
		num := m.issue.Number
		gh := m.gh
		return func() tea.Msg {
			if err := gh.EditIssue(context.Background(), num, title, body); err != nil {
				return app.ErrMsg{Err: err}
			}
			return app.IssueEditedMsg{Number: num, Title: title, Body: body}
		}
	case key.Matches(msg, app.Global.Back):
		m.mode = detailModeView
		m.editTitle.Blur()
		m.editBody.Blur()
	}
	return nil
}

func (m *DetailModel) reloadCmd() tea.Cmd {
	if m.issue == nil {
		return nil
	}
	return m.fetchCmd(m.issue.Number)
}

func (m DetailModel) View() string {
	if m.issue == nil {
		if m.status != "" {
			return app.StyleError.Render("  ✗ " + m.status)
		}
		return app.StyleDim.Render("  loading…")
	}

	if m.mode == detailModeEdit {
		return m.viewEdit()
	}

	stateStyle := app.StyleSuccess
	if m.issue.State == "closed" {
		stateStyle = app.StyleError
	}

	var labelParts []string
	for _, l := range m.issue.Labels {
		labelParts = append(labelParts, app.StyleLabel.Render(l.Name))
	}
	labels := strings.Join(labelParts, " ")

	header := strings.Join([]string{
		"",
		app.StyleBold.Render(fmt.Sprintf("  #%d  %s", m.issue.Number, m.issue.Title)),
		fmt.Sprintf("  State: %s   %s", stateStyle.Render(m.issue.State), labels),
		"",
	}, "\n")

	var lines []string
	lines = append(lines, header, m.vp.View())

	if m.mode == detailModeComment {
		lines = append(lines, "",
			app.StyleAccent.Render("  Comment: ")+m.commentIn.View(),
			app.StyleDim.Render("  enter to post · esc to cancel"),
		)
	} else {
		lines = append(lines, components.FooterHint(m.width, app.Global.Back, app.Global.Quit))
	}

	if m.status != "" {
		s := app.StyleStatusOK.Render("  ✓ " + m.status)
		if m.statusErr {
			s = app.StyleStatusErr.Render("  ✗ " + m.status)
		}
		lines = append(lines, s)
	}

	return strings.Join(lines, "\n")
}

func (m DetailModel) viewEdit() string {
	focused := "title"
	if m.editBody.Focused() {
		focused = "body"
	}
	titleStyle := app.StyleDim
	bodyStyle := app.StyleDim
	if focused == "title" {
		titleStyle = app.StyleAccent
	} else {
		bodyStyle = app.StyleAccent
	}

	lines := []string{
		"",
		app.StyleBold.Render(fmt.Sprintf("  Edit  #%d", m.issue.Number)),
		"",
		titleStyle.Render("  Title"),
		"  " + m.editTitle.View(),
		"",
		bodyStyle.Render("  Body"),
		"  " + m.editBody.View(),
		"",
		app.StyleDim.Render("  tab switch field  ·  ctrl+s save  ·  esc cancel"),
	}

	if m.status != "" {
		s := app.StyleStatusOK.Render("  ✓ " + m.status)
		if m.statusErr {
			s = app.StyleStatusErr.Render("  ✗ " + m.status)
		}
		lines = append(lines, s)
	}

	return strings.Join(lines, "\n")
}

func renderIssueBody(issue *types.Issue, width int) string {
	if issue == nil {
		return ""
	}
	return components.RenderMarkdown(issue.Body, width)
}
