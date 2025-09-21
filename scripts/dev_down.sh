#!/usr/bin/env bash
# scripts/dev_down.sh
# Stop docker-compose dev

set -e
docker compose -f deployments/compose/docker-compose.dev.yaml down --remove-orphans
