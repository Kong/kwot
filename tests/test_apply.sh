#!/bin/bash
set -x
timeout 5 kwot apply --dry-run 2>&1
echo "Exit code: $?"
