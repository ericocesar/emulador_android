#!/bin/bash
set -e

ROOT_DIR="$(cd "$(dirname "$0")" && pwd)"

cleanup() {
  echo ""
  echo "Shutting down..."
  kill $FRONTEND_PID $BACKEND_PID 2>/dev/null
  wait $FRONTEND_PID $BACKEND_PID 2>/dev/null
  echo "Done."
}
trap cleanup EXIT INT TERM

export JWT_SECRET="${JWT_SECRET:-$(grep JWT_SECRET "$ROOT_DIR/.env" 2>/dev/null | cut -d= -f2)}"

echo "==> Starting backend (gateway)..."
(cd "$ROOT_DIR/gateway" && go run .) &
BACKEND_PID=$!

echo "==> Starting frontend..."
(cd "$ROOT_DIR/frontend" && npm run dev) &
FRONTEND_PID=$!

echo ""
echo "  Frontend : http://localhost:5173"
echo "  Backend  : http://localhost:8080"
echo ""

wait
