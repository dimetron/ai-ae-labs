#!/usr/bin/env bash
# Provision the ai-ae-labs dev container. Run once by onCreateCommand.
#
# Adapted from pi-go's .devcontainer/post-create.sh.
#
# This is the week-1 branch: it carries no Taskfile.yml, so this installs no
# `task` (the full repository has one). The gates are the three AGENTS.md §2
# commands — `go build ./...`,
# `go test ./...`, `gofmt -l .` — plus the demos. Everything installed here is
# either needed by those, or a tool the course material tells learners to use.
set -euo pipefail

# Cache volumes are created root-owned; the Go toolchain needs them writable.
sudo chown -R "$(id -u):$(id -g)" "${GOPATH:-$HOME/go}" "$HOME/.cache/go-build" 2>/dev/null || true

echo "==> apt packages"
sudo apt-get update -y
sudo DEBIAN_FRONTEND=noninteractive apt-get install -y --no-install-recommends \
  build-essential `# MANDATORY: see the -race note below` \
  ripgrep `# rg — the course material and the Go extension both use it` \
  jq \
  ca-certificates
sudo rm -rf /var/lib/apt/lists/*

# Why build-essential is not optional here.
#
# `go test -race` is cgo-backed. Without a C compiler it fails outright with
#   go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
# rather than degrading to a non-race run. Verified on golang:alpine with no gcc.
#
# The root module needs no cgo of its own (CGO_ENABLED=0 builds everywhere),
# so build-essential exists purely to make the race detector link.
#
echo "==> Go tools"
# gopls: the Go language server, and what the VS Code Go extension drives.
go install golang.org/x/tools/gopls@latest
# dlv: debugging from the IDE.
go install github.com/go-delve/delve/cmd/dlv@latest

echo "==> gitleaks"
# On the full repository this is mandatory: .githooks/pre-commit runs gitleaks
# and blocks EVERY commit when the binary is missing, because the shell reports
# "command not found" and the hook's fallback branch reads that as a found
# secret. This branch ships no .githooks and sets no core.hooksPath, so the
# hazard is not here — but the scan is still worth having, since the whole
# point of the credential rules in AGENTS.md §4 is that a key never reaches
# git. Run it by hand before you push:
#
#   gitleaks detect --no-git --redact      # scan the working tree
#
# Module path is github.com/zricethezav/gitleaks/v8, not github.com/gitleaks/…
# upstream renamed the org but the Go module path was never moved; the
# github.com/gitleaks/gitleaks/v8 path resolves only to ancient v8.15.3.
go install github.com/zricethezav/gitleaks/v8@v8.30.1

echo "==> mono-go-mcp"
# week1/Day2 labs attach this MCP server over stdio; mcptool.go resolves it from
# $GOPATH/bin by name and, when missing, reports an explicit error instead of
# silently running with local tools only. Optional for the gates, so a failure
# here is a warning rather than a broken container.
go install github.com/dimetron/mono-go-mcp/cmd/mono-go-mcp@latest ||
  echo "    (skipped: mono-go-mcp install failed — only the week1/Day2 MCP lab needs it)"

echo "==> pi-go CLI"
# pi-go is two separable things, and it is worth not confusing them:
#
#   * the LIBRARY the labs compile against — `github.com/dimetron/pi-go/pimodels`,
#     imported by demo/adk-quickstart and resolved by its own go.mod. Go handles
#     that version; nothing here needs to.
#   * the CLI, `pi`, installed below. No lab imports it or shells out to it, so
#     it is a convenience, not a build dependency — which is why it tracks the
#     newest release while the demo stays on whatever its go.mod pins.
#
# Installed from the published release binaries rather than `go install`: that
# module's go.mod carries `replace` directives for its vendored third_party/
# SDKs, and `go install pkg@version` refuses any module whose build would differ
# from being the main module:
#   The go.mod file for the module providing named packages contains one or more
#   replace directives.
# Building from a clone would work but pulls the whole tree; the tarball is
# faster and is what a learner would grab anyway. Checksums below are from the
# release's checksums.txt.
PI_GO_VERSION="0.1.6"
case "$(uname -m)" in
aarch64 | arm64) pi_go_arch=arm64 ; pi_go_sha=decf9eb1f5591e44c93e6fc1a3fa8848d0541898fc36382ff44b24afbd3aabf8 ;;
x86_64 | amd64) pi_go_arch=amd64 ; pi_go_sha=54ecef34f86345de3648abb9fd3c4a253b8f1b78ce14e351e83fc7964acd825c ;;
*)
  echo "    (skipped: no pi-go release binary for $(uname -m))"
  pi_go_arch=""
  ;;
esac
if [ -n "$pi_go_arch" ]; then
  pi_go_tgz="pi-go_${PI_GO_VERSION}_linux_${pi_go_arch}.tar.gz"
  pi_go_tmp="$(mktemp -d)"
  if curl -fsSL -o "$pi_go_tmp/$pi_go_tgz" \
    "https://github.com/dimetron/pi-go/releases/download/v${PI_GO_VERSION}/${pi_go_tgz}" &&
    echo "${pi_go_sha}  $pi_go_tmp/$pi_go_tgz" | sha256sum -c - >/dev/null 2>&1; then
    # The tarball ships the binary as `pi`; install under both names so the
    # documented `pi --print …` and a plain `pi-go` both resolve.
    tar -xzf "$pi_go_tmp/$pi_go_tgz" -C "$pi_go_tmp" pi
    install -m 0755 "$pi_go_tmp/pi" "${GOPATH:-$HOME/go}/bin/pi-go"
    ln -sf pi-go "${GOPATH:-$HOME/go}/bin/pi"
  else
    echo "    (skipped: download or checksum check failed for $pi_go_tgz)"
  fi
  rm -rf "$pi_go_tmp"
fi

echo "==> warm module cache"
# The root module and both nested ones, so the first build is not a cold start.
go mod download
for m in demo/adk-quickstart demo/1_ai-gateway/mock-llm; do
  [ -f "$m/go.mod" ] && (cd "$m" && go mod download)
done

echo "==> PATH"
# remoteEnv in devcontainer.json covers VS Code terminals and lifecycle commands,
# but not `docker exec`, an SSH session into a Codespace, or anything else that
# starts a shell directly. Write the same entries into the system profile so the
# tools above resolve in every shell however it was started.
sudo tee /etc/profile.d/10-ai-ae-labs-path.sh >/dev/null <<'EOF'
export GOPATH="${GOPATH:-$HOME/go}"
# Prepend only when absent, so re-sourcing does not stack duplicate entries.
for _dir in "$GOPATH/bin" "$HOME/.local/bin"; do
	case ":$PATH:" in
	*":$_dir:"*) ;;
	*) PATH="$_dir:$PATH" ;;
	esac
done
unset _dir
export PATH
EOF
sudo chmod 0644 /etc/profile.d/10-ai-ae-labs-path.sh
mkdir -p "${GOPATH:-$HOME/go}/bin"

# Interactive non-login shells — what `docker exec -it … zsh` gives you — read
# only the rc files, which do not source /etc/profile.d.
for rc in "$HOME/.bashrc" "$HOME/.zshrc"; do
	[ -f "$rc" ] || continue
	grep -q '10-ai-ae-labs-path' "$rc" ||
		printf '\n. /etc/profile.d/10-ai-ae-labs-path.sh\n' >>"$rc"
done

echo "==> git"
# The repo ships .githooks/pre-commit (gitleaks secret scan); core.hooksPath is
# set in the host's .git/config, which is NOT cloned, so a fresh container would
# otherwise commit without it.
git config --local core.hooksPath .githooks || true
# The workspace is bind-mounted from the host, so its owner differs from the
# container user; without this every git command in it errors.
git config --global --add safe.directory "$(pwd)" || true

echo "==> verify"
# Fail the build loudly if something the gates depend on cannot actually run,
# rather than at the first lesson.
for tool in go gitleaks rg docker git gh; do
	printf '    %-12s %s\n' "$tool" "$(command -v "$tool" || echo 'MISSING')"
done

cat <<'EOF'

ai-ae-labs dev container ready.

  go build ./... && go test ./...      # the two gates every commit must pass
  gofmt -l .                           # the third; gofmt -w <file> to fix
  go test -race ./...                  # race detector (needs the C compiler)

  go run ./week1/Day1_Models_and_Frameworks_Landscape/labs
  cd week1/Day1_Models_and_Frameworks_Landscape/labs/solution && go run .   # key-free
  cd demo/adk-quickstart && make verify                                    # nested module

  pi-go --mode print "hello"   # multi-provider client the labs import
  kind create cluster --name ai-ae-labs     # docker-in-docker; kubectl + helm ready

Provider keys
  Copy apps/.env-example to apps/.env and fill in whichever keys you have.
  Keys are NOT forwarded from the host on purpose: adkenv.Load only sets a key
  that is not already in the environment, so a forwarded-but-empty variable
  would shadow apps/.env and silently break the documented path.

The agentgateway demo stack
  cd demo/1_ai-gateway && docker compose up -d
  Then: agentgateway :4000, mock-llm :8080, Jaeger :16686, Prometheus :9090,
  Grafana :3000. The mock backend needs no API key, so the whole DZ 1 chain
  (agent -> gateway -> backend -> trace -> metric) runs offline and free.

Notes
  * Ollama, if you run it, is on the HOST: use http://host.docker.internal:11434.
    That name resolves if the host runs Docker Desktop or OrbStack; on bare Linux
    Docker it does not — use the host's routable address instead.
  * Commit signing is not provisioned. If you sign with a host-only helper
    (1Password's op-ssh-sign, say), commit from the host or configure a key here.
EOF
