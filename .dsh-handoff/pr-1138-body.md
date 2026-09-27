## 背景

`#1138` triage 的结论是「这条工具侧缺陷应该写进文档」，但核实后发现**文档早就有了** —— 移动端 `docs/adr/0008-移动端验收门与证据.md` 的「2026-09-18 修订」第 6 条由 **#1136（已合并）** 写入主干，逐字记录的就是这件事。

所以本票不需要等任何未合并的 PR。真正剩下的缺口是：**那一条里有一句归因是错的**。

## 改什么

第 6 条末句原文：

> ② 门不受影响（外部调 `cli.bat` 再 `connect()`），但凡走 `uni-automator` 的 mp-weixin 路径（**内部用 `Automator.launch`**）在本机 Node 上都会撞这条。

「内部用 `Automator.launch`」经实测**推翻**：

- `@dcloudio/uni-automator@2.0.0` 的 **`dependencies` 里没有** `miniprogram-automator`（实读其 `package.json`）；
- 全包对 `miniprogram-automator` / `Automator.launch` 的引用 **0 处**；
- 它有自己的 launcher，从开发者工具的 `.automator/*.json` 读 `wsEndpoint`。

**同类缺陷确实存在，但在另一条代码路径上**，本次一并核实并写进 ADR：它的编译步骤在 Windows 下取 `npm.cmd` 去 spawn，而该调用**同样没有 `shell`**（全包 `shell` 出现 0 次）⇒ 触发条件是 `package.json` 里命中 `<platform>:<mode>` 脚本条目 **才**会抛 `EINVAL`，不是「只要用 uni-automator 就必撞」。另照实记 `@dcloudio/uni-automator@2.0.0` 在 npm 上已带 `deprecated: error version` 标记。

**不改** ①②③④ 的门语义、不改 `pr-evidence` 校验逻辑；改动量 = 1 行替换。

## 验收证据

免（未命中运行时面）—— 改动集只有 `training-app/叉车维修培训学员端跨端应用/docs/adr/0008-移动端验收门与证据.md` 一个 `.md` 文件（`git diff --stat` = 1 file changed, 1 insertion(+), 1 deletion(-)），不含 `*.uvue` / `*.uts` / 三份 json。

本改动的判据是「**替进去的那句话在真实链路上成立**」，故按实测取证而非截图：

| 断言 | 取证方式 | 结果 |
| --- | --- | --- |
| uni-automator 不引用 `miniprogram-automator` | 读 `@dcloudio/uni-automator@2.0.0` 的 `package.json` 依赖表 | 依赖表为 `address / debug / default-gateway / kill-port / licia / postcss-selector-parser / qrcode-reader / qrcode-terminal / ws` —— **无** `miniprogram-automator` |
| 全包引用数 | 递归 grep `miniprogram-automator\|Automator\.launch` | **0 处** |
| Windows 下取 `npm.cmd` | 递归 grep `npm\.cmd` | 命中其 `index.js`：`process.env.UNI_NPM_PATH \|\| (/^win/.test(process.platform) ? "npm.cmd" : "npm")` |
| 该 spawn 缺 `shell` | 递归 grep `shell` | **0 处** |
| `npm.cmd` / `.bat` 在本机 Node 上确实抛 `EINVAL` | 自造无害 `.bat` 与 `npm.cmd` 各 spawn 一次 | `spawn('*.bat', …)` → 同步抛 `EINVAL`（3ms）；`spawn('npm.cmd', …)` → 同样 `EINVAL`；`spawn('node', …)` 正常 |
| npm 的 deprecated 标记 | `npm install` 输出 | `npm warn deprecated @dcloudio/uni-automator@2.0.0: error version` |

环境：Windows / Node v24.14.0。

## 关联

- 承载 issue：#1138
- 写入第 6 条的原始 PR：#1136（已合并，含本条的 `miniprogram-automator` 部分，那部分实测成立、未改动）
