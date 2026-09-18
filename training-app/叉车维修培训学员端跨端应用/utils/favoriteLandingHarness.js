/**
 * 收藏落点的**共享执行器**：取 `.uvue` 里的落点函数体 → 注入桩 `uni` → **真的执行**它。
 *
 * ## 为什么要有这个文件（#1159）
 *
 * 这一族缺陷的成因写在 #1089 的复盘里：**同一张落点表被照抄到第二处，缺的分支没人补**，
 * 而「点了没反应」不会让任何守护变红。所以守护这边也不能照抄第二套执行器 ——
 * 出处是本仓 PR #1147 的 `utils/favoritesLandingContract.test.js`（#1089 PR-B）里的本地实现，
 * #1159 把它提到 `utils/` 下成为**唯一实现**，两页共用同一把尺子。
 *
 * ⚠️ **待收口（#1159 如实声明）**：本模块出生时 #1147 尚未合并（它的测试文件里仍带一份本地副本）。
 * #1147 合并后应删掉那份副本、改 `require('./favoriteLandingHarness')`。
 *
 * **它不是 drop-in**（写实：`/code-review` 指出这里原先写的「同名 ⇒ 机械替换」是**过度声明**）
 * —— 逐点改名如下，**语义一一对应、无行为变化**，但调用点要一并改写：
 *
 * | #1147 文件内的本地名 | 本模块导出名 |
 * | --- | --- |
 * | `read(rel)` | `readSource(rel)` |
 * | `fnBodyOf` / `stripUtsTypes` / `stripAs` | **同名** |
 * | `runOnItemClickBody(body, item)` | `runLandingBody(body, item)` |
 * | `const click = (item) => …`（模块级、写死 `PAGE`） | `landingClick(pageRel, marker)` **工厂**（每次调用重读源码，故能服务多页） |
 * | `fav(type, id, courseId)` | `favoriteFixture(type, id, courseId)` |
 * | （无） | `fnFromSource(rel, marker, deps, argNames)` —— 本模块新增，「数据前提」那一组用 |
 *
 * ## 断言方式（照移动端 ADR-0008「守护从断言源码文本改为断言行为」）
 *
 * 断言的**产物**是函数对外做的事（`navigateTo` 的 url / `showToast` 的 title），
 * 不是「源码里出现过某个字符串」—— 后者是坏实现的超集（写对字符串、接错分支照样绿）。
 *
 * ## 边界（写实，不假装通用）
 *
 * - 只跑**落点函数体**这一段；页面其余代码（模板 / 取数 / store）不参与。
 * - 函数体里若用了本仓未出现过的语法，`new Function` 会抛错 ⇒ 判红（安全方向），不静默降级。
 * - 取不到函数体时返回 `''`，调用方的正向用例随即判红，**不空跑变绿**。
 */
const fs = require('fs');
const path = require('path');

/** 被测树的应用根（`utils/` 的上一级） */
const ROOT = path.join(__dirname, '..');

/**
 * 读源码并归一 CRLF。
 * ⚠️ 必须归一：Windows 工作树里的 `.uvue` / `.uts` **未被 `.gitattributes` 钉行尾**
 * （只钉了 `*.vue` / `*.ts` / `*.js` 等），实测是 CRLF；不归一则按 `\n` 锚定的多行逻辑会错位。
 * @param {string} rel 相对应用根的文件路径
 * @returns {string} 内容（LF 行尾）
 */
function readSource(rel) {
  return fs.readFileSync(path.join(ROOT, rel), 'utf8').replace(/\r\n/g, '\n');
}

/**
 * 取 `marker` 之后那个函数的**函数体**（花括号配平，与缩进风格 / 行尾无关）。
 * 配平而非「找下一个行首 `}`」：本仓空格与制表符混用（先例 #1111 契约测试记的三个坑）。
 * @param {string} src 源码
 * @param {string} marker 定位串（如 `function onFavoriteClick(`）
 * @returns {string} 函数体；解析失败返回 `''`
 */
