#!/usr/bin/env bash
# scripts/healthcheck.sh
# Cek endpoint healthz

curl -s http://localhost:8080/healthz | jq .
