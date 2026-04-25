#!/usr/bin/env node
/**
 * Tag-to-CI coverage check (REBAR template).
 *
 * Source: rebar/feedback/2026-04-22-testing-rigor-six-moments.md §Proposal 5
 *
 * Any `@<tag>` that appears in a Playwright/vitest/jest spec file must
 * have a path to CI:
 *
 *   1. A package.json script command exists that would run the tag
 *      (either via explicit `--grep @tag` OR via a config whose `grep`
 *      pattern matches the tag).
 *   2. That script is invoked by at least one CI workflow job.
 *
 * Failure mode this catches: a brand-new `@security-audit` tag lands
 * with NO path to CI. The suite passes locally; the tag is dead in CI;
 * regressions slip past review.
 *
 * On first run in pdf-signer-web, this surfaced 35 pre-existing orphan
 * tags. Lifting it into REBAR templates gives every adopter the same
 * mechanical floor.
 *
 * Usage (from repo root):
 *   node scripts/check-tag-ci-coverage.mjs              # report + exit 1 on gap
 *   node scripts/check-tag-ci-coverage.mjs --verbose    # show every tag's path
 *   node scripts/check-tag-ci-coverage.mjs --strict     # also flag allowlisted tags
 *
 * Configure via scripts/tag-ci-allowlist.json (optional). Each entry maps
 * a `@tag` to a reason string explaining why it's not in CI (e.g.,
 * `@local-only`, `@manual-visual`).
 *
 * Generic — works in any repo. Discovers spec dirs, config files, and
 * package.json files automatically. Override discovery with env vars
 * REBAR_SPEC_GLOBS, REBAR_PKG_PATH if defaults are wrong.
 */
import { readFileSync, readdirSync, statSync, existsSync } from 'node:fs';
import { join, relative, dirname } from 'node:path';

const REPO = process.cwd();
const VERBOSE = process.argv.includes('--verbose');
const STRICT = process.argv.includes('--strict');

const SPEC_PATTERN =
  process.env.REBAR_SPEC_PATTERN ?? '\\.(spec|test)\\.(ts|tsx|js|jsx|mjs)$';
const SKIP_DIRS = new Set([
  'node_modules', '.git', 'dist', 'build', 'out', '.next', '.nuxt',
  'coverage', '.turbo', '.cache', 'vendor', '__pycache__',
]);

const ALLOWLIST_PATH = join(REPO, 'scripts/tag-ci-allowlist.json');
const allowlist = existsSync(ALLOWLIST_PATH)
  ? JSON.parse(readFileSync(ALLOWLIST_PATH, 'utf8'))
  : {};
delete allowlist._doc;

// -------------------------------------------------------------------------
// File discovery
// -------------------------------------------------------------------------
function walk(dir, matcher, acc = []) {
  let entries;
  try { entries = readdirSync(dir); } catch { return acc; }
  for (const name of entries) {
    if (SKIP_DIRS.has(name)) continue;
    const full = join(dir, name);
    let stats;
    try { stats = statSync(full); } catch { continue; }
    if (stats.isDirectory()) walk(full, matcher, acc);
    else if (matcher(full)) acc.push(full);
  }
  return acc;
}

