package labrun_test

import (
	"testing"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/agent/workflowagent"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
	"google.golang.org/adk/v2/workflow"

	"github.com/dimetron/ai-eng-course/labs/internal/fakellm"
	"github.com/dimetron/ai-eng-course/labs/internal/labrun"
)

// echoIn/echoOut are the typed tool contract; functiontool infers the schema.
type echoIn struct {
	Word string `json:"word" jsonschema:"the word to echo back"`
}

type echoOut struct {
	Echoed string `json:"echoed"`
}

// TestRunDrivesToolCall is the load-bearing test of this package.
//
// It proves the scripted model actually reaches the tool through a real
// runner — the step week 1 part 2's test explicitly deferred ("the script is
// staged but unconsumed until a Runner drives the agent"). If this breaks,
// every lab solution's end-to-end test breaks with it.
func TestRunDrivesToolCall(t *testing.T) {
	var gotWord string
	echo, err := functiontool.New(
		functiontool.Config{
			Name:        "echo",
			Description: "Echoes the supplied word back to the caller verbatim.",
		},
		func(ctx agent.Context, in echoIn) (echoOut, error) {
			gotWord = in.Word
			return echoOut{Echoed: in.Word}, nil
		},
	)
	if err != nil {
		t.Fatalf("functiontool.New() error = %v", err)
	}

	m := fakellm.New("fake",
		fakellm.CallTurn("echo", map[string]any{"word": "агент"}),
		fakellm.TextTurn("готово: агент"),
	)

	a, err := llmagent.New(llmagent.Config{
		Name:        "echo_agent",
		Model:       m,
		Description: "Echoes a word using the echo tool.",
		Instruction: "Always call the echo tool. Never answer from memory.",
		Tools:       []tool.Tool{echo},
	})
	if err != nil {
		t.Fatalf("llmagent.New() error = %v", err)
	}

	res, err := labrun.Run(t.Context(), a, "echo the word агент")
	if err != nil {
		t.Fatalf("labrun.Run() error = %v", err)
	}

	if gotWord != "агент" {
		t.Errorf("tool handler received word = %q, want %q", gotWord, "агент")
	}
	if !res.CalledTool("echo") {
		t.Errorf("ToolCalls = %v, want to contain %q", res.ToolCalls, "echo")
	}
	if len(res.ToolResults) == 0 {
		t.Error("ToolResults is empty; the tool response never reached the event stream")
	}
	if res.Final != "готово: агент" {
		t.Errorf("Final = %q, want %q", res.Final, "готово: агент")
	}
	// Two scripted turns, both consumed: one tool call, one final answer.
	if m.Remaining() != 0 {
		t.Errorf("Remaining() = %d, want 0 (agent stopped early)", m.Remaining())
	}
	if len(res.Events) == 0 {
		t.Error("Events is empty; nothing to assert an event-sourced trace against")
	}
}

// TestRunWorkflowAgent proves the same harness reads plain workflow function
// nodes, whose output lands in Event.Output rather than Content parts.
func TestRunWorkflowAgent(t *testing.T) {
	shout := workflow.NewFunctionNode("shout",
		func(ctx agent.Context, in string) (string, error) {
			return in + "!", nil
		}, workflow.NodeConfig{})

	a, err := workflowagent.New(workflowagent.Config{
		Name:        "shout_flow",
		Description: "Appends an exclamation mark.",
		Edges:       workflow.Chain(workflow.Start, shout),
	})
	if err != nil {
		t.Fatalf("workflowagent.New() error = %v", err)
	}

	res, err := labrun.Run(t.Context(), a, "готово")
	if err != nil {
		t.Fatalf("labrun.Run() error = %v", err)
	}
	if res.Final != "готово!" {
		t.Errorf("Final = %q, want %q", res.Final, "готово!")
	}
}

// TestRunnerSharesSessionAcrossTurns pins the behaviour week 5 depends on:
// two turns on one Runner accumulate history in the same session.
func TestRunnerSharesSessionAcrossTurns(t *testing.T) {
	m := fakellm.New("fake",
		fakellm.TextTurn("перша відповідь"),
		fakellm.TextTurn("друга відповідь"),
	)
	a, err := llmagent.New(llmagent.Config{
		Name:        "chat_agent",
		Model:       m,
		Description: "Plain chat agent.",
		Instruction: "Answer briefly.",
	})
	if err != nil {
		t.Fatalf("llmagent.New() error = %v", err)
	}

	r := labrun.NewRunner(a)
	if _, err := r.Turn(t.Context(), "u1", "s1", "привіт"); err != nil {
		t.Fatalf("first Turn() error = %v", err)
	}
	second, err := r.Turn(t.Context(), "u1", "s1", "ще раз")
	if err != nil {
		t.Fatalf("second Turn() error = %v", err)
	}
	if second.Final != "друга відповідь" {
		t.Errorf("Final = %q, want %q", second.Final, "друга відповідь")
	}

	// The second request must carry the first turn's history, otherwise the
	// session is not actually shared.
	reqs := m.Requests()
	if len(reqs) != 2 {
		t.Fatalf("model saw %d requests, want 2", len(reqs))
	}
	if len(reqs[1].Contents) <= len(reqs[0].Contents) {
		t.Errorf("second request has %d contents, first had %d; history did not accumulate",
			len(reqs[1].Contents), len(reqs[0].Contents))
	}
}
