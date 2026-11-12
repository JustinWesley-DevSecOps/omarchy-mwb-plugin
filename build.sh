#!/usr/bin/env bash
# Builds the helper into bin/mwbcfg.
set -euo pipefail
cd "$(dirname "$0")/helper"
mkdir -p ../bin
go build -trimpath -o ../bin/mwbcfg .
echo "built $(cd .. && pwd)/bin/mwbcfg"
