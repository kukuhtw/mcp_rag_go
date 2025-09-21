#!/usr/bin/env bash
set -euo pipefail

echo "==> Stopping and removing all containers for this project..."
docker compose -f deployments/compose/docker-compose.dev.yaml down -v --remove-orphans

echo "==> Pruning unused Docker data (dangling containers, images, networks, and build cache)..."
docker system prune -af --volumes

echo "==> All clean. Next you can run: make up atau make demo-data"
