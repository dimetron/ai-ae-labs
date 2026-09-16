// mcptool.go — the external MCP tool boundary, made real.
//
// The lecture's "MCP пізніше" slide said: today the tool lives in our Go
// code, MCP is the next layer. This file is that layer, wired to a real
// Ukrainian MCP server: dimetron/mono-go-mcp (github.com/dimetron/mono-go-mcp),
// which exposes the monobank open API as 5 tools through the official
// Go MCP SDK (github.com/modelcontextprotocol/go-sdk) over stdio.
//
// Two shapes are taught in this repo; this lab uses the first:
//
//  1. ADK `mcptoolset.New` — the MCP server's tools become ADK tools the
//     model calls like any local tool. This is the default production path
//     and the one wired into the agent here.
//  2. Raw SDK `mcp.NewClient` + `mcp.CommandTransport` — for agents outside
//     ADK. Shown in Homework's 🔥 bonus; compile-verified in scratch.
//
// The boundary teaching point is unchanged from the lecture: the MCP server
// validates input against its own `inputSchema` before executing, and our
// side still sanitizes the output. The protocol moves the boundary; it does
// not remove the responsibility.
package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/mcptoolset"
)

// monoMCPBin resolves the mono-go-mcp server binary: MONO_MCP_BIN first,
// then $GOPATH/bin, then $HOME/go/bin. A missing binary is an explicit
// error, not a silent degradation — the agent still works with its local
// tools only (least agency: the model sees exactly the tools we wired).
func monoMCPBin() (string, error) {
	if bin := os.Getenv("MONO_MCP_BIN"); bin != "" {
		if st, err := os.Stat(bin); err == nil && !st.IsDir() {
			return bin, nil
		}
		return "", fmt.Errorf("MONO_MCP_BIN=%s does not exist", bin)
	}
	home := os.Getenv("HOME")
	gopath := os.Getenv("GOPATH")
	if gopath == "" {
		gopath = filepath.Join(home, "go")
	}
	for _, dir := range []string{filepath.Join(gopath, "bin"), filepath.Join(home, "go", "bin")} {
		if dir == "" || dir == filepath.Join("", "bin") {
			continue
		}
		candidate := filepath.Join(dir, "mono-go-mcp")
		if st, err := os.Stat(candidate); err == nil && !st.IsDir() {
			return candidate, nil
		}
	}
	return "", errors.New("mono-go-mcp binary not found: install it with " +
		"`go install github.com/dimetron/mono-go-mcp/cmd/mono-go-mcp@latest` " +
		"or set MONO_MCP_BIN to its path")
}

// MonoMCPToolset builds an ADK Toolset wired to the mono-go-mcp server over
// stdio. The public tools (mono_currency_rates, mono_bank_sync) need no
// token; the /personal/* tools need MONO_TOKEN in the environment —
// LoadEnv has already loaded it from apps/.env by the time this runs.
//
// Optional env: MONO_MCP_BIN (explicit binary path), MONO_TOKEN (passed
// through to the server process).
func MonoMCPToolset() (tool.Toolset, error) {
	bin, err := monoMCPBin()
	if err != nil {
		return nil, err
	}
	cmd := exec.Command(bin)
	// The server loads its own .env and honors MONO_TOKEN from the
	// environment; nothing to pass here beyond what LoadEnv set.
	return mcptoolset.New(mcptoolset.Config{
		Transport: &mcp.CommandTransport{Command: cmd},
	})
}

// HasMonoMCP is a cheap check for the agent wiring: should the external MCP
// toolset be attached? true when the binary is resolvable. Errors are
// reported by MonoMCPToolset itself; this only decides.
func HasMonoMCP() bool {
	_, err := monoMCPBin()
	return err == nil
}
