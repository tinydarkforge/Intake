package components

import (
	"strings"

	"github.com/charmbracelet/glamour"
)

// RenderMarkdown converts markdown string to terminal-friendly styled output.
func RenderMarkdown(content string, width int) string {
	if strings.TrimSpace(content) == "" {
		return ""
	}

	r, err := glamour.NewTermRenderer(
		glamour.WithAutoStyle(),
		glamour.WithWordWrap(width),
	)
	if err != nil {
		return content
	}

	out, err := r.Render(content)
	if err != nil {
		return content
	}

	return out
}
