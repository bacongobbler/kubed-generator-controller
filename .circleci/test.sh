#!/usr/bin/env bash

set -euo pipefail
IFS=$'\n\t'
ROOT="${BASH_SOURCE[0]%/*}/.."

cd "$ROOT"

run_unit_tests() {
  echo "Running unit tests"
  make test-cover
}

run_style_checks() {
  echo "Running style checks"
  make test-lint
}

case "${CIRCLE_NODE_INDEX-0}" in
  0) run_unit_tests   ;;
  1) run_style_checks ;;
esac
