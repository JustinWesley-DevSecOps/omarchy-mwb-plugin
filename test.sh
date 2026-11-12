#!/usr/bin/env bash
# Runs the helper's Go tests and static checks on the QML security rules.
set -euo pipefail
cd "$(dirname "$0")"

(cd helper && gofmt -l . | (! grep .) && go vet ./... && go test ./...)

fail=0
# The key must never be part of an argv array handed to run().
if grep -nE 'run\(\[[^]]*(draftKey|\.key|key)' Panel.qml; then
  echo "FAIL: key referenced in a helper argv" >&2; fail=1
fi
# Every Text element must opt out of rich text.
texts=$(grep -cE '^\s*Text \{' Panel.qml || true)
plain=$(grep -cE 'textFormat: Text.PlainText' Panel.qml || true)
if [ "$texts" -ne "$plain" ]; then
  echo "FAIL: $texts Text elements but $plain PlainText" >&2; fail=1
fi
# No shell strings.
if grep -nE '"(sh|bash)"|-c"' Panel.qml Helper.qml; then
  echo "FAIL: shell invocation in QML" >&2; fail=1
fi
# The key is cleared when the panel closes.
grep -q 'draftKey = ""' Panel.qml || { echo "FAIL: key not cleared" >&2; fail=1; }
[ "$fail" -eq 0 ] && echo "static checks ok"
exit "$fail"
