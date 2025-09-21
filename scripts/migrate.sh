#!/usr/bin/env bash
# scripts/migrate.sh
# Jalankan migration SQL ke MySQL container

set -e
docker exec -i mcp-mysql mysql -umcpuser -psecret mcp < db/mysql/schema.sql
