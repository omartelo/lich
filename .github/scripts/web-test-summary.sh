#!/usr/bin/env bash
# Runs the frontend (Vitest) suite with coverage, appends file/pass/fail counts
# and line coverage to the GitHub Actions job summary, and exits with the
# suite's own status. node_modules must already be installed. Falls back to
# stdout when run outside Actions.
set -uo pipefail

cd "$(dirname "$0")/../../frontend"
out="$(mktemp)"

pnpm exec vitest run --coverage --reporter=default --reporter=json --outputFile.json="$out"
code=$?

read -r files passed failed < <(jq -r '
  [ (.testResults | length), .numPassedTests, .numFailedTests ] | @tsv
' "$out")

cov="$(jq -r '.total.lines.pct' coverage/coverage-summary.json 2>/dev/null || echo n/a)"
[ "$cov" != "n/a" ] && cov="${cov}%"

summary="${GITHUB_STEP_SUMMARY:-/dev/stdout}"
{
  echo "### Frontend (Vitest)"
  echo ""
  echo "| Files | Passed | Failed | Coverage (lines) |"
  echo "|------:|-------:|-------:|-----------------:|"
  echo "| ${files:-0} | ${passed:-0} | ${failed:-0} | ${cov:-n/a} |"
  echo ""
} >>"$summary"

exit "$code"
