# REBAR CLI

Go binary providing integrity verification, enforced commits, agent execution
with sealed envelopes, and optional Ed25519 digital signatures.

## Build

```bash
cd cli
go build -o ../bin/rebar .
```

Requires Go 1.22+.

## Quick Start

```bash
rebar init                    # Bootstrap .rebar/ with salt, manifest
rebar verify                  # Check integrity of protected files
rebar status                  # Health dashboard

rebar commit -m "message"     # Enforced commit (no --no-verify)
rebar check                   # Run all enforcement checks
rebar push                    # Verify + check + push

rebar ask architect "question"  # Query agents (delegates to bin/ask)

rebar agent start --role developer "task"  # Sealed envelope execution
rebar agent finish <id>                    # Audit agent work
rebar agent list                           # Show active envelopes

rebar key init --identity "you@org.com"    # Generate signing keypair
rebar key trust --identity "ci" --role ci --pubkey <hex>
rebar sign --role steward --all-verified   # Sign protected files
rebar verify --signatures                  # Verify signatures
```

## Integrity Layers

```
Layer 0: File contents          "what is the data?"
Layer 1: SHA-256 hashes         "has it changed?"        ← rebar verify
Layer 2: Role HMACs             "did it go through CLI?" ← rebar commit
Layer 3: Digital signatures     "who attests to this?"   ← rebar sign
Layer 4: Policy enforcement     "do required roles agree?"← rebar verify --signatures
```

## Design

See [docs/REBAR-CLI-INTEGRITY.md](../docs/REBAR-CLI-INTEGRITY.md) for the
full design specification.
