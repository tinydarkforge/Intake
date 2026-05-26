package app

import (
	"github.com/tinydarkforge/intake/types"
)

// ---- Navigation ----

type NavigateMsg struct{ Screen Screen }
type BackMsg struct{}

// OpenIssueMsg navigates to the detail screen and fetches the given issue.
type OpenIssueMsg struct{ Number int }

// ---- Issues ----

type IssuesLoadedMsg struct {
	Issues []types.Issue
	State  string
}
type IssueLoadedMsg struct{ Issue *types.Issue }
type IssueCreatedMsg struct {
	Number int
	URL    string
	Draft  types.Draft
}
type BranchCreatedMsg struct {
	Name string
}
type IssueActionDoneMsg struct {
	Action string
	Number int
}
type IssueEditedMsg struct {
	Number int
	Title  string
	Body   string
}

// ---- Agent / Create flow ----

type AgentResponseMsg struct{ Resp types.AgentResponse }
type AgentFinalMsg struct{ Resp types.AgentResponse }

// ---- Status bar ----

type StatusMsg struct {
	Text    string
	IsError bool
}
type ClearStatusMsg struct{}

// ---- Error ----

type ErrMsg struct{ Err error }

func (e ErrMsg) Error() string { return e.Err.Error() }

// ---- Config ----

type ConfigSavedMsg struct{}

// ---- Mouse ----

// CursorToMsg asks a pane to move its cursor to the given row index.
type CursorToMsg struct{ Row int }
