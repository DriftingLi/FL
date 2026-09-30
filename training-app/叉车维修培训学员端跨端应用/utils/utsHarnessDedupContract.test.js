/**
 * utsHarness 自身回归：模块**自有导出**与注入 bindings 同名时，`loadUts` 不得撞重复声明
 *
 * 病根（#1404 实施中抓获）：loadUts 的 `names = [...new Set(need.concat(Object.keys(bindings)))]`
 * 里 Set 只去**列表内**的重复，但装配方把「某模块的自有导出」当键注入（如把 `registerX` 塞进
 * 导出 `registerX` 的 request.uts 的 bindings —— 供消费方取用，属正常接线），生成的
 * `const { registerX } = bindings` 就与模块体内 `function registerX(…)` 顶层重复声明 ⇒
 * **SyntaxError: Identifier 'registerX' has already been declared**，模块根本跑不起来。
 *
 * 裁定：自有导出**永不进注入清单** —— 与 JS 模块语义一致（模块不会被注入自己的导出），
 * 消费方要的引用从 `return { … }` 回读即可，一条都不少。既有全部装配不受影响
 * （此前没有任何模块导入过另一模块的自有导出，判据只在新撞名形态下生效）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');
const { loadUts } = require('./utsHarness');

/** 写一个临时 .uts 夹具（mkdtempSync 唯一目录，与全仓 temp 纪律一致） */
function fixture(name, src) {
  const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'uts-harness-dedup-'));
  const file = path.join(dir, name);
  fs.writeFileSync(file, src, 'utf8');
  return file;
}

describe('loadUts：自有导出与注入 bindings 撞名（#1404 回归）', () => {
  test('导出与注入同名 ⇒ 不抛 SyntaxError，且回读的是模块**自有**函数（注入桩被剔除）', () => {
    const file = fixture(
      'provider.uts',
      "import { sideEffect } from '../somewhere'\n" +
      'export function registerThing(handler : () => void) : void {\n' +
      '    sideEffect(handler)\n' +
      '}\n'
    );
    const calls = [];
    // 装配方把 provider 的自有导出也放进了 bindings（#1404 里 concurrent401RefreshBehavior 的真实形态）
    const injectedStub = () => { throw new Error('注入桩不得被当成自有导出回读'); };
    const mod = loadUts(file, {
      sideEffect: (h) => { calls.push(h); },
      registerThing: injectedStub,
    });
    // 术前此处直接抛 SyntaxError（has already been declared）
    expect(typeof mod.registerThing).toBe('function');
    expect(mod.registerThing).not.toBe(injectedStub);
    // 自有函数真的可跑：调用落到本模块注入的 sideEffect
    const handler = () => {};
    mod.registerThing(handler);
    expect(calls).toEqual([handler]);
  });

  test('export const 与注入撞名同样不抛（const 在体中部，声明序无关）', () => {
    const file = fixture(
      'prefixed.uts',
      "import { other } from '../somewhere'\n" +
      "export const MESSAGE_PREFIX = 'x:'\n" +
      'export function useIt() : string { return other(MESSAGE_PREFIX) }\n'
    );
    const mod = loadUts(file, { other: (p) => p + 'used', MESSAGE_PREFIX: '注入值' });
    expect(mod.MESSAGE_PREFIX).toBe('x:'); // 自有导出赢（JS 模块里注入方拿不到别人的自有导出，本就应剔除）
    expect(mod.useIt()).toBe('x:used');
  });
});
