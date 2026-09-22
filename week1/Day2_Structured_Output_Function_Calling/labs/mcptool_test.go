package main

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"google.golang.org/adk/v2/tool"
)

// TestToolNames pins the startup log line. It is the only place an operator
// sees the model's tool surface, so a silent empty string would hide a wiring
// mistake rather than reveal it.
func TestToolNames(t *testing.T) {
	tests := []struct {
		name  string
		tools []tool.Tool
		want  string
	}{
		{name: "empty", tools: nil, want: ""},
		{
			name:  "single",
			tools: []tool.Tool{stubTool{name: "get_exchange_rate"}},
			want:  "get_exchange_rate",
		},
		{
			name: "several keep call order",
			tools: []tool.Tool{
				stubTool{name: "get_exchange_rate"},
				stubTool{name: "mono_currency_rates"},
				stubTool{name: "mono_bank_sync"},
			},
			want: "get_exchange_rate, mono_currency_rates, mono_bank_sync",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := toolNames(tt.tools); got != tt.want {
				t.Errorf("toolNames() = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestMonoMCPBinHonoursEnvOverride covers the MONO_MCP_BIN branch: an explicit
// path wins, and a path that does not exist is an error rather than a silent
// fallback to a different binary.
func TestMonoMCPBinHonoursEnvOverride(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "fake-mcp")
	if err := os.WriteFile(bin, []byte("#!/bin/sh\n"), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	t.Setenv("MONO_MCP_BIN", bin)
	got, err := monoMCPBin()
	if err != nil {
		t.Fatalf("monoMCPBin() error = %v", err)
	}
	if got != bin {
		t.Errorf("monoMCPBin() = %q, want the MONO_MCP_BIN value %q", got, bin)
	}

	t.Setenv("MONO_MCP_BIN", filepath.Join(dir, "absent"))
	if _, err := monoMCPBin(); err == nil {
		t.Error("monoMCPBin() error = nil for a nonexistent MONO_MCP_BIN, want an error")
	} else if !strings.Contains(err.Error(), "does not exist") {
		t.Errorf("monoMCPBin() error = %q, want it to say the path does not exist", err)
	}
}

// TestAgentMCPToolsWithholdsWebhook exercises the real MCP boundary when the
// server binary is installed. The assertion is the lab's policy: the agent
// offers the server's tools minus the mutating one, no matter what the server
// exposes. Skipped when mono-go-mcp is not installed.
func TestAgentMCPToolsWithholdsWebhook(t *testing.T) {
	if !HasMonoMCP() {
		t.Skip("mono-go-mcp not installed; the key-free path uses the local tool only")
	}

	tools, err := AgentMCPTools(context.Background())
	if err != nil {
		t.Skipf("mono-go-mcp unavailable: %v", err)
	}

	for _, tl := range tools {
		if tl.Name() == "mono_set_webhook" {
			t.Errorf("AgentMCPTools() offered the withheld mutating tool %q", tl.Name())
		}
	}
	// The public rate tool must survive the filter: it is the one the lab
	// actually demonstrates.
	var sawRates bool
	for _, tl := range tools {
		if tl.Name() == "mono_currency_rates" {
			sawRates = true
		}
	}
	if !sawRates {
		t.Errorf("AgentMCPTools() dropped mono_currency_rates; got %v", toolNames(tools))
	}
}
