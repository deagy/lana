#!/bin/bash
# Simple test fixture for plugin execution tests
# Echoes arguments to stdout
for arg in "$@"; do
  echo "arg: $arg"
done
exit 0
