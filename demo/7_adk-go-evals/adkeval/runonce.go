package adkeval

import (
	"context"
	"fmt"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/runner"
	"google.golang.org/adk/v2/session"
	"google.golang.org/genai"
)

// RunOnce — виконує одну сесію агента через runner.NewInMemory і зводить
// події в []*EventRecord. Це «generate»-половина Python-циклу eval
// (generate → grade): журнал траєкторії — самостійний артефакт, який можна
// зберегти як golden або погрейдити офлайн.
func RunOnce(ctx context.Context, a agent.Agent, appName, userText string) ([]*EventRecord, error) {
	rn, err := runner.NewInMemory(appName, a)
	if err != nil {
		return nil, fmt.Errorf("runner: %w", err)
	}
	sessionID := "eval-session-1"
	events := []*EventRecord{}
	for ev, err := range rn.Run(ctx, "student", sessionID,
		genai.NewContentFromText(userText, "user"), agent.RunConfig{}) {
		if err != nil {
			return events, fmt.Errorf("run: %w", err)
		}
		if ev == nil {
			continue
		}
		rec := &EventRecord{Author: ev.Author, NodePath: nodePath(ev)}
		if ev.Content != nil {
			var txt string
			for _, p := range ev.Content.Parts {
				if p.Text != "" {
					txt += p.Text
				}
			}
			rec.Text = txt
			for _, fc := range FunctionCalls(ev.Content) {
				rec.ToolCalls = append(rec.ToolCalls, ToolCallRec{
					ID: fc.ID, Name: fc.Name, Args: fc.Args})
			}
		}
		events = append(events, rec)
	}
	return events, nil
}

func nodePath(ev *session.Event) string {
	if ev.NodeInfo == nil {
		return ""
	}
	return ev.NodeInfo.Path
}