function fnBodyOf(src, marker) {
  const start = src.indexOf(marker);
  if (start < 0) return '';
  const open = src.indexOf('{', start);
  if (open < 0) return '';
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    const ch = src[i];
    if (ch === '{') depth++;
    else if (ch === '}') {
      depth--;
      if (depth === 0) return src.slice(open + 1, i);
    }
  }
  return '';
}

/**
 * 剥掉 UTS 的标量类型标注（`const url : string = …`），让函数体能在 Node 里直接跑。
 * 本仓 UTS 写法允许局部变量带标注 —— 不剥会在 `new Function` 里变成语法错误（判红，
 * 是安全方向），但会让守护变成「改个写法就红」的噪音源，故显式剥掉。
 * @param {string} body 函数体
 * @returns {string} 去标注后的函数体
 */
function stripUtsTypes(body) {
  return body.replace(/:\s*(string|number|boolean|UTSJSONObject|FavoriteItem)\b/g, '');
}

/** 剥掉 TS/UTS 的 `as X` 断言（`} as FavoriteItem` 在 `new Function` 里是语法错误） */
function stripAs(body) {
  return body.replace(/\s+as\s+[A-Za-z_$][\w$]*/g, '');
}

/**
 * 在沙箱里执行一段落点函数体，返回它对外做的事。
 * @param {string} body 函数体（`fnBodyOf` 的产物）
 * @param {object} item 落点条目（收藏条目 / 搜索条目，视被测函数而定）
 * @returns {{navigations: string[], toasts: string[]}}
 */
function runLandingBody(body, item) {
  const calls = { navigations: [], toasts: [] };
  const uni = {
    navigateTo: (o) => calls.navigations.push(o.url),
    showToast: (o) => calls.toasts.push(o.title),
  };
  // 只跑从源码里取出的函数体；不引入被测页面的任何其它代码
  new Function('item', 'uni', stripUtsTypes(body))(item, uni); // eslint-disable-line no-new-func
  return calls;
}

/**
 * 造一个「点第 N 条条目」的函数，**每次调用都重读源码**（页面改了就立刻反映，不缓存）。
 * @param {string} pageRel 页面文件相对路径
 * @param {string} marker 落点函数的定位串
 * @returns {(item: object) => {navigations: string[], toasts: string[]}}
 */
function landingClick(pageRel, marker) {
  return (item) => runLandingBody(fnBodyOf(readSource(pageRel), marker), item);
}

/**
 * 把 `export function <name>(…) { … }` 取出来，用**源码里的真实函数体**
 * 加注入的依赖构造一个可调用函数（不写镜像实现，避免两份实现悄悄分叉）。
 * @param {string} rel 文件相对路径
 * @param {string} marker 定位串
 * @param {Record<string, Function>} deps 该函数引用到的依赖（按参数名注入）
 * @param {string[]} argNames 该函数的形参名
 * @returns {{body: string, call: (...args: any[]) => any}}
 */
function fnFromSource(rel, marker, deps, argNames) {
  const body = stripAs(stripUtsTypes(fnBodyOf(readSource(rel), marker)));
  const names = Object.keys(deps);
  return {
    body,
    call: (...args) => new Function(...names, ...argNames, body)(...names.map((n) => deps[n]), ...args), // eslint-disable-line no-new-func
  };
}

/**
 * 收藏条目夹具。
 * `course_id` 是 **0 哨兵**：后端契约是「键恒在、非 null，其余类型恒 0」（#1133 / #1147）。
 * @param {string} targetType course / chapter / question / featured / topic
 * @param {number} targetId 目标 id
 * @param {number} courseId 所属课程 id（仅 chapter 有意义）
 */
function favoriteFixture(targetType, targetId, courseId = 0) {
  return {
    favorite_id: 1,
    target_type: targetType,
    target_id: targetId,
    course_id: courseId,
    title: 't',
    cover: '',
    created_at: '',
  };
}

module.exports = {
  readSource,
  fnBodyOf,
  stripUtsTypes,
  stripAs,
  runLandingBody,
  landingClick,
  fnFromSource,
  favoriteFixture,
};
