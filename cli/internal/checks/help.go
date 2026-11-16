package checks

// The scripts' --help text: their header comment, as `sed -n '2,/^$/p' "$0" | sed 's/^# \{0,1\}//'` prints it.
// Generated from scripts/*.sh; the --help parity cases keep it in step.
var scriptHelp = map[string]string{
	"check-fix-commit": "" +
		"check-fix-commit.sh — Gate G enforcement: fix:/regression: commits must\n" +
		"document where the symptom was reproduced.\n" +
		"\n" +
		"Source: feedback/processed/2026-04-24-process-gates-G-through-L.md §Gate G.\n" +
		"\n" +
		"Rule: any commit prefixed `fix:`, `fix(...)`, or `regression:` /\n" +
		"`regression(...)` MUST contain a `Reproduced on:` line in the body\n" +
		"referencing a SHA, deploy URL, log excerpt, or screenshot link. Without\n" +
		"this, the author is fixing a phantom — the user-reported symptom hasn't\n" +
		"been observed independently before fixing began.\n" +
		"\n" +
		"Modes:\n" +
		"  ./scripts/check-fix-commit.sh                       # check HEAD's commit msg\n" +
		"  ./scripts/check-fix-commit.sh <commit-msg-file>     # check given file (commit-msg hook usage)\n" +
		"  ./scripts/check-fix-commit.sh --range <sha>..<sha>  # audit a range of commits\n" +
		"  ./scripts/check-fix-commit.sh --warn                # flag but don't fail\n" +
		"\n" +
		"Exit code: 0 if all qualifying commits are clean, 1 if any violations.\n" +
		"\n" +
		"Bash 3.2 compatible.\n" +
		"\n",
	"check-bypass-flags": "" +
		"check-bypass-flags.sh — Gate I enforcement: bypass flags require ticket refs.\n" +
		"\n" +
		"Source: feedback/processed/2026-04-24-process-gates-G-through-L.md §Gate I.\n" +
		"\n" +
		"Rule: any commit whose body mentions a test-bypass flag (--skip-stress,\n" +
		"--skip-tests, --no-verify, --force, SKIP_TESTS=1, etc.) MUST contain a\n" +
		"`Bypass tickets:` line listing the broken-test IDs being deferred. Without\n" +
		"this, the bypass is a quiet escape hatch that ships regressions in\n" +
		"regression-fix PRs.\n" +
		"\n" +
		"The rule covers the COMMIT MESSAGE — it's the audit trail for why a bypass\n" +
		"was chosen. The actual flag invocation (e.g., wrapping promote-to-prod) is\n" +
		"project-specific and lives in your project's deploy scripts.\n" +
		"\n" +
		"Modes:\n" +
		"  ./scripts/check-bypass-flags.sh                       # check HEAD's commit msg\n" +
		"  ./scripts/check-bypass-flags.sh <commit-msg-file>     # check given file (commit-msg hook usage)\n" +
		"  ./scripts/check-bypass-flags.sh --range <sha>..<sha>  # audit a range of commits\n" +
		"  ./scripts/check-bypass-flags.sh --warn                # flag but don't fail\n" +
		"\n" +
		"Exit code: 0 if all qualifying commits are clean, 1 if any violations.\n" +
		"\n" +
		"Bash 3.2 compatible.\n" +
		"\n",
	"check-doc-refs": "" +
		"check-doc-refs.sh — Verify that every file referenced from a tracked *.md\n" +
		"is itself tracked in git.\n" +
		"\n" +
		"Catches a class of drift Tier 2 ci-check.sh misses: a load-bearing doc\n" +
		"(e.g., FEDERATION-STORIES-DRAFT.md) is added to the working tree, cited\n" +
		"from another doc, but never `git add`-ed. The current repo passes; a\n" +
		"fresh clone fails to resolve the link.\n" +
		"\n" +
		"Source: rebar/feedback/2026-04-21-filedag-cross-ref-and-federation-coord.md §1\n" +
		"\n" +
		"Usage:\n" +
		"  ./scripts/check-doc-refs.sh           # report broken refs, exit 1 if any\n" +
		"  ./scripts/check-doc-refs.sh --quiet   # only print failures\n" +
		"  ./scripts/check-doc-refs.sh --json    # JSON summary on stdout\n" +
		"\n" +
		"Heuristics:\n" +
		"  - Walks every tracked *.md file (`git ls-files '*.md'`).\n" +
		"  - Extracts markdown-link targets: `[text](TARGET)`.\n" +
		"  - Skips: external URLs (http*, mailto, tel, ftp), anchor-only refs (#x),\n" +
		"    home-relative refs (~/...), template placeholders ({...}), allowlisted\n" +
		"    paths from .rebar/doc-refs-allow.txt (one path per line, # for comments).\n" +
		"  - Resolves remaining targets relative to the source file's directory.\n" +
		"  - Fails if resolved target is not in `git ls-files`.\n" +
		"\n" +
		"Bash 3.2 compatible (macOS default).\n" +
		"\n",
	"check-decay-patterns": "" +
		"check-decay-patterns.sh — Flag soft-hardening patterns that look like\n" +
		"done work but ship a longer-fuse failure mode.\n" +
		"\n" +
		"Sources: rebar/feedback/2026-04-24-fidelity-decay-soft-hardening-patterns.md\n" +
		"         rebar/feedback/processed/2026-04-27-e2e-test-bypass-closed-loop-verification-drift.md (P8)\n" +
		"\n" +
		"An author cannot reliably self-audit hardening work because the author's\n" +
		"context (right now, with the test fresh) matches the consumer's context\n" +
		"(six months from now, scanning a dashboard). This script applies external\n" +
		"state — a structural lens — at commit time.\n" +
		"\n" +
		"Eight named patterns from the fidelity-decay feedback. The grep-detectable\n" +
		"subset is implemented; the semantic ones (hermeticity, env-name\n" +
		"plausibility) are left for the author's self-audit prompt linked from\n" +
		"AGENTS.template.md. P8 (demo-spec bypass) is the banned-pattern gate from\n" +
		"the e2e-bypass feedback: demo-claiming and uaks-declared specs must drive\n" +
		"the surface a user touches (see practices/test-fidelity.md, Gate 2).\n" +
		"\n" +
		"Modes:\n" +
		"  ./scripts/check-decay-patterns.sh              # scan staged files\n" +
		"  ./scripts/check-decay-patterns.sh --all        # scan everything tracked\n" +
		"  ./scripts/check-decay-patterns.sh --paths a b  # scan specific paths\n" +
		"  ./scripts/check-decay-patterns.sh --warn       # flag but don't fail\n" +
		"\n" +
		"Exit code: 0 if no findings (or --warn), 1 if findings in default mode.\n" +
		"\n" +
		"Bash 3.2 compatible (macOS default).\n" +
		"\n",
}
