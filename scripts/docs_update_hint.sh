#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

if ! git rev-parse --is-inside-work-tree >/dev/null 2>&1; then
  echo "No git repository detected. Review AGENTS.md and .ai-context manually if contracts changed."
  exit 0
fi

CHANGED_FILES=$(git diff --name-only HEAD -- .)

if [ -z "$CHANGED_FILES" ]; then
  CHANGED_FILES=$(git ls-files --others --exclude-standard)
fi

if [ -z "$CHANGED_FILES" ]; then
  echo "No changed files detected."
  echo "Docs do not need updates unless you intentionally changed contracts, architecture, or operations."
  exit 0
fi

need_agents=0
need_arch=0
need_domain=0
need_ops=0
need_layer_rules=0

echo "Changed files:"
printf '%s\n' "$CHANGED_FILES"

for file in $CHANGED_FILES; do
  case "$file" in
    AGENTS.md|.ai-context/*)
      need_agents=1
      need_arch=1
      need_domain=1
      need_ops=1
      need_layer_rules=1
      ;;
    internal/router.go|internal/features/*/routers/*)
      need_arch=1
      need_layer_rules=1
      ;;
    internal/features/*/handlers/*|internal/features/*/schemas/*)
      need_domain=1
      ;;
    internal/features/*/services/*|internal/features/*/repositories/*|internal/features/*/models/*|migrations/*)
      need_domain=1
      ;;
    internal/features/realtime/*)
      need_domain=1
      need_arch=1
      ;;
    internal/core/*|cmd/server/main.go|docker-compose.yml|Makefile|.env.example)
      need_ops=1
      need_arch=1
      ;;
  esac
done

echo
echo "Docs only need updates when a contract changed."
echo "Skip doc edits for implementation-only fixes that do not change behavior, boundaries, commands, or invariants."
echo
echo "Suggested files to review:"

if [ "$need_agents" -eq 1 ] || [ "$need_layer_rules" -eq 1 ]; then
  echo "- AGENTS.md"
  echo "- .ai-context/agent-rules/agent-instructions.md"
fi

if [ "$need_layer_rules" -eq 1 ]; then
  echo "- .ai-context/agent-rules/instructions/*.instructions.md"
fi

if [ "$need_arch" -eq 1 ]; then
  echo "- .ai-context/docs/architecture.md"
fi

if [ "$need_domain" -eq 1 ]; then
  echo "- .ai-context/docs/domain-rules.md"
fi

if [ "$need_ops" -eq 1 ]; then
  echo "- .ai-context/docs/operations.md"
fi

echo
echo "After any doc edits, run: make docs-check"