// -------------------------------------------------------------------------
// Tag extraction from spec files
// -------------------------------------------------------------------------
function extractTagsFromSpecs() {
  const re = new RegExp(SPEC_PATTERN);
  const specs = walk(REPO, (f) => re.test(f));
  const tagRegex = /@([a-zA-Z][a-zA-Z0-9_-]*)/g;
  const tags = new Map(); // tag -> Set<file>
  for (const spec of specs) {
    let src;
    try { src = readFileSync(spec, 'utf8'); } catch { continue; }
    const lines = src.split('\n');
    for (const line of lines) {
      // Restrict to test/describe call sites — reduces false positives
      // from email addresses, decorators, JSX, etc.
      if (!/(\btest\.describe\(|\btest\(|\bit\(|\bdescribe\()/.test(line)) continue;
      for (const m of line.matchAll(tagRegex)) {
        const tag = `@${m[1]}`;
        if (!tags.has(tag)) tags.set(tag, new Set());
        tags.get(tag).add(relative(REPO, spec));
      }
    }
  }
  return tags;
}

// -------------------------------------------------------------------------
// package.json + Playwright config discovery
// -------------------------------------------------------------------------
function findPackageJsons() {
  return walk(REPO, (f) => /\/package\.json$/.test(f) || f.endsWith('/package.json'));
}

function findPlaywrightConfigs() {
  return walk(REPO, (f) => /playwright\.[^/]*\.config\.(ts|js|mjs)$/.test(f) || /playwright\.config\.(ts|js|mjs)$/.test(f));
}

function configGrepTags(cfgPath, allTags) {
  let src;
  try { src = readFileSync(cfgPath, 'utf8'); } catch { return new Set(); }
  const grepLine = src.match(/grep\s*:\s*\/([^/]+)\//);
  if (!grepLine) return new Set();
  const pattern = grepLine[1];
  const matched = new Set();
  for (const tag of allTags) {
    try {
      if (new RegExp(pattern).test(tag)) matched.add(tag);
    } catch { /* malformed pattern */ }
  }
  return matched;
}

function extractScriptsThatMatchTags(allTags) {
  // Returns: Map<scriptKey, Set<tag>>
  // scriptKey is "<package-relpath>:<scriptName>" so monorepos with
  // colliding names disambiguate.
  const pkgs = findPackageJsons();
  const allConfigs = findPlaywrightConfigs();

  // Pre-compute config grep coverage.
  const configCoverage = new Map(); // absolute config path -> Set<tag>
  for (const cfg of allConfigs) {
    configCoverage.set(cfg, configGrepTags(cfg, allTags));
  }

  const scriptTags = new Map();
  for (const pkgPath of pkgs) {
    let pkg;
    try { pkg = JSON.parse(readFileSync(pkgPath, 'utf8')); } catch { continue; }
    const scripts = pkg.scripts ?? {};
    const pkgDir = dirname(pkgPath);
    const pkgRel = relative(REPO, pkgPath) || 'package.json';

    for (const [name, cmdRaw] of Object.entries(scripts)) {
      const cmd = String(cmdRaw);
      // Only consider commands that look like test invocations.
      if (!/(playwright|vitest|jest|test:e2e|^test\b)/.test(cmd)) continue;

      const covered = new Set();

      // (a) Direct --grep on the command line
      for (const m of cmd.matchAll(/--grep\s+['"]?([^\s'"]+)/g)) {
        const pattern = m[1];
        for (const tag of allTags) {
          try {
            if (tag === pattern || new RegExp(pattern).test(tag)) covered.add(tag);
          } catch { /* malformed */ }
        }
      }

      // (b) --config FILE inherits config's `grep` tags
      const cfgArg = cmd.match(/--config\s+(\S+)/);
      if (cfgArg) {
        const cfgPath = join(pkgDir, cfgArg[1]);
        if (configCoverage.has(cfgPath)) {
          for (const t of configCoverage.get(cfgPath)) covered.add(t);
        }
      } else if (/playwright\s+test\b/.test(cmd)) {
        // (c) No --config → default playwright.config.ts in package dir
        for (const ext of ['ts', 'js', 'mjs']) {
          const defaultCfg = join(pkgDir, `playwright.config.${ext}`);
          if (configCoverage.has(defaultCfg)) {
            for (const t of configCoverage.get(defaultCfg)) covered.add(t);
            break;
          }
        }
      }

      scriptTags.set(`${pkgRel}:${name}`, covered);
    }
  }
  return scriptTags;
}

// -------------------------------------------------------------------------
// CI workflow parsing — which scripts are actually invoked?
// -------------------------------------------------------------------------
function extractScriptsInvokedByCI() {
  const workflows = walk(
    join(REPO, '.github/workflows'),
    (f) => /\.ya?ml$/.test(f),
  );
  const invoked = new Set(); // bare script names (no package prefix)
  const invokers = ['pnpm', 'npm run', 'yarn', 'bun run'];
  for (const wf of workflows) {
    let src;
    try { src = readFileSync(wf, 'utf8'); } catch { continue; }
    for (const tool of invokers) {
      // Match `pnpm [--filter X] foo:bar` or `npm run foo:bar`
      const re = new RegExp(`\\b${tool.replace(/\s+/g, '\\s+')}(?:\\s+--\\S+\\s+\\S+)*\\s+([\\w:-]+)`, 'g');
      for (const m of src.matchAll(re)) {
        const candidate = m[1];
        // Filter to plausible test scripts.
        if (/^(test|e2e)/i.test(candidate) || candidate.includes('test:')) {
          invoked.add(candidate);
        }
      }
    }
  }
  return invoked;
}

// -------------------------------------------------------------------------
// Analyze coverage
// -------------------------------------------------------------------------
function main() {
  const specTags = extractTagsFromSpecs();
  if (specTags.size === 0) {
    console.log('check-tag-ci-coverage: no @tags found in spec files (nothing to verify).');
    process.exit(0);
  }

  const allTags = new Set(specTags.keys());
  const scriptTags = extractScriptsThatMatchTags(allTags);
  const invokedScripts = extractScriptsInvokedByCI();

  // tag -> Set<scriptKey>
  const tagScripts = new Map();
  for (const [scriptKey, covers] of scriptTags) {
    for (const tag of covers) {
      if (!tagScripts.has(tag)) tagScripts.set(tag, new Set());
      tagScripts.get(tag).add(scriptKey);
    }
  }

  const report = [];
  const gaps = [];
  const allowed = [];
  for (const tag of [...allTags].sort()) {
    const specFiles = specTags.get(tag);
    const scripts = tagScripts.get(tag) ?? new Set();
    // Match scripts to invoked-in-CI by their bare script name (after ":").
    const ciScripts = [...scripts].filter((s) => {
      const bare = s.includes(':') ? s.split(':').slice(1).join(':') : s;
      return invokedScripts.has(bare);
    });
    const isAllowed = Object.prototype.hasOwnProperty.call(allowlist, tag);

    const status = ciScripts.length > 0 ? 'covered'
      : isAllowed ? 'allowed'
      : scripts.size > 0 ? 'script-only'
      : 'orphan';

    const row = {
      tag, specs: specFiles.size,
      scripts: [...scripts], ciScripts,
      allowed: isAllowed, allowReason: isAllowed ? allowlist[tag] : null,
      status,
    };
    report.push(row);
    if (status === 'allowed') allowed.push(row);
    else if (status !== 'covered') gaps.push(row);
  }

  if (VERBOSE) {
    console.log('\nTag-to-CI coverage report:\n');
    for (const r of report) {
      const icon =
        r.status === 'covered' ? '\x1b[32m✓\x1b[0m'
        : r.status === 'allowed' ? '\x1b[34m-\x1b[0m'
        : r.status === 'script-only' ? '\x1b[33m~\x1b[0m'
        : '\x1b[31m✗\x1b[0m';
      console.log(`${icon} ${r.tag}  (${r.specs} spec(s))`);
      if (r.scripts.length > 0) console.log(`    scripts: ${r.scripts.join(', ')}`);
      if (r.ciScripts.length > 0) console.log(`    CI jobs: ${r.ciScripts.join(', ')}`);
      else if (r.status === 'allowed') console.log(`    allowed: ${r.allowReason}`);
      else if (r.scripts.length > 0) console.log(`    CI jobs: \x1b[31m(none — script exists but not invoked by CI)\x1b[0m`);
      else console.log(`    CI jobs: \x1b[31m(none — no script matches this tag)\x1b[0m`);
    }
    console.log('');
  }

  console.log(`Checked ${allTags.size} @tag(s) across ${specTags.size} unique tag bucket(s).`);
  console.log(`  ✓ covered:      ${report.filter((r) => r.status === 'covered').length}`);
  console.log(`  - allowed:      ${allowed.length}`);
  console.log(`  ~ script-only:  ${report.filter((r) => r.status === 'script-only').length}`);
  console.log(`  ✗ orphan:       ${report.filter((r) => r.status === 'orphan').length}`);

  if (gaps.length > 0) {
    console.log(`\n\x1b[31mFAIL: ${gaps.length} tag(s) with no path to CI and not allowlisted:\x1b[0m`);
    for (const g of gaps) {
      console.log(`  - ${g.tag}  (${g.status})  specs: ${[...specTags.get(g.tag)].join(', ')}`);
    }
    console.log(`
Fix (pick one):
  (a) Add an npm/pnpm script that runs this tag (e.g., \`--grep ${gaps[0].tag}\`),
      then invoke it from .github/workflows/*.yml
  (b) If the tag is intentionally not in CI, add it to scripts/tag-ci-allowlist.json
      with a reason string
  (c) Remove the orphaned tag from the spec file(s)
`);
    process.exit(1);
  }

  if (STRICT && allowed.length > 0) {
    console.log(`\n\x1b[33m--strict: ${allowed.length} allowlisted tag(s) — review whether these should be wired to CI:\x1b[0m`);
    for (const a of allowed) {
      console.log(`  - ${a.tag}  reason: ${a.allowReason}`);
    }
    process.exit(1);
  }

  console.log('\n\x1b[32mAll @tags have a path to CI (or are allowlisted with a reason).\x1b[0m');
  process.exit(0);
}

main();
