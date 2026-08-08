// Copies the user-facing integration docs into admin/public/docs so the admin
// site self-hosts them (served at /docs/*.md) and in-app doc links stay valid
// without hardcoding any repository URL. Runs via admin prebuild/predev; the
// copies are gitignored — docs/ stays the single source of truth.
import { copyFileSync, mkdirSync } from 'node:fs';
import { dirname, join } from 'node:path';
import { fileURLToPath } from 'node:url';

const root = join(dirname(fileURLToPath(import.meta.url)), '..');
const outDir = join(root, 'admin', 'public', 'docs');
const files = ['mcp.md', 'open-api.md'];

mkdirSync(outDir, { recursive: true });
for (const f of files) {
  copyFileSync(join(root, 'docs', f), join(outDir, f));
}
console.log(`[sync-admin-docs] copied ${files.join(', ')} -> admin/public/docs/`);
