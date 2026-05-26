package services

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/tinydarkforge/intake/types"
)

type GitHub struct {
	Repo string
}

func NewGitHub(repo string) *GitHub { return &GitHub{Repo: repo} }

func (g *GitHub) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, "gh", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = err.Error()
		}
		return nil, fmt.Errorf("gh %s: %s", strings.Join(args, " "), msg)
	}
	return stdout.Bytes(), nil
}

func (g *GitHub) CheckAuth(ctx context.Context) error {
	_, err := g.run(ctx, "auth", "status")
	return err
}

func (g *GitHub) List(ctx context.Context, state string, limit int) ([]types.Issue, error) {
	if state == "" {
		state = "open"
	}
	if limit <= 0 {
		limit = 30
	}
	out, err := g.run(ctx,
		"issue", "list",
		"--repo", g.Repo,
		"--state", state,
		"--limit", strconv.Itoa(limit),
		"--json", "number,title,state,url,labels,assignees,createdAt,updatedAt",
	)
	if err != nil {
		return nil, err
	}
	var issues []types.Issue
	if err := json.Unmarshal(out, &issues); err != nil {
		return nil, fmt.Errorf("decode issue list: %w", err)
	}
	return issues, nil
}

func (g *GitHub) View(ctx context.Context, number int) (*types.Issue, error) {
	out, err := g.run(ctx,
		"issue", "view", strconv.Itoa(number),
		"--repo", g.Repo,
		"--json", "number,title,state,url,body,labels,assignees,createdAt,updatedAt",
	)
	if err != nil {
		return nil, err
	}
	var issue types.Issue
	if err := json.Unmarshal(out, &issue); err != nil {
		return nil, fmt.Errorf("decode issue view: %w", err)
	}
	return &issue, nil
}

func (g *GitHub) Comments(ctx context.Context, number int) ([]types.Comment, error) {
	out, err := g.run(ctx,
		"issue", "view", strconv.Itoa(number),
		"--repo", g.Repo,
		"--json", "comments",
	)
	if err != nil {
		return nil, err
	}
	var wrap struct {
		Comments []types.Comment `json:"comments"`
	}
	if err := json.Unmarshal(out, &wrap); err != nil {
		return nil, err
	}
	return wrap.Comments, nil
}

// Create writes the body to a temp file and invokes `gh issue create`.
// Returns the issue number and URL.
func (g *GitHub) Create(ctx context.Context, draft types.Draft) (int, string, error) {
	if strings.TrimSpace(draft.Title) == "" {
		return 0, "", errors.New("empty title")
	}
	f, err := os.CreateTemp("", "intake-body-*.md")
	if err != nil {
		return 0, "", err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(draft.Body); err != nil {
		f.Close()
		return 0, "", err
	}
	f.Close()

	args := []string{
		"issue", "create",
		"--repo", g.Repo,
		"--title", draft.Title,
		"--body-file", f.Name(),
	}
	for _, l := range draft.Labels {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		args = append(args, "--label", l)
	}
	out, err := g.run(ctx, args...)
	if err != nil {
		return 0, "", err
	}
	// gh prints the URL as the last non-empty line
	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	url := strings.TrimSpace(lines[len(lines)-1])
	
	// Extract number from URL (e.g. https://github.com/owner/repo/issues/123)
	parts := strings.Split(url, "/")
	num, _ := strconv.Atoi(parts[len(parts)-1])

	return num, url, nil
}

func (g *GitHub) Comment(ctx context.Context, number int, body string) error {
	_, err := g.run(ctx,
		"issue", "comment", strconv.Itoa(number),
		"--repo", g.Repo,
		"--body", body,
	)
	return err
}

func (g *GitHub) Close(ctx context.Context, number int) error {
	_, err := g.run(ctx, "issue", "close", strconv.Itoa(number), "--repo", g.Repo)
	return err
}

func (g *GitHub) Reopen(ctx context.Context, number int) error {
	_, err := g.run(ctx, "issue", "reopen", strconv.Itoa(number), "--repo", g.Repo)
	return err
}

func (g *GitHub) AssignMe(ctx context.Context, number int) error {
	_, err := g.run(ctx,
		"issue", "edit", strconv.Itoa(number),
		"--repo", g.Repo,
		"--add-assignee", "@me",
	)
	return err
}

// EditIssue updates the title and/or body of an existing issue.
func (g *GitHub) EditIssue(ctx context.Context, number int, title, body string) error {
	f, err := os.CreateTemp("", "intake-edit-*.md")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err := f.WriteString(body); err != nil {
		f.Close()
		return err
	}
	f.Close()

	_, err = g.run(ctx,
		"issue", "edit", strconv.Itoa(number),
		"--repo", g.Repo,
		"--title", title,
		"--body-file", f.Name(),
	)
	return err
}

func (g *GitHub) Unassign(ctx context.Context, number, _unused int) error {
	_, err := g.run(ctx,
		"issue", "edit", strconv.Itoa(number),
		"--repo", g.Repo,
		"--remove-assignee", "@me",
	)
	return err
}
