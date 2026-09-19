# ①a 真机取证 · issue #1159（**改判后**重跑）

> 本目录是 #1159 **改判后**的 ①a 通过证据（一次分支收口，ADR-0016）。
> 改判前的失败记录（结论「本页收藏条目整行不可点、本票不能 close」）**不在本分支上** ——
> 它由已回退的旧分支 `fix/1159-personal-activity-favorite-landing`（tip `b671d7f6`）引入，
> 位于那边的 `docs/verification/personal-activity-favorite/issue-1159/`；其结论与本次反证
> 已在 #1159 正文「真机反证」段如实转录，并由根仓库 `ADR-0018` 补遗的「未复现声明」锚定。

## 被测树与环境

- 分支 `fix/1159-drop-favorite-tab`，tip `5b5607e9`（= `origin/master` `1dda2ca0` + 本票 1 个提交）。
- 设备：小米 `23049RAD8C`（serial `b32d8398`），HBuilderX 调试基座。
- 部署：`npm run hx:run -- -Device b32d8398`，机检行**原文**：

```
HX_RUN_DEPLOY deployed=true www=/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www=1789731025->1789738601 pid_before=io.dcloud.uniappx=10984 pid_after=io.dcloud.uniappx=26634 foreground=io.dcloud.uniappx reason=资源已落到设备：/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www 的 mtime 相对基线前进
HX_RUN mode=incremental compile=140 deploy=0 total=169 exit=ok
```

- 进页：`cli.exe launch app-android --project <项目> --deviceId b32d8398 --pagePath pages/profile/personal-activity`，
  到位判据 = logcat 出现 `进入页面:pages/profile/personal-activity`（实测第 132 秒到位）。

## 通过判据（逐条量出来的）

| # | 判据 | 产物 | 结论 |
| --- | --- | --- | --- |
| 1 | 主 tab 为「我的帖子 / **赞过** / 互动 / 游览记录」，**全页无「收藏」任何入口**，且**不再有第二行子 tab 栏** | `01-personal-activity-main-tabs.png` + `.content-desc.txt` | ✅ 通过 |
| 2 | 「赞过」直列点赞过的帖子（平铺 tab，无子 tab 中转） | `02-liked-tab-flat-list.png` + `.content-desc.txt` | ✅ 通过 |
| 3 | 点「赞过」首行 ⇒ 进入 `forum-detail` 且正文完整渲染 | `03-forum-detail-after-tap-liked-row.png` + `logcat-forum-detail-navigation.txt` | ✅ 通过 |
| 4 | 「互动」子 tab 栏（我的疑问 / 我的回复 / 我的围观）未受影响（`:sub` 模式仍有消费方） | `04-interact-subtabs-unchanged.png` + `.content-desc.txt` | ✅ 通过 |

判据 3 的机检原文（`logcat-forum-detail-navigation.txt` 全文）：

```
09-18 21:40:55.448 27735 31545 I console : [LOG]---BEGIN:CONSOLE---{"type":"string","value":"进入页面:/pages/forum/forum-detail?id=20 。[...]"}---END:CONSOLE---
```

判据 1 的文案原文（`content-desc` 抽取；注意 uni-app-x 的文案在 `content-desc` 而非 `text`）：

```
我的帖子  bounds=[55,557][215,611]
赞过    bounds=[365,557][445,611]
互动    bounds=[635,557][715,611]
游览记录  bounds=[865,557][1025,611]
```

—— 四个主 tab **之后直接是列表项**（`测试markdown渲染` 从 y=712 起），即**子 tab 栏整条消失**；
且全文案中不含「收藏」二字。

## 未验证面（如实声明，不声称）

- **未用「手指」复核**：本会话只有 `adb input` 注入。原失败记录里「维护者手指点击 7 分钟零跳转」
  这一条，本会话**无法复核**；改判后该承载面已摘除，现象**失去对象**（同族声明已写入 ADR-0018 补遗）。
- **本页不再有「收藏条目落点」这一验收面**：本页已无收藏条目可点。五类条目的落点由
  `docs/verification/favorites/1147/`（`favorites.uvue`，PR #1147）覆盖，本目录**不重复**取证。
- **「收藏过的帖子」的查看路径不在本页**：论坛详情页仍有「★ 收藏」按钮（帖子收藏走通用收藏，
  ADR-0018），但从论坛侧**看不到**收藏列表 —— 这是裁定 (a) 明确接受的取舍（查看路径 =
  `我的 → 收藏夹`，用「帖子」chip 筛选）。该取舍已写入 ADR-0018 补遗的「明确不做」段。

## 产物清单

| 文件 | 说明 |
| --- | --- |
| `01-personal-activity-main-tabs.png` + `.content-desc.txt` | 进页默认（我的帖子）：四个主 tab、无收藏入口、**无第二行子 tab 栏** |
| `02-liked-tab-flat-list.png` + `.content-desc.txt` | 「赞过」平铺列表（点赞过的帖子） |
| `03-forum-detail-after-tap-liked-row.png` + `.content-desc.txt` | 点「赞过」首行后的 `forum-detail`，正文与回复完整 |
| `04-interact-subtabs-unchanged.png` + `.content-desc.txt` | 「互动」tab：子 tab 栏未受影响 |
| `logcat-forum-detail-navigation.txt` | 判据 3 的 `进入页面` 机检原文 |

口径：移动端 `docs/adr/0008-移动端验收门与证据.md`（①a 由 agent 出证）、
`docs/adr/0016-真机门的人工性收缩与按批取证.md`（①a 按一次分支收口）。
决策：根仓库 `docs/adr/ADR-0018-移动端P1通用能力.md` 补遗（2026-09-18）。
