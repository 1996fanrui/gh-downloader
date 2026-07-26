#!/usr/bin/env bash
# run_test.sh — Run lints and/or tests for this project.
#
# Usage:
#   bash scripts/run_test.sh          # run all (lint + vet + test)
#   bash scripts/run_test.sh lint     # run lints only
#   bash scripts/run_test.sh vet      # run Go vet only
#   bash scripts/run_test.sh test     # run tests only
#   bash scripts/run_test.sh all      # run all (lint + vet + test)

set -e

info() { echo "[INFO] $*"; }

run_lints() {
    info "Running pre-commit hooks..."
    pre-commit run --all-files

    for f in scripts/lints/*.sh; do
        [[ -f "$f" ]] || continue
        info "Running $f..."
        bash "$f"
    done
}

go_packages() {
    go list ./... 2>/dev/null
}

run_go_vet() {
    local packages
    packages="$(go_packages)"
    if [[ -z "$packages" ]]; then
        info "No Go packages found; skipping Go vet."
        return
    fi

    info "Running Go vet..."
    go vet ./...
}

run_tests() {
    local packages
    packages="$(go_packages)"
    if [[ -z "$packages" ]]; then
        info "No Go packages found; skipping Go tests."
        return
    fi

    info "Running Go tests..."
    go test ./...
}

case "${1:-all}" in
    lint)  run_lints ;;
    vet)   run_go_vet ;;
    test)  run_tests ;;
    all)   run_lints; run_go_vet; run_tests ;;
    *)     echo "Usage: bash scripts/run_test.sh [lint|vet|test|all]"; exit 1 ;;
esac
