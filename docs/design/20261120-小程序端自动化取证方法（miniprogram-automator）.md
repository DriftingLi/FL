# 小程序端自动化取证方法（miniprogram-automator 穿透自定义组件）

> 立：2026-11-20。来源 = #2709（论坛 WYSIWYG 工具栏点了没反应）的诊断与验收取证过程，实施见 PR #2717 / commit `ecec0328`。
> 定位：**诊断与取证手法**，不是验收门。② 微信开发者工具门已于 #2320 整体退役，本方法**不判红、不入 PR 正文**；截图按 A1 口径（ADR-0049）也不作验收证据。

## 一句话

`page.$()` / `selectComponent` 都只覆盖**页面作用域**，自定义组件内部的节点与事件对 automator 不可见；改走 `getCurrentPages()` 末页的 `$vm.$children` 拿到 uni-app x 组件实例、再用 `$vm.$scope.data` 里的短名反查 WeChat Component 的事件处理器，就能既读组件内状态、又派发组件内事件。

## 适用与不适用

- **适用**：uni-app x 编译到 `mp-weixin` 的页面，要验证「组件内部状态变化」（计数器、提示行、列表项）或「组件事件链路」（点击工具条 → emit → 父组件处理）。
- **不适用**：纯页面级交互（`page.$('.selector')` 就够）、需要真实触摸手势的场景（canvas 涂鸦、长按拖拽）、以及任何要用作验收证据的场合 —— 后者走真机冒烟（ADR-0043 决策 1）。
- **前提**：微信开发者工具**非最小化**。最小化时 `connect` 会成功，但 `systemInfo` / `currentPage` 全部 `ERR timeout waiting for automator response`，现象像「连上了却什么都不动」。

## 环境准备

```powershell
# 1) 启动自动化会话（--auto-port 每个项目一个端口）
D:\软件\微信web开发者工具\cli.bat auto --project 'D:\path\to\unpackage\dist\build\mp-weixin' --auto-port 9420
# 期望输出：√ auto

# 2) automator 装到仓外临时目录，别进 node_modules、别进 package.json
cd $env:TEMP\mpauto2709; npm i miniprogram-automator
```

```js
const automator = require('miniprogram-automator');
const mp = await automator.connect({ wsEndpoint: 'ws://127.0.0.1:9420' });
await mp.screenshot({ path: 'f1-empty.png' }); // 设备 iPhone 12/13 Pro 390x844，产物约 461x997
```

要点：

- **端口分工做对照实验**：一个端口开修复版产物、另一个端口开旧产物（同一 `--project` 不同 dist），同套脚本跑两遍即得「修复前 / 修复后」读数。
- 调试基础库版本对 `$scope.data` 的短名生成有影响，换项目/换构建要先 dump 再看（见下）。
- `mp.screenshot()` 可用，但**只作会话附证**；A1 之后 PR 正文只留门读数行。

## 登录态：把已登录项目的 storage 搬过去

build 产物的 storage 是空的，`reLaunch('/pages/forum/forum-create')` 会被重定向到 `pages/login/login`。做法：先连一个**已登录**的旧项目，读关键 key，再写进目标项目。

```js
// 在已登录项目上
const keys = ['auth_token','auth_user','auth_refresh_token','auth_login_provider','selected_cert'];
const dump = await mp.evaluate((keys) => { const o = {}; keys.forEach(k => { try { o[k] = wx.getStorageSync(k) } catch (e) {} }); return JSON.stringify(o) }, keys);

// 在目标项目上
await mp2.evaluate(function (payload) { var o = JSON.parse(payload); Object.keys(o).forEach(function (k) { wx.setStorageSync(k, o[k]) }) }, dump);
await mp2.reLaunch('/pages/forum/forum-create'); // 落在已登录页面
```

`auth_token` 这个键名是硬编码常量 `STORAGE_KEY_TOKEN`（`constants/app.uts:6`），不是猜的。取证完记得 `reLaunch('/pages/dashboard/dashboard')` 把旧项目复原。

## 可达性三层（决定性）

| 层 | 写法 | 能否进自定义组件 |
| --- | --- | --- |
| 页面选择器 | `page.$('#forum-wysiwyg-editor')` / `page.$$('.tool-btn-more')` | **否**，`count = 0` |
| 组件选择器 | `page.selectComponent('forum-wysiwyg-input')` | **否**，返回 `false` |
| 运行时实例 | `getCurrentPages()` 末页 `.\$vm` | **是** |

实测：`page.$$('view')` 只捞到 8 个页面级节点（container / scroll-body / post-card / category-row / category-chip×2 / post-submit / submit-btn），`forum-wysiwyg-*` 子树整体不可见。这与 #2709 的根因**同源**：选择器只在页面作用域里选，`wx.createSelectorQuery()` 不带 `component` 就选不到自定义组件里的 `editor`。

```js
const info = await mp.evaluate(function () {
  var pages = getCurrentPages();
  var pg = pages[pages.length - 1];
  var vm = pg.$vm;              // uni-app x 运行时组件实例
  return JSON.stringify({
    children: vm.$children.map(function (c) { return c.$options && c.$options.name || typeof c }),
    idx: 1,
  });
});
```

`page.$vm.$children` 是运行时组件树：本仓论坛页上是 `[0] = app-nav-bar`、`[1] = components/forum-wysiwyg-input`；input 自己再往下 `[0] = forum-wysiwyg-toolbar`、`[1] = forum-wysiwyg-image-area`。组件实例上能直接调到组件暴露的方法（本次 `insertMention` / `clear`）。

