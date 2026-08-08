#!/usr/bin/env node
// Admin first-load bundle budget gate.
//
// Checks the gzip size of the admin initial bundle (admin/dist/umi.js and
// umi.css) against a fixed budget so first-load size regressions fail CI
// instead of landing silently. Baseline (R187 performance audit):
// umi.js 320.6 kB gzip. Budget leaves headroom for normal feature growth;
// raising it requires an explicit, reviewed change to this file.
//
// Usage: pnpm build:admin && node scripts/check-bundle-budget.mjs

import { gzipSync } from 'node:zlib';
import { readFileSync, readdirSync, existsSync } from 'node:fs';
import path from 'node:path';
import { fileURLToPath } from 'node:url';

const root = path.resolve(path.dirname(fileURLToPath(import.meta.url)), '..');
const distDir = path.join(root, 'admin', 'dist');

// Budgets in bytes (gzip), matched against hashed entry files
// (e.g. umi.dd868e44.js). umi.js baseline: 320.6 kB gzip (R187).
const BUDGETS = [
  { name: 'umi.js', pattern: /^umi(\.[0-9a-f]{8,})?\.js$/, maxGzipBytes: 350 * 1024 },
  { name: 'umi.css', pattern: /^umi(\.[0-9a-f]{8,})?\.css$/, maxGzipBytes: 20 * 1024 },
];

const kb = (n) => (n / 1024).toFixed(1);

if (!existsSync(distDir)) {
  console.error(`[bundle-budget] FAIL: ${distDir} not found (run pnpm build:admin first)`);
  process.exit(1);
}
const entries = readdirSync(distDir);

let failed = false;
for (const { name, pattern, maxGzipBytes } of BUDGETS) {
  const file = entries.find((f) => pattern.test(f));
  if (!file) {
    console.error(`[bundle-budget] FAIL ${name}: no file matching ${pattern} in ${distDir} (run pnpm build:admin first)`);
    failed = true;
    continue;
  }
  const raw = readFileSync(path.join(distDir, file));
  const gz = gzipSync(raw, { level: 9 }).length;
  const ok = gz <= maxGzipBytes;
  const status = ok ? 'OK  ' : 'FAIL';
  console.log(
    `[bundle-budget] ${status} ${file}: gzip ${kb(gz)} kB / budget ${kb(maxGzipBytes)} kB (raw ${kb(raw.length)} kB)`,
  );
  if (!ok) failed = true;
}

if (failed) {
  console.error(
    '[bundle-budget] initial bundle exceeds budget. Prefer code-splitting / lazy routes over raising the budget; budget changes must be reviewed in scripts/check-bundle-budget.mjs.',
  );
  process.exit(1);
}
