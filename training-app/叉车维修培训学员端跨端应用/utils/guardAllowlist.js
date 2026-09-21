/**
 * `GUARD_ALLOWLIST` 的**唯一声明点**（ADR-0023 决策 ⑧）
 *
 * 为什么单独一个文件：在这之前，`GUARD_ALLOWLIST` 定义在 `utils/utsAndroidCompile.test.js` 里，
 * 而 12 份契约测试要「本模块域不许出现豁免」时，只能去**解析那份源码文本**
 * （先定位常量声明行的下标、再 `slice` 出来，然后对切片跑正则）。
 * 那种做法的代价写在那些测试自己的注释里：**终止符缺失会静默扩扫全文** —— 判据从「这个常量」
 * 悄悄变成「这个文件的剩余全部内容」，红绿都可能失真。同一个值被 12 处各自抠一遍文本，
 * 正是 ADR-0023 要治的「同一条事实各写一遍」。
 *
 * 现在的形态：数据一处（本文件）、查询一处（`utils/contractHarness.js#allowlistPaths`）。
 * 消费方一律 `require` 取用，**全仓不得再解析该常量的源码文本**（守护：
 * `utils/modulesDeclarationContract.test.js` 的 D3/D4）。
 *
 * 表语义（沿用原注释口径）：键 = 守护规则标识，值 = 豁免文件的**仓库根相对路径**集合。
 * 规则上线时已存在的违例按「规则 → 文件」豁免，由后续工单在各自范围清零；
 * 仅存量不为零的规则入表（F/G/I 在 master 树上实测零存量，全量执法无豁免）。
 */

const GUARD_ALLOWLIST = {
  H: new Set([
    'api/checkin.uts',
    'pages/notifications/notifications.uvue',
  ]),
};

/**
 * 全部豁免文件（扁平、去重、排序）—— 「模块面不回潮」类断言的读取面。
 *
 * 排序保证同一集合在任何平台、任何 `readdir` 顺序下给出同一结果（对账不得依赖平台）。
 * @returns {string[]} 仓库根相对路径（posix 分隔符）
 */
function allowlistPaths() {
  const out = new Set();
  for (const files of Object.values(GUARD_ALLOWLIST)) {
    for (const f of files) out.add(f);
  }
  return [...out].sort();
}

module.exports = { GUARD_ALLOWLIST, allowlistPaths };
