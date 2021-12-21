#!/usr/bin/env bash

set -x
set -o errexit
set -o pipefail
set -o nounset

function check_linters {
  if ! command -v golangci-lint &> /dev/null || ! command -v yamllint &> /dev/null; then
    echo false
  fi
  echo true
}

function get_linters {
  if [[ $# -ne 1 ]]; then
    echo "get_linters expects LINTER_SETUP_DIR argument"
    exit 1
  fi
  local lintbin=${1}

  if [[ $(check_linters) == true ]]; then
    echo "linters are in the PATH, skipping..."
    exit 0
  fi

  if [[ -x "${lintbin}/bin/yamllint" ]]; then
    echo "yamllint is installed, skipping..."
  else
    pip3 install --target "${lintbin}" yamllint
  fi

  if [[ -x "${lintbin}/bin/golangci-lint" ]]; then
    echo "golangci-lint is installed, skipping..."
  else
    curl -sSfL https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s -- -b "${lintbin}/bin" v1.43.0
  fi
}