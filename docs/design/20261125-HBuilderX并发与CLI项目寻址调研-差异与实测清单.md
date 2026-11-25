# HBuilderX 并发调研：与仓内写法的差异、待实测清单与抓取清单

> 本文件是 `20261125-HBuilderX并发与CLI项目寻址调研.md` 的第二部分（原 §3–§5）；官方原文证据与结论表在上半部分。

## 3. 与仓内写法的差异：哪几句要改、改成什么

| # | 位置 | 现写法 | 问题 | 建议原句 |
| --- | --- | --- | --- | --- |
| 1 | 移动端 `AGENTS.md`「并发会话纪律」首句 | 「HBuilderX 的 CLI 步骤**由仓内锁串行化**——不是平台上限（主程序可多开）。」 | 「主程序可多开」**官方无据**（官方只写多窗体/多项目，从未写多实例） | 「HBuilderX 的 CLI 步骤**由仓内锁串行化**——不是平台上限（官方记载的是同一程序内多窗体、主窗体可显示多个项目；**是否可并存多个实例，官方未记载**）。」 |
| 2 | `training-app/…/docs/agents/concurrency.md`「边界（2026-11-25 更正）」 | 「主程序本身可以同时打开多个项目（多棵树各开一个即可）。」 | 前半有官方据；「**多棵树各开一个即可**」是推论（CLI 把 N 个 `--path` 落到 N 个项目，官方未记载） | 「官方记载同一程序内可开多个窗体、**主窗体可以显示多个项目列表**；CLI 侧多路径并发 `open` 的行为**官方未记载**（待实测 Q-A）。」 |
| 3 | 同上 | 「加锁防的是「多个 CLI 并发驱动**同一个主程序**」的互相踩踏与假失败」 | 「同一个主程序」官方未明说（只能说 CLI 要求主程序在跑） | 「加锁防的是「多个 CLI 并发驱动**同一个 HBuilderX 程序**」的互相踩踏与假失败（CLI 要求主程序已在运行，这一点官方有明确原文）。」 |
| 4 | `scripts/lib/hx-busy.ps1` 头注释 | 「单实例串行资源 = 本仓库的编排约定，不是 HBuilderX 的平台上限：主程序本身可以同时打开多个项目，**多棵树各开一个即可**」 | 同 #2：末半句是推论 | 拆成两句：「官方未记载实例数量与并发行为；官方只记载**多窗体/多项目**。故本仓把「同一时刻只跑一个 CLI 步骤」定为编排约定。」 |
| 5 | 全仓「HBuilderX 是**单实例**串行资源」这一措辞（散见 `hx-run.ps1`、`dev-finish.ps1`、`compile-check.ps1`、ADR-0008 等多处） | 「单实例串行资源」 | 与 #1/#2 的「不是单实例（可多开）」**自相矛盾**；且「单实例」本身官方未记载 | 统一改为「**HBuilderX 串行使用口径**（本仓编排约定）」——去掉「单实例」这一未经验证的事实断言，保留「串行」这一仓内选择。**这是本次最该改的一句**，因为它同时消掉了自相矛盾的根。 |
| 6 | `docs/agents/multi-agent-git.md`「HBuilderX 5.23 没有 `project` 这个子命令」 | 「HBuilderX 5.23 没有 `project` 这个子命令（实测 `cli project close --path …` → `命令'project'不存在或缺少参数`）」 | 与官方文档**直接冲突**：官方 `/cli/project` 明确有 `list`/`open`/`close`，且标明 3.1.5+ 起可用 | 「本机 HBuilderX 5.23 执行 `cli project close --path …` 返回「命令'project'不存在或缺少参数」，而**官方文档（3.1.5+）明确记载 `cli project` 子命令**；版本或发行版差异，按待实测项 Q-D 复核。」 |
| 7 | `docs/agents/multi-agent-git.md`「任何一次新的 `cli launch` 都会让旧会话退出」 | 陈述为事实 | 官方未记载（本机实测成立，但不可当官方语义） | 「本机实测（2026-09）：任何一次新的 `cli launch` 会让旧会话退出、句柄释放。**官方文档未记载此行为**，升级后须复测。」 |
| 8 | `docs/agents/multi-agent-git.md`「CLI / 脚本（待实测）」 | 「若 CLI 真按路径解析，同名不同路径不冲突…此点**未验证**」 | 与官方一致（官方确实只写 `--path`、不写同名语义），**保留不改** | 可加半句锚点：「官方 `cli project open` 只接受 `--path`，但**未记载同名项目的解析优先级**，因此仍需 Q-A 实测。」 |

> 补充口径（建议加一句在 `concurrency.md` 顶部）：**「官方文档既不支持也不否定并发；本仓按『未验证』处理」**。这比「不是平台上限（主程序可多开）」更抗复核——前者是事实，后者是推断。

---

## 4. 仍需实测的项（官方文档回答不了）

以下五项官方均无记载，只能实验判定。**测前仍按现有串行纪律办**；测完按「日期 + 读数 + 结论」写回 `concurrency.md` / `multi-agent-git.md` 的「待实测」节。

