package services

import (
	"context"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/tinydarkforge/intake/types"
)

// Agent runs the multi-turn issue-drafting conversation with Ollama.
// Ported from agent.sh lines 740–897.
type Agent struct {
	O                 *OllamaClient
	Template          types.Template
	Title             string
	Brief             string
	Context           map[string]string // path -> content
	History           []types.Turn
	RefinementHistory []types.Turn // Follow-up instructions from the user
	MaxTurns          int
	Debug             bool
}

const systemInstructions = `You are intake, an agent that drafts GitHub issues from whatever the user provides.

The user may give you:
- A polished one-liner title + brief description
- A raw dump: Slack message, error log, PR description, meeting notes, stack trace, or any unstructured text
- Attached file contents for context
- A mix of the above

Your job is to extract intent and structure from ALL of it — never discard information.

On every turn return STRICTLY ONE JSON object:

{
  "status": "needs_info" | "ready",
  "questions": ["...", "..."],
  "title": "...",
  "body": "...",
  "rationale": "..."
}

Rules:
- If the user gave you rich context (long paste, error details, code snippets), prefer status="ready" and draft the best possible issue immediately. Do NOT ask questions just for the sake of it.
- If the title hint is blank or generic, infer a clear, specific title from the context.
- body MUST follow the template structure. Fill every section with real content extracted from the user's input. Use TBD only for things genuinely unknown and not inferable.
- When status is "needs_info": include 1–3 targeted questions for truly missing critical details only.
- Never include prose outside the JSON object. Never wrap it in markdown fences.
- Reproduce relevant details verbatim (error messages, stack traces, steps) — do not paraphrase away specifics.
`

// NextTurn sends the next prompt to Ollama and parses the response.
func (a *Agent) NextTurn(ctx context.Context, turn int) (types.AgentResponse, error) {
	prompt := a.buildPrompt(turn, false)
	var resp types.AgentResponse
	raw, err := a.O.GenerateJSON(ctx, prompt)
	if err != nil {
		return resp, err
	}
	if a.Debug {
		fmt.Fprintf(debugSink(), "---- ollama raw (turn %d) ----\n%s\n-----------------------------\n", turn, raw)
	}
	if err := ParseInto(raw, &resp); err != nil {
		return resp, fmt.Errorf("parse agent response: %w", err)
	}
	return resp, nil
}

// Finalize forces the agent to produce a ready answer immediately,
// used when the user opts out of more questions (agent.sh skip mode).
func (a *Agent) Finalize(ctx context.Context) (types.AgentResponse, error) {
	prompt := a.buildPrompt(len(a.History)+1, true)
	var resp types.AgentResponse
	raw, err := a.O.GenerateJSON(ctx, prompt)
	if err != nil {
		return resp, err
	}
	if err := ParseInto(raw, &resp); err != nil {
		return resp, err
	}
	if !resp.IsReady() {
		resp.Status = types.StatusReady
	}
	return resp, nil
}

// Refine sends a follow-up instruction to the agent to adjust the current draft.
func (a *Agent) Refine(ctx context.Context, instruction string) (types.AgentResponse, error) {
	a.RefinementHistory = append(a.RefinementHistory, types.Turn{Answer: instruction})
	prompt := a.buildPrompt(len(a.History)+1, true)
	var resp types.AgentResponse
	raw, err := a.O.GenerateJSON(ctx, prompt)
	if err != nil {
		return resp, err
	}
	if err := ParseInto(raw, &resp); err != nil {
		return resp, err
	}
	// Always treat refinement as producing a "ready" draft.
	resp.Status = types.StatusReady
	return resp, nil
}

func debugSink() io.Writer { return os.Stderr }

func (a *Agent) buildPrompt(turn int, forceReady bool) string {
	var b strings.Builder
	b.WriteString(systemInstructions)
	b.WriteString("\n\n# Template\n")
	if a.Template.Name != "" {
		fmt.Fprintf(&b, "Name: %s\n", a.Template.Name)
	}
	if len(a.Template.Labels) > 0 {
		fmt.Fprintf(&b, "Labels: %s\n", strings.Join(a.Template.Labels, ", "))
	}
	b.WriteString("Body scaffold:\n")
	b.WriteString(a.Template.Body)
	b.WriteString("\n\n# User's input\n")
	if strings.TrimSpace(a.Title) != "" {
		fmt.Fprintf(&b, "Title hint: %s\n", a.Title)
	} else {
		b.WriteString("Title hint: (none — infer from context below)\n")
	}

	if len(a.Context) > 0 {
		b.WriteString("\n# Attached Context Files\n")
		for path, content := range a.Context {
			fmt.Fprintf(&b, "File: %s\n---\n%s\n---\n", path, content)
		}
	}

	fmt.Fprintf(&b, "\nRaw context (may be a paste, error log, Slack message, etc.):\n---\n%s\n---\n", a.Brief)
	if len(a.History) > 0 {
		b.WriteString("\n# Conversation so far\n")
		for i, t := range a.History {
			fmt.Fprintf(&b, "Q%d: %s\nA%d: %s\n", i+1, t.Question, i+1, t.Answer)
		}
	}

	if len(a.RefinementHistory) > 0 {
		b.WriteString("\n# Refinement Instructions\n")
		for i, t := range a.RefinementHistory {
			fmt.Fprintf(&b, "Instruction %d: %s\n", i+1, t.Answer)
		}
	}

	fmt.Fprintf(&b, "\n# Turn %d of %d\n", turn, a.MaxTurns)
	if forceReady {
		b.WriteString("The user wants an immediate draft. Return status=\"ready\" now. Extract everything you can from the raw context; mark only genuinely unknowable details as TBD.\n")
	} else if turn >= a.MaxTurns {
		b.WriteString("This is the last allowed turn. Return status=\"ready\" with the best draft you can produce from available context.\n")
	} else {
		b.WriteString("If the raw context contains enough information to write a solid issue, return status=\"ready\" immediately — do not ask unnecessary questions. Only use status=\"needs_info\" if a critical detail is truly missing and cannot be inferred.\n")
	}
	return b.String()
}
