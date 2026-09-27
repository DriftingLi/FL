import { readFileSync } from 'node:fs';
import { execFileSync } from 'node:child_process';

const REPO = process.argv[2];
const BASE = process.argv[3];
const HEAD = process.argv[4];
const BODY_FILE = process.argv[5];

import { validatePrEvidence as validate } from './validator.mjs';

const names = execFileSync('git', ['-c', 'core.quotepath=false', '-C', REPO, 'diff', '--name-only', '-z', `${BASE}...${HEAD}`], { encoding: 'utf8' })
  .split('\0').filter(Boolean);

const files = names.map((filename) => {
  const patch = execFileSync('git', ['-c', 'core.quotepath=false', '-C', REPO, 'diff', '--unified=3', `${BASE}...${HEAD}`, '--', filename],
    { encoding: 'utf8', maxBuffer: 64 * 1024 * 1024 });
  return { filename, patch };
});

const body = readFileSync(BODY_FILE, 'utf8');
const res = await validate({ files, body, author: 'zhengcookie', gateComments: [], headSha: HEAD, fetchCompare: null });

console.log('--- files classified ---');
for (const f of files) console.log(`${f.filename}`);
console.log('runtime        :', res.runtime.length, 'file(s)');
console.log('lowRiskRuntime :', res.lowRiskRuntime);
console.log('needs4b        :', res.needs4b);
console.log('--- notes ---');
res.notes.forEach((n) => console.log(' * ' + n));
console.log('--- errors ---');
if (res.errors.length === 0) console.log(' (none)  ok =', res.ok);
res.errors.forEach((e) => console.log(' ! ' + e));
