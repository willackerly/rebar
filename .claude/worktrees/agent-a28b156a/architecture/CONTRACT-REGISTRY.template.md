# Contract Registry

Index of all architecture contracts. Kept in sync with the actual
`CONTRACT-*.md` files in this directory.

<!-- Regenerate this list with:
     ls architecture/CONTRACT-*.md | grep -v TEMPLATE | grep -v REGISTRY
-->

---

## Services

<!-- Top-level system boundaries -->

| ID | Contract | Version | Status | Description |
|----|----------|---------|--------|-------------|
<!-- Example:
| S1 | [CONTRACT-S1-AUTH](CONTRACT-S1-AUTH.1.0.md) | 1.0 | active | Authentication and session management |
| S2 | [CONTRACT-S2-API-GATEWAY](CONTRACT-S2-API-GATEWAY.1.0.md) | 1.0 | active | API gateway and routing |
-->

## Components

<!-- Internal modules with defined interfaces -->

| ID | Contract | Version | Status | Description |
|----|----------|---------|--------|-------------|
<!-- Example:
| C1 | [CONTRACT-C1-BLOBSTORE](CONTRACT-C1-BLOBSTORE.2.1.md) | 2.1 | active | Encrypted blob storage |
| C2 | [CONTRACT-C2-RELAY](CONTRACT-C2-RELAY.1.0.md) | 1.0 | active | Blind message relay |
-->

## Interfaces

<!-- Shared contracts between components -->

| ID | Contract | Version | Status | Description |
|----|----------|---------|--------|-------------|
<!-- Example:
| I1 | [CONTRACT-I1-SESSION](CONTRACT-I1-SESSION.1.0.md) | 1.0 | active | Session lifecycle (check-out/check-in) |
-->

## Protocols

<!-- Wire formats and messaging contracts -->

| ID | Contract | Version | Status | Description |
|----|----------|---------|--------|-------------|
<!-- Example:
| P1 | [CONTRACT-P1-SIGNALING](CONTRACT-P1-SIGNALING.1.0.md) | 1.0 | active | WebRTC signaling protocol |
-->

---

## Quick Audit

```bash
# Contracts with no implementing code (orphaned contracts)
for f in architecture/CONTRACT-*.md; do
  id=$(basename "$f" .md | sed 's/CONTRACT-//')
  count=$(grep -rn "CONTRACT:$id" --include="*.go" --include="*.ts" . 2>/dev/null | wc -l)
  [ "$count" -eq 0 ] && echo "ORPHAN: $f (0 implementations)"
done

# Code with contract refs pointing to non-existent contracts
grep -rn "CONTRACT:" --include="*.go" --include="*.ts" . | while read line; do
  ref=$(echo "$line" | grep -o 'CONTRACT:[A-Z0-9-]*\.[0-9]*\.[0-9]*' | sed 's/CONTRACT://')
  [ -n "$ref" ] && [ ! -f "architecture/CONTRACT-${ref}.md" ] && echo "BROKEN REF: $line"
done
```
