#!/usr/bin/env bash
# Provision the ai-ae-labs dev container. Run once by onCreateCommand.
#
# Adapted from pi-go's .devcontainer/post-create.sh. Everything here is either a
# tool `task check` invokes, or one the course material tells learners to use.
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
# `task test` and `task check` both run `go test -race`, and -race is
# cgo-backed: without a C compiler it fails outright with
#   go: -race requires cgo; enable cgo by setting CGO_ENABLED=1
# rather than degrading to a non-race run. Verified on golang:alpine with no gcc.
#
# This repo needs no cgo for its own code (CGO_ENABLED=0 builds everywhere,
# including the demo/1_ai-gateway Dockerfile), so build-essential exists purely
# to make the race detector link.

echo "==> Go tools"
# gopls: the Go language server, and what the VS Code Go extension drives.
go install golang.org/x/tools/gopls@latest
# dlv: debugging from the IDE.
go install github.com/go-delve/delve/cmd/dlv@latest

echo "==> task"
# The repo's command surface (Taskfile.yml): task check, task build:all, task
# run. Pinned to the version the host and docs/devenv-mac.md use (3.53.1).
go install github.com/go-task/task/v3/cmd/task@v3.53.1

echo "==> gitleaks"
# MANDATORY, not a convenience.
#
# .githooks/pre-commit is `gitleaks git --staged … || { echo blocked; exit 1; }`.
# When gitleaks is absent the shell reports "command not found", the `||` branch
# fires, and EVERY commit is refused with a message about a found secret — which
# reads like a real leak and is not one. Verified by running the hook with a
# stripped PATH.
#
# Module path is github.com/zricethezav/gitleaks/v8, not github.com/gitleaks/…
# upstream renamed the org but the Go module path was never moved; the
# github.com/gitleaks/gitleaks/v8 path resolves only to ancient v8.15.3.
go install github.com/zricethezav/gitleaks/v8@v8.30.1

echo "==> mono-go-mcp"
# week1/Day2 labs attach this MCP server over stdio; mcptool.go resolves it from
# $GOPATH/bin by name and, when missing, reports an explicit error instead of
# silently running with local tools only. Optional for the task gates, so a
# failure here is a warning rather than a broken container.
go install github.com/dimetron/mono-go-mcp/cmd/mono-go-mcp@latest ||
  echo "    (skipped: mono-go-mcp install failed — only the week1/Day2 MCP lab needs it)"

echo "==> pi-go CLI"
# The multi-provider client the labs import as a library (github.com/dimetron/pi-go).
# Installed from the published release binaries, not `go install`: that module's
# go.mod carries `replace` directives for its vendored third_party/ SDKs, and
# `go install pkg@version` refuses any module whose build would differ from
# being the main module:
#   The go.mod file for the module providing named packages contains one or more
#   replace directives.
# Building from a clone would work but pulls the whole tree; the tarball is
# faster and is what a learner would grab anyway.
#
# Keep PI_GO_VERSION in sync with the github.com/dimetron/pi-go requirement in
# go.mod, or the CLI and the library the labs compile against will disagree.
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
# The root module and every nested one, so `task build:all` is not a cold start.
go mod download
for m in demo/7_adk-go-evals demo/adk-quickstart examples; do
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
# Fail the build loudly if a gate the repo depends on cannot actually run, rather
# than at the first `task check` in a lesson.
for tool in go task gitleaks rg docker git gh; do
	printf '    %-12s %s\n' "$tool" "$(command -v "$tool" || echo 'MISSING')"
done
(cd "$(pwd)" && task --version >/dev/null 2>&1) || echo "    task present but not runnable here"

cat <<'EOF'

ai-ae-labs dev container ready.

  task                 task check          task build:all
  task test            task cover          task run PKG=./week1/Day1_Models_and_Frameworks_Landscape/labs

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
