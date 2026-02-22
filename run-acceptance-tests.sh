#!/usr/bin/env bash
set -euo pipefail
rm -rf acceptance-pipeline/ir/
go run ./acceptance/cmd/pipeline -action=run
