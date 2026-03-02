#!/usr/bin/env bash

set -euo pipefail

cat <<'EOF'
Arda Web Gateway Phase 0 Bootstrap Checklist
===========================================
1) Backend:
   cd backend && go test ./...
2) Frontend dependencies:
   cd frontend && npm install
3) Generate API client:
   cd frontend && npm run api:generate
4) Local dev:
   make run-backend
   make dev-frontend
5) Docker parity:
   make docker-up
EOF
