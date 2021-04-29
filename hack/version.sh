#!/bin/sh

hash=$(git rev-list -1 HEAD)
tag=$(git describe --tags --exact-match "${hash}" 2> /dev/null)

if [ $? -eq 0 ]; then
    echo "${tag}"
else
    echo "${hash}"
fi
