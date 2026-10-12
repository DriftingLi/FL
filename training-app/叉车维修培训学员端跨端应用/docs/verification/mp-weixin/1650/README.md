# 原型取证 — #1650 cert-selector mp-weixin 浮层三形态对照（UI 分支）

**本目录 = 本票「验收证据」段的可核验产物落点**（`.ci-verify/` 被 `training-app/…/.gitignore:10` 忽略，故复制入库）。诊断收口与拍板结论见 [issue #1650](https://github.com/DriftingLi/FL/issues/1650) 正文「## 状态（2026-10-12 triage）」段；由此拆出的修复票 = [#1671](https://github.com/DriftingLi/FL/issues/1671)（真机复证为其第一项验收）。

## 工具与环境

| 项 | 值 |
| --- | --- |
| 微信开发者工具 | Stable **2.02.2608070** |
| 基础库 / SDK | **3.17.3** |
| 工程 | `wt-1650/training-app/叉车维修培训学员端跨端应用/unpackage/dist/build/mp-weixin`（appid `wx38c3e31b16a7ced0`，feat/1650，基线 `8a95e05b`） |
| HX 编译 | 10:41–10:55，60 页编译成功，产物 mtime 10:55:10 |
| 取证日期 | 2026-10-12 |
| 自动化 | `scripts/cert-prototype-evidence3.mjs`（connect → `pageStack()` 探活 → 固定节奏连拍，截图自带重试 + sha256） |
| 判读 | `scripts/png-lum-profile.mjs`（自写：PNG 解码 + 20 带行亮度剖面 + 实心白面板行检测 + 遮罩强度代理）+ `scripts/png-crop.mjs`（行区间裁剪目检） |

**脚本来源（重要）**：判读/取证脚本由本票 spike 期写在 `wt-1650/.../scripts/` 下（`cert-prototype-evidence.mjs` / `evidence2` / `evidence3` / `evidence4`、`png-lum-profile.mjs`、`png-crop.mjs`）。它们是 **throwaway 夹具**，收口时**未入版本库**（原型提交 `7a4e9066` / `d7712354` 已按 throwaway 收口回退）—— 本目录只留存**产物**（截图 + log + 本 README），故复现本目录读数需自行重写判读逻辑；本 README 的判据表已把全部读数写全，不依赖脚本可复核。

**取证面诚实边界**：12 帧全部来自**开发者工具 automator 截图**，真机面本票未独立复现（这正是 Q2 拍板「真机复证留到 #1671」的原因；先例 #1473 的真机取证抓到过开发者工具看不到的 fixed z 序 bug）。

## 夹具确已进编译产物（机器证据，非目测）

- `unpackage/dist/build/mp-weixin/pages/dashboard/dashboard.js`：`cert-prototype` ×2（`console.log("[cert-prototype] variant =",q.value,"show =",l.value)`）、`j=["full","drawer","anchor"]`、标签表 `full:"A 全屏下拉（现款）"`
- `unpackage/dist/build/mp-weixin/components/cert-selector/cert-selector.js`：`cert-dropdown--drawer` / `cert-dropdown--anchor` / `cert-dropdown-mask--drawer` 均在

## 12 帧清单（sha256 前缀 / 字节 / mtime，连拍参数 `--shots 12 --cycle-ms 3400`）

| 帧 | sha256 前缀 | 字节 | mtime | 归簇 |
| --- | --- | --- | --- | --- |
| variant-1 | `7B244651745A` | 134500 | 16:47:39 | A 簇 |
| variant-2 | `7B244651745A` | 134500 | 16:47:40 | A 簇 |
| variant-3 | `6413EA08DAE0` | 87933 | 16:47:42 | B 簇 |
| variant-4 | `6413EA08DAE0` | 87933 | 16:47:43 | B 簇 |
| variant-5 | `7F1BCEC22DDA` | 180058 | 16:47:44 | C 簇 |
| variant-6 | `2C4739F6B817` | 180475 | 16:47:46 | C 簇 |
| variant-7 | `2C4739F6B817` | 180475 | 16:47:47 | C 簇 |
| variant-8 | `AC8ED7F20B43` | 134499 | 16:47:48 | A 簇（差 1 字节，渲染细微差） |
| variant-9 | `7B244651745A` | 134500 | 16:47:50 | A 簇 |
| variant-10 | `6413EA08DAE0` | 87933 | 16:47:51 | B 簇 |
| variant-11 | `6413EA08DAE0` | 87933 | 16:47:52 | B 簇 |
| variant-12 | `90CD20B2F11C` | 180488 | 16:47:54 | D（浮标文案「B 底部抽屉」） |

v8 与 v1/v2/v9 同簇但 sha 差 1 字节（渲染细微差），已核磁盘 mtime 确认四帧同属 16:47 这一波（v3 慢采），非早期波次残留。

## 判据与读数

| 判据 | 读数 |
| --- | --- |
| 全屏遮罩 `position:fixed;inset:0;z-index:300;background-color:rgba(0,0,0,.4)` | **12/12 帧 `mask=NONE`，darkFrac 最高仅 0.03 —— 一帧都没绘制** |
| 面板高度 | 仅屏高 **7–8%**（约 66–80 行 / 997px），三形态均远低于 60% / 满屏规格 |
| 原生 tab bar | 三形态下**全程完整可见** ⇒ 面板既没铺满也没盖住底部 |
| 面板内容 | 「选择学习阶段」+ 特种作业配套上岗证分组 + 叉车司机N1证 / 低压电工证 / 焊工证三枚 chip **渲染得出** |

裁剪目检（`png-crop.mjs`）：`crop-v12-rows780-997.png` / `crop-v3-rows780-997.png` 底部裁均见 tab bar（首页/课程/AI学/交流/我的）完整；`crop-v12-rows0-330.png` 顶部裁见 AppNavBar「叉车司机N1证 ▼」+ 通知栏 + 横幅 ⇒ 页面本体未被遮挡。

## ③ 契约门记录

`npx jest --config jest.config.unit.js --listTests`（worktree `wt-1650`，目录名合规）列出 **184 个套件**，exit 0。改动集 = `.uvue` ×2 + spike 脚本（`.mjs`）+ 本目录产物，**不含任何 `.uts`** ⇒ ③ 无契约用例需跑；为防「改了 cert 交互却无契约护栏」，仍单跑与本票最近的一套作冒烟：`utils/credentialSwitchBehavior.test.js`（`Test Suites: 1 passed` / `Tests: 2 passed`，exit 0）。

## 诚实边界

- 本目录只做**形态验证**（原型之问的答案：三态皆渲染但规格皆不兑现 ⇒ 责任在 fixed 挂载机制，不在形态选择）；**不含修复**，修复在 #1671。
- 遮罩「一帧未绘制」是**开发者工具渲染面**读数，真机可能不同 —— 已作为 #1671 第一项验收（真机复证）。
- 自动化过程未提交表单、未改业务代码、未点发布；`feat/1650` 上 `7a4e9066` / `d7712354` 为 throwaway 夹具提交，收口后回退。
