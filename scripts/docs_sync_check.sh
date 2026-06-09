#!/usr/bin/env sh
set -eu

ROOT_DIR=$(CDPATH= cd -- "$(dirname -- "$0")/.." && pwd)
cd "$ROOT_DIR"

failed=0

check_file() {
  if [ ! -f "$1" ]; then
    echo "Missing required file: $1"
    failed=1
  fi
}

echo "==> Checking required agent context files"
check_file "AGENTS.md"
check_file ".ai-context/agent-rules/agent-instructions.md"
check_file ".ai-context/docs/architecture.md"
check_file ".ai-context/docs/domain-rules.md"
check_file ".ai-context/docs/operations.md"

echo "==> Checking instruction frontmatter"
for file in .ai-context/agent-rules/instructions/*.md; do
  [ -e "$file" ] || continue
  if ! grep -q '^applyTo: "' "$file"; then
    echo "Missing applyTo frontmatter: $file"
    failed=1
  fi
done

echo "==> Checking AGENTS.md required sections"
if ! grep -q '^## AI Context Map$' AGENTS.md; then
  echo "AGENTS.md is missing section: AI Context Map"
  failed=1
fi

if ! grep -q '^## Definition of Done$' AGENTS.md; then
  echo "AGENTS.md is missing section: Definition of Done"
  failed=1
fi

echo "==> Checking unresolved template placeholders"
if grep -R -n '{{' AGENTS.md .ai-context >/tmp/docs_sync_placeholders.txt 2>/dev/null; then
  echo "Unresolved template placeholders found:"
  cat /tmp/docs_sync_placeholders.txt
  failed=1
fi
rm -f /tmp/docs_sync_placeholders.txt

if [ "$failed" -ne 0 ]; then
  echo "Documentation consistency check failed."
  exit 1
fi

echo "Documentation consistency check passed."