| 编号 | 问题 | 最小实验 | 判据 |
| --- | --- | --- | --- |
| **Q-A** | 两个 **basename 相同**、路径不同的项目能否同时导入？CLI 会不会解析到错的那棵？ | 两棵 worktree 各跑 `cli project open --path <绝对路径>`，再 `cli project list` | 列 2 项 ⇒ 可共存；列 1 项 ⇒ 后者顶掉前者。再跑 `cli launch --project <目录名>` 看编的是哪棵树（产物路径反查） |
| **Q-B** | 两棵树能否**真并行**跑 CLI（而非只是排队）？ | 两棵树同时 `npm run hx:compile-only`（不经本仓锁） | 第二个卡在「正在编译中...」/ 互相假失败 ⇒ 只能串行；两者都出结果 ⇒ 锁粒度可再议（仍须查假失败面） |
| **Q-C** | 能否并存**两个 HBuilderX 实例**？`.lock` 是不是单实例锁？ | 双开 `HBuilderX.exe`（同一安装目录）；再试不同安装目录；观察 `%appdata%\HBuilder X\.lock` 的出现与报错 | 第二个被拒/报锁错 ⇒ 单实例；两者都可用 ⇒ 多开成立（注意：官方未记载，结论只能记「本机实测」） |
| **Q-D** | `cli project` 子命令在本机版本是否真不可用？`project list` 到底输出哪些字段（含不含路径）？ | `cli project list`、`cli project close --path <项目>` 各跑一次，保留完整 stdout | 官方文档说 3.1.5+ 有该子命令；本机 5.23 实测报「不存在」⇒ 结论写「版本差异」，**不写「官方没有」** |
| **Q-E** | 「新 `cli launch` 顶掉旧会话」在**当前版本**是否仍成立？ | 起一次真运行 → 不手动停 → 再发一次 `launch`（同项目） | 旧进程退出、目录句柄释放 ⇒ 成立；否则该条经验作废（它支撑着 `multi-agent-git.md` 的收尾配方） |

**测这些之前不要动的**：`$env:TEMP\hx-agent.lock` 锁、`HX_BUSY` 等待上限、`绝不 kill 主程序/cli` 三条红线。本文只改**措辞的依据**，不改**编排的现状**——官方未记载 ≠ 官方支持并发。

---

## 5. 附：本次抓取清单（可复算）

| 页面 | URL | 源 `.md` |
| --- | --- | --- |
| CLI 概述 | <https://hx.dcloud.net.cn/cli/README> | `/zh-cn/cli/README.md` |
| cli 无头模式 | <https://hx.dcloud.net.cn/cli/headless> | `/zh-cn/cli/headless.md` |
| cli project | <https://hx.dcloud.net.cn/cli/project> | `/zh-cn/cli/project.md` |
| cli pack | <https://hx.dcloud.net.cn/cli/pack> | `/zh-cn/cli/pack.md` |
| cli launch app | <https://hx.dcloud.net.cn/cli/launch-app> | `/zh-cn/cli/launch-app.md` |
| cli config | <https://hx.dcloud.net.cn/cli/config> | `/zh-cn/cli/config.md` |
| cli uniCloud | <https://hx.dcloud.net.cn/cli/uniCloud> | `/zh-cn/cli/uniCloud.md` |
| cli compile-uts | <https://hx.dcloud.net.cn/cli/compile-uts> | `/zh-cn/cli/compile-uts.md` |
| Linux CLI 使用示例 | <https://hx.dcloud.net.cn/cli/linux-uts-workflow> | `/zh-cn/cli/linux-uts-workflow.md` |
| Linux CLI 环境配置 | <https://hx.dcloud.net.cn/cli/linux-env> | `/zh-cn/cli/linux-env.md` |
| Linux HBuilderX CLI | <https://hx.dcloud.net.cn/Tutorial/install/linux-cli> | `/zh-cn/Tutorial/install/linux-cli.md` |
| 多窗体 | <https://hx.dcloud.net.cn/Tutorial/UserGuide/multi-window> | `/zh-cn/Tutorial/UserGuide/multi-window.md` |
| Command 列表 | <https://hx.dcloud.net.cn/Tutorial/Other/command> | `/zh-cn/Tutorial/Other/command.md` |
| Windows 启动问题排查 | <https://hx.dcloud.net.cn/Tutorial/Questions/WindowsStart> | `/zh-cn/Tutorial/Questions/WindowsStart.md` |
| 正式版更新日志 | <https://hx.dcloud.net.cn/Tutorial/changelog/ReleaseNote_release> | `/zh-cn/Tutorial/changelog/ReleaseNote_release.md` |
| Alpha 版更新日志 | <https://hx.dcloud.net.cn/Tutorial/changelog/ReleaseNote_alpha> | `/zh-cn/Tutorial/changelog/ReleaseNote_alpha.md` |
| uni-app manifest.json | <https://uniapp.dcloud.net.cn/collocation/manifest.html> | （HTML 页，已抓 2026-11-25） |

> 抓取日期一律 **2026-11-25**。文档站为 docsify SPA，直接 `GET` 页面 URL 只得到空壳（各页长度相同即为此故），正文必须取其 Markdown 源。