## 派发组件内事件

`$vm.$scope` 是 WeChat Component 实例。编译产物把事件处理器挂在 `$scope.data` 的**短名**上（`f`、`g`、`h`…，值是 `e0_54`、`e1_29` 这类方法名），而 `bind*` 属性把短名指向它们。所以：**短名 → 方法名 → 调用**，就等价于真事件。

```js
// ① 先 dump，别猜键（换构建/换基础库短名会变）
const keys = await mp.evaluate(function () {
  var p = getCurrentPages(); var vm = p[p.length - 1].$vm.$children[1];
  var d = vm.$scope.data, out = {};
  Object.keys(d).forEach(function (k) { if (typeof d[k] === 'function') out[k] = d[k] });
  return JSON.stringify(out);
});
// 本次实测：f=e0_54 g=e1_29 h=e2_72 i=e3_85 j=e4_0b k=e5_d3 l=e6_09 m=e7_fb
// 顺序对应 bindready/bindfocus/bindblur/bindinput/bindstatuschange/bindaction/bindpaste/bindpress

// ② 派发
await mp.evaluate(function (action) {
  var p = getCurrentPages(); var sc = p[p.length - 1].$vm.$children[1].$scope;
  sc[sc.data.k](action);            // onActionTap
}, 'divider');
```

参数形态（踩过才知道）：

| 处理器 | 传什么 | 传错的后果 |
| --- | --- | --- |
| `onActionTap` | **裸字符串**，如 `'divider'` / `'list'` / `'header'` | 传 `{ detail: 'divider' }` 静默无效（计数不动） |
| `onPasteTap` | `{ type: 'tap' }` | 传 `undefined` 抛 `Cannot read properties of undefined (reading 'type')` |
| `onEditorFocus` / `onEditorBlur` | `{ type: 'focus' }` / `{ type: 'blur' }` | 同上 |
| 工具条 `press` | `{ type: 'touchstart' }` | 同上 |

## 读组件内状态

同一路径读 `$scope.data` 的渲染用字段。本次实测的键（`forum-wysiwyg-input`）：

| 字段 | 含义 |
| --- | --- |
| `p` | 字数计数文本，形如 `"15/10000"` |
| `x` | 提示行正文 |
| `w` | 提示行是否显示 |
| `a` | 编辑器是否挂载 |

**别跨构建照抄这些短名**：旧 build 的计数在 `data.o`、默认提示串在 `data.w`，照 `f/g/h…` 猜键会调到别的函数，表现成「点了没反应」—— 与真 bug 同形，极易误判。先 dump，再调。

## 剪贴板

```js
await mp.mockWxMethod('getClipboardData', { data: '#2709 粘贴验证文本' });
// 用 wx.getClipboardData 读回验证 mock 生效，再去点粘贴
```

mock 只在同一 session 内有效；`reLaunch` / 重连后失效，重跑前重新 mock。

## #2709 的实测读数（方法有效性的样本）

| 场景 | 修复版 build | 旧代码 dev 产物 |
| --- | --- | --- |
| 初始 | `0/10000` | `0/10000` |
| 点「粘贴」 | `15/10000`，提示行「已按纯文本粘贴，原格式未保留」 | `0/10000`，纹丝不动 |
| 点「分割线」 | `19/10000`（插 `---\n`） | 无变化 |
| 点「标题」 | `23/10000` | 无变化 |

误报抑制的正反对照（同一 session 内）：

| 时序 | 提示行 |
| --- | --- |
| `focus → 950ms → 工具条按下 → blur → 700ms` | 无「粘贴没生效」 |
| `focus → 950ms → blur`（不按工具条） | 出现「粘贴没生效？点「粘贴」按钮插入剪贴板里的纯文本」 |

第二行是**必要的对照组**：只验「抑制生效」不够，还要验「检测没被整体关掉」，否则修法退化成删功能也照样绿。

## 陷阱清单

1. **窗口最小化** ⇒ connect 成功但所有调用超时。先 `Get-Process` / 手动还原窗口再连。
2. **短名随构建变** ⇒ 永远先 dump `Object.keys($scope.data)`，再对 `bind*` 名找映射。
3. **不传参派发** ⇒ wrapper 读 `e.type` 抛错，界面表现为「点了没反应」，与真 bug 同形。第一次跑先拿一个必然变化的东西（计数器、`wx.getClipboardData` 读回）当阳性对照。
4. **`$$('view')` 只到页面级** ⇒ 别用「数节点」证明渲染正常，要读组件 `$scope.data`。
5. **截图不是证据** ⇒ A1 之后 PR 正文只留门读数行；截图放会话或仓外临时目录，别进仓库。
6. **仓外装依赖** ⇒ automator 不写进 `package.json`，否则会被当成生产依赖并过依赖门。

## 与验收门的关系

- ② 微信开发者工具门已于 #2320 整体退役，日常 PR 不跑；需要时只能手动诊断、不判红。
- 真机逐页取证降级为**发版前一次**（ADR-0043 决策 1）。本方法是 ①a 降级后的**诊断替代**，不是它的替代品 —— 它能证明「逻辑在模拟器里成立」，证不了「真机上成立」。
- 本笔记不立任何机检：automator 脚本随票写、随票弃，进仓库只进这份方法说明。
