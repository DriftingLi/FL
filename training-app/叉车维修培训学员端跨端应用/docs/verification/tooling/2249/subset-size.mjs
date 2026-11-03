import { execFileSync } from 'node:child_process';
const files = process.argv.slice(2);
for (const f of files) {
  let out;
  try {
    out = execFileSync(process.execPath, ['scripts/behavior-suite-deps.mjs', '--changed-json', '--changed', f], { encoding: 'utf8' });
  } catch (e) {
    console.log(`${f} => EXEC_ERR ${e.message.slice(0, 80)}`);
    continue;
  }
  const j = JSON.parse(out);
  console.log(`${f} => ${j.subset.length}/${j.total} (${(100 * j.subset.length / j.total).toFixed(1)}%) uncovered=${j.uncovered.length}`);
}
