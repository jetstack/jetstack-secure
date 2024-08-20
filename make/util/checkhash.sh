#!/usr/bin/env bash

set -eu -o pipefail

# This script takes the hash of its first argument and verifies it against the
# hex hash given in its second argument

SHASUM=$(./make/util/hash.sh "$1")

# When running 'make learn-sha-tools', we don't want this script to fail.
# Instead we log what sha values are wrong, so the make.mk file can be updated.
if [ "$SHASUM" != "$2" ] && [ "${LEARN_FILE:-}" != "" ]; then
	echo "s/$2/$SHASUM/g" >> "${LEARN_FILE:-}"
	exit 0
fi

if [ "$SHASUM" != "$2"  ]; then
	echo "invalid checksum for \"$1\": wanted \"$2\" but got \"$SHASUM\""
	exit 1
fi
