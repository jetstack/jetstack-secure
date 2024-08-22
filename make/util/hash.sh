#!/usr/bin/env bash

set -eu -o pipefail

# This script is a wrapper for outputting purely the sha256 hash of the input file,
# ideally in a portable way.

sha256sum "$1" | cut -d" " -f1
