#!/usr/bin/env bash
# scripts/dev_up.sh
# Jalankan docker-compose dev

set -e
docker compose -f deployments/compose/docker-compose.dev.yaml up -d --build
