const babel = require('@babel/core');
const syntaxTs = require('@babel/plugin-syntax-typescript').default;
const src = `
let inflight : Promise<boolean> | null = null
let holdMs : number = 100
export function resetGate() : void { inflight = null }
export function gate(runner : () => Promise<boolean>) : Promise<boolean> {
    const existing = inflight
    if (existing != null) return existing
    const p = runner()
    inflight = p
    p.then((ok : boolean) => { setTimeout(() => { if (inflight == p) inflight = null }, holdMs) })
    return p
}
`;
const strip = () => ({
  visitor: {
    TSTypeAnnotation(path) { path.remove() },
    TSTypeParameterInstantiation(path) { path.remove() },
    TSTypeParameterDeclaration(path) { path.remove() },
    TSAsExpression(path) { path.replaceWith(path.node.expression) },
    TSNonNullExpression(path) { path.replaceWith(path.node.expression) },
    TSTypeAliasDeclaration(path) { path.remove() },
    TSInterfaceDeclaration(path) { path.remove() },
    TSDeclareFunction(path) { path.remove() },
    ExportNamedDeclaration(path) {
      if (path.node.declaration) path.replaceWith(path.node.declaration);
      else path.remove();
    },
  },
});
const out = babel.transformSync(src, { filename: 'gate.ts', plugins: [syntaxTs, strip], babelrc: false, configFile: false });
console.log('--- OUTPUT ---\n' + out.code);
const names = [...src.matchAll(/export\s+function\s+([A-Za-z0-9_]+)/g)].map(m => m[1]);
const body = out.code + '\nreturn {\n' + names.map(n => n + ': ' + n).join(',\n') + '\n};';
const mod = new Function('setTimeout', 'clearTimeout', body)(setTimeout, clearTimeout);
console.log('exports:', Object.keys(mod));
(async () => {
  let calls = 0;
  const runner = () => { calls++; return new Promise(r => setTimeout(() => r(true), 40)); };
  const res = await Promise.all([mod.gate(runner), mod.gate(runner), mod.gate(runner), mod.gate(runner)]);
  console.log('calls=', calls, 'res=', JSON.stringify(res));
  await new Promise(r => setTimeout(r, 150));
  const res2 = await mod.gate(() => { calls++; return Promise.resolve(false); });
  console.log('after hold, res2=', res2, 'calls=', calls);
})();
