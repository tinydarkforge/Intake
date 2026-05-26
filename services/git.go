package services

import (
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// GitStatus represents the status of a file in git.
type GitStatus string

const (
	GitModified  GitStatus = "M"
	GitAdded     GitStatus = "A"
	GitUntracked GitStatus = "?"
	GitDeleted   GitStatus = "D"
	GitRenamed   GitStatus = "R"
	GitNone      GitStatus = ""
)

// Git handles local git operations.
type Git struct {
	Dir string
}

func NewGit(dir string) *Git {
	return &Git{Dir: dir}
}

// Status returns a map of filename -> GitStatus for the current directory.
func (g *Git) Status(ctx context.Context) (map[string]GitStatus, error) {
	cmd := exec.CommandContext(ctx, "git", "status", "--porcelain")
	cmd.Dir = g.Dir
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	stats := make(map[string]GitStatus)
	lines := strings.Split(string(out), "\n")
	for _, line := range lines {
		if len(line) < 4 {
			continue
		}
		// status is first two chars
		s := strings.TrimSpace(line[:2])
		// file path starts at index 3
		path := line[3:]
		
		// handle renames "R  old -> new"
		if strings.Contains(path, " -> ") {
			parts := strings.Split(path, " -> ")
			path = parts[len(parts)-1]
		}

		var status GitStatus
		switch s {
		case "M":
			status = GitModified
		case "A":
			status = GitAdded
		case "??":
			status = GitUntracked
		case "D":
			status = GitDeleted
		case "R":
			status = GitRenamed
		default:
			if strings.Contains(s, "M") {
				status = GitModified
			} else if strings.Contains(s, "A") {
				status = GitAdded
			} else {
				status = GitNone
			}
		}
		
		if status != GitNone {
			// porcelain path is relative to repo root
			stats[path] = status
		}
	}
	return stats, nil
}

// CreateBranch creates and switches to a new local branch.
func (g *Git) CreateBranch(ctx context.Context, number int, title string) (string, error) {
	// Sanitize title for branch name: lowercase, replace spaces/special chars with hyphens.
	name := strings.ToLower(title)
	name = strings.Map(func(r rune) rune {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			return r
		}
		return '-'
	}, name)

	// Clean up consecutive hyphens and leading/trailing hyphens.
	for strings.Contains(name, "--") {
		name = strings.ReplaceAll(name, "--", "-")
	}
	name = strings.Trim(name, "-")

	// Limit length.
	if len(name) > 40 {
		name = name[:40]
		name = strings.TrimRight(name, "-")
	}

	branchName := fmt.Sprintf("feat/%d-%s", number, name)
	if number == 0 {
		branchName = fmt.Sprintf("feat/%s", name)
	}

	cmd := exec.CommandContext(ctx, "git", "checkout", "-b", branchName)
	cmd.Dir = g.Dir
	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git checkout -b %s: %s", branchName, strings.TrimSpace(string(out)))
	}

	return branchName, nil
}
