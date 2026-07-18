#!/usr/bin/env bash
# Install stell CLI, scaffold ~/.stell/agent, optionally clone sources for development.
#
#   curl -fsSL https://raw.githubusercontent.com/stelmakhdigital/stell-coding/master/scripts/install.sh | bash
#   curl -fsSL …/scripts/install.sh | STELL_DEV=1 bash
#
# Env:
#   STELL_VERSION   module version for go install (default: latest)
#   STELL_DEV       set to 1 to clone sources
#   STELL_SRC_DIR   clone destination (default: $HOME/src/stell-coding)
#   STELL_AGENT_DIR agent home (default: ~/.stell/agent)

set -euo pipefail

MODULE="github.com/stelmakhdigital/stell-coding"
BIN_PKG="${MODULE}/cmd/stell"
REPO_URL="https://github.com/stelmakhdigital/stell-coding.git"
STELL_VERSION="${STELL_VERSION:-latest}"
STELL_SRC_DIR="${STELL_SRC_DIR:-$HOME/src/stell-coding}"
STELL_AGENT_DIR="${STELL_AGENT_DIR:-$HOME/.stell/agent}"

info()  { printf '==> %s\n' "$*"; }
warn()  { printf 'warning: %s\n' "$*" >&2; }
die()   { printf 'error: %s\n' "$*" >&2; exit 1; }

need_cmd() {
	command -v "$1" >/dev/null 2>&1 || die "'$1' is required but not found. $2"
}

go_bin_dir() {
	local gobin gopath
	gobin="$(go env GOBIN 2>/dev/null || true)"
	gobin="$(printf '%s' "$gobin" | tr -d '\r\n')"
	if [[ -n "$gobin" ]]; then
		printf '%s\n' "$gobin"
		return
	fi
	gopath="$(go env GOPATH 2>/dev/null || true)"
	gopath="$(printf '%s' "$gopath" | tr -d '\r\n')"
	gopath="${gopath%%:*}"
	if [[ -z "$gopath" ]]; then
		gopath="${HOME}/go"
	fi
	printf '%s\n' "${gopath}/bin"
}

ensure_go() {
	need_cmd go "Install Go 1.24+ from https://go.dev/dl/"
	local ver major minor
	ver="$(go env GOVERSION 2>/dev/null || go version | awk '{print $3}')"
	ver="${ver#go}"
	major="${ver%%.*}"
	minor="${ver#*.}"
	minor="${minor%%.*}"
	if [[ -z "$major" || -z "$minor" ]] || (( major < 1 )) || (( major == 1 && minor < 24 )); then
		die "Go 1.24+ required (found go${ver}). See https://go.dev/dl/"
	fi
}

install_binary() {
	local pkg="${BIN_PKG}@${STELL_VERSION}"
	info "Installing stell (${pkg})"
	# Fresh tags often hit proxy/sumdb negative cache ("unknown revision").
	# Prefer direct fetch; fall back with GOSUMDB=off if sum.golang.org lags.
	if ! GOTOOLCHAIN=auto GOPROXY="${GOPROXY:-direct}" go install "$pkg"; then
		warn "go install failed; retrying with GOPROXY=direct GOSUMDB=off (sumdb/proxy lag)"
		GOTOOLCHAIN=auto GOPROXY=direct GOSUMDB=off go install "$pkg" \
			|| die "go install ${pkg} failed"
	fi
}

check_path() {
	local bin_dir stell_path
	bin_dir="$(go_bin_dir)"
	stell_path="${bin_dir}/stell"

	if command -v stell >/dev/null 2>&1; then
		info "stell is on PATH: $(command -v stell)"
		return
	fi

	if [[ -x "$stell_path" ]]; then
		warn "stell installed at ${stell_path} but not on PATH"
		printf '\nAdd to your shell profile:\n\n  export PATH="%s:$PATH"\n\n' "$bin_dir"
		return
	fi

	die "stell binary not found after install (expected ${stell_path})"
}

scaffold() {
	info "Scaffolding ${STELL_AGENT_DIR}"
	mkdir -p \
		"${STELL_AGENT_DIR}/extensions" \
		"${STELL_AGENT_DIR}/packages" \
		"${STELL_AGENT_DIR}/skills" \
		"${STELL_AGENT_DIR}/prompts" \
		"${STELL_AGENT_DIR}/themes" \
		"${STELL_AGENT_DIR}/context"
}

clone_sources() {
	need_cmd git "Install git to clone sources (STELL_DEV=1)."
	info "Cloning sources → ${STELL_SRC_DIR}"
	mkdir -p "$(dirname "$STELL_SRC_DIR")"

	if [[ -d "${STELL_SRC_DIR}/.git" ]]; then
		info "Repo already exists; pulling (ff-only)"
		if ! git -C "$STELL_SRC_DIR" pull --ff-only; then
			warn "git pull --ff-only failed; leaving ${STELL_SRC_DIR} unchanged"
		fi
	elif [[ -e "$STELL_SRC_DIR" ]]; then
		die "${STELL_SRC_DIR} exists but is not a git repository"
	else
		git clone --depth 1 "$REPO_URL" "$STELL_SRC_DIR"
	fi
}

print_next_steps() {
	printf '\nDone.\n\n'
	printf 'Next steps:\n'
	printf '  stell                          # interactive TUI\n'
	printf '  stell install <path|git:url>   # install a package\n'
	printf '  # or drop an extension into:\n'
	printf '  #   %s/extensions/\n' "$STELL_AGENT_DIR"
	if [[ "${STELL_DEV:-}" == "1" ]]; then
		printf '\nDevelopment:\n'
		printf '  cd %s\n' "$STELL_SRC_DIR"
		printf '  go run ./cmd/stell\n'
		printf '  go install ./cmd/stell\n'
	fi
	printf '\n'
}

main() {
	ensure_go
	need_cmd git "Install git (needed for go modules and optional STELL_DEV clone)."
	install_binary
	check_path
	scaffold
	if [[ "${STELL_DEV:-}" == "1" ]]; then
		clone_sources
	fi
	print_next_steps
}

main "$@"
