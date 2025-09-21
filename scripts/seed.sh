#!/usr/bin/env bash
# scripts/seed.sh
# Isi data dummy ke database

set -e
docker exec -i mcp-mysql mysql -umcpuser -psecret mcp < db/mysql/seed.sql
