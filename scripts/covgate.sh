#!/bin/sh
# covgate.sh — per-package statement coverage gate (AGENTS.md §4).
#
# Usage:
#   scripts/covgate.sh                    # gate every package at 85%
#   scripts/covgate.sh 95                 # custom threshold
#   scripts/covgate.sh 85 week1/          # gate only packages matching a substring
#   scripts/covgate.sh -s 85              # also fail packages that have no tests
#
# Coverage is measured EXCLUDING func main(), which go test can never execute.
# Without that exclusion the threshold is unreachable by arithmetic: in a lab
# whose main() holds a quarter of the statements, raw coverage is capped below
# 80% however thoroughly the real logic is tested. The exclusion is not a
# loophole — main() must still be thin, because every statement written inline
# in it is logic no test can ever reach. See AGENTS.md §4.
#
# Exit status: 0 if every gated package meets the threshold, 1 otherwise.
set -eu

# Positional: the FIRST bare argument is the threshold, the second is a package
# filter. Tracking position rather than "is it empty" matters — testing for an
# empty pattern makes the second argument overwrite the threshold, which
# silently gates every package at a path-shaped threshold.
threshold=85
strict=0
pattern=""
bare=0
for arg in "$@"; do
	case "$arg" in
	-s) strict=1 ;;
	*)
		if [ "$bare" = 0 ]; then
			threshold="$arg"
		else
			pattern="$arg"
		fi
		bare=$((bare + 1))
		;;
	esac
done

cd "$(dirname "$0")/.."

prof=$(mktemp)
trap 'rm -f "$prof"' EXIT

failed=0

gate() {
	pkg="$1"
	short="${pkg#github.com/dimetron/ai-eng-course/labs/}"

	# Ask the toolchain whether the package has tests at all. A package with
	# none still prints "coverage: 0.0%" and writes a full coverprofile, so the
	# output alone cannot tell "untested" from "tested and failing"; only
	# go list knows.
	testfiles=$(go list -f '{{len .TestGoFiles}}{{len .XTestGoFiles}}' "$pkg" 2>/dev/null || echo 00)
	if [ "$testfiles" = "00" ]; then
		printf 'UNTESTED     %-22s %s\n' "(no tests)" "$short"
		[ "$strict" = 1 ] && failed=1
		return 0
	fi

	if ! out=$(go test -count=1 -coverprofile="$prof" "$pkg" 2>&1); then
		printf 'FAIL(test)   %-22s %s\n' "" "$short"
		printf '%s\n' "$out" | sed 's/^/               /'
		failed=1
		return 0
	fi

	# Drop func main()'s statements from both numerator and denominator.
	mainline=$(go tool cover -func="$prof" | awk -F: '$0 ~ /[[:space:]]main[[:space:]]/ {print $2; exit}')
	pct=$(awk -v ml="${mainline:-0}" '
		NR==1 { next }
		{ split($1,a,":"); n=split(a[1],b,"/"); file=b[n]; start=a[2]+0 }
		file=="main.go" && ml>0 && start>=ml { next }
		{ t+=$2; if ($3+0>0) c+=$2 }
		END { printf "%.1f", (t ? 100*c/t : 100) }
	' "$prof")

	# awk prints 1 when below the threshold, so this is a plain string compare.
	below=$(awk -v p="$pct" -v t="$threshold" 'BEGIN { print (p < t) ? 1 : 0 }')
	if [ "$below" = "1" ]; then
		printf 'BELOW        %6s%% (want >=%s%%)  %s\n' "$pct" "$threshold" "$short"
		failed=1
	else
		printf 'ok           %6s%%             %s\n' "$pct" "$short"
	fi
}

if [ -n "$pattern" ]; then
	pkgs=$(go list ./... 2>/dev/null | grep -F "$pattern" || true)
else
	pkgs=$(go list ./... 2>/dev/null)
fi

for pkg in $pkgs; do
	gate "$pkg"
done

exit "$failed"
