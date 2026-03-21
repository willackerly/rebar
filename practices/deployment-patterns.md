# Deployment Patterns

**Referenced from AGENTS.md. Read before deploying to production.**

---

These patterns cause recurring production incidents in monorepo projects. Document the specifics for your project below.

## Static Frontend vs Backend Deploy

In monorepos with separate frontend and backend packages, the default deploy command often targets the wrong service. Common trap: `railway up` (or equivalent PaaS CLI) deploys the backend/API, not the frontend static site.

**Document for your project:**
- How production frontend deploys (e.g., deploy script, CI/CD pipeline, git-push-to-deploy)
- How staging/test frontend deploys (often manual — build + upload)
- How backend deploys and why it is a different command/flow
- What happens if you accidentally run the wrong deploy command

<!-- Example:
- **Production frontend:** `scripts/deploy.sh` pushes to a separate git repo that the PaaS auto-deploys
- **Test frontend:** Manual build with test env vars -> `railway up --path-as-root dist`
- **Backend:** `railway up` from repo root (or with appropriate service linked)
-->

---

## Origin Allowlists for Cross-Origin Popups/iframes

If your app uses popup windows or iframes with `postMessage` (auth surfaces, payment gateways, OAuth providers, identity verification), the target window must allowlist the parent origin. The failure mode is **silent and devastating**: the popup completes its flow successfully but the `postMessage` response is blocked by origin checking. The parent window hangs indefinitely with no error visible to the user — only a console warning in the popup's DevTools.

**Document for your project:**
- Which files contain `ALLOWED_ORIGINS` or equivalent allowlists
- The exact steps to add a new origin (add to list + redeploy the service)
- How to debug: check the popup/iframe's console, not the parent's

<!-- Example:
- `reference-implementations/biometric3/surface-handler.js` has `ALLOWED_ORIGINS`
- New frontend URLs must be added + Surface redeployed
- Debug: open DevTools on the popup window, look for postMessage origin errors
-->

---

## MIME Type Issues on CDN/PaaS

Some hosting platforms serve non-standard file extensions with incorrect MIME types. Common problems:
- `.mjs` served as `application/octet-stream` (breaks ES module imports)
- `.wasm` served without `application/wasm` (breaks WebAssembly instantiation)
- `.map` served with wrong type (breaks source maps)

**Workaround pattern — blob URL for web workers:**
```typescript
// Instead of: import workerUrl from './worker.mjs?url'
// Use:
import workerSource from './worker.mjs?raw';
const blob = new Blob([workerSource], { type: 'application/javascript' });
const workerUrl = URL.createObjectURL(blob);
```

**Document for your project:**
- Which files use this workaround and why
- Which hosting platform causes the issue
- Do NOT revert to `?url` imports without verifying MIME types on the deployed platform

---

## Environment Variables Baked at Build Time

Vite, Next.js, and Create React App all **bake environment variables into the bundle at build time**. They are string-replaced during the build and become literal constants in the output JavaScript. This means:

- Building with the wrong `API_URL` and deploying will point the frontend at the wrong backend **permanently** until rebuilt
- There is no way to change these values after build without rebuilding
- Deploy scripts that default to production values will silently use those defaults if you forget to override

**Document for your project:**
- Which env vars are build-time (Vite: `VITE_*`, Next.js: `NEXT_PUBLIC_*`, CRA: `REACT_APP_*`)
- Which env vars are runtime (server-side only, read from `process.env` at request time)
- How to verify the correct values after deploy (e.g., check the built JS bundle, check network requests in DevTools)
- What the deploy script defaults to if no override is provided

<!-- Example:
- `VITE_API_URL` is build-time — always verify before `vite build`
- `JWT_SECRET` is runtime — only needs to be set on the server
- After deploy, open DevTools Network tab and verify API requests go to the expected URL
-->

---

## Production Deploy Confirmation

Deploy scripts targeting production MUST require interactive confirmation.
Without this, agents with max autonomy can and will deploy autonomously —
autonomy grants are for development workflow, not production operations.

**Pattern:**
```bash
# In your production deploy script:
if [ -t 0 ]; then
  read -p "Deploy to PRODUCTION? Type 'yes' to confirm: " confirm
  [ "$confirm" = "yes" ] || { echo "Aborted."; exit 1; }
else
  echo "ERROR: Production deploy requires interactive terminal (TTY)."
  echo "This prevents automated/scripted deploys without human confirmation."
  exit 1
fi
```

The `-t 0` check ensures the script runs in an interactive terminal, not
piped or called from another script. This is a deliberate friction point —
the one place where we want to slow agents down.

**Document for your project:**
- Which deploy commands target production vs. staging
- Which commands have this guard and which don't
- How to bypass for CI/CD pipelines (e.g., `DEPLOY_CONFIRMED=1`)
