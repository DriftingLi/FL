# #652 ①a 真机取证（T14 域 api 批量收紧 A · 术前↔术后对照）

- **PR**：#1327 · **issue**：#652 · **分支**：`refactor/api-batch-a` @ `d3e6cd2b`
- **执行人**：agent 执行 · **日期**：2026-09-25
- **设备**：`192.168.10.54:40999`（Xiaomi `23049RAD8C` / Android 15），ADB `D:\android-sdk\platform-tools\adb.exe`
- **取证驱动**：`.ci-verify/t14/652-capture.ps1`（v2；`.ci-verify/` 不入库，故本 README 承载方法与机检行）

## 为什么这次两轮是「同一段取证代码路径」

驱动**不编译**，只负责导航 + 落定判定 + 取证；部署由每次 `cli launch app-android` 自己完成
⇒ 两轮唯一不同的输入是**工作树上那 6 个文件**，这正是 ①a 要证的自变量。

| 轮 | 树 | 树身份（`tree.txt` 记录的逐文件 sha256） |
|---|---|---|
| 术前 | **master 面**：`git checkout origin/master -- api/job.uts api/search.uts types/index.uts pages/jobs/*.uvue` + `git rm types/job.uts`（工作树呈 `M/M/M/M/M/D`） | `api/search.uts` `4787D31E…` · `api/job.uts` `A1BB2751…` · `pages/jobs/job-list.uvue` `0BECB665…` · `pages/jobs/job-detail.uvue` `45B28294…` |
| 术后 | **分支面**：`git checkout HEAD -- …`，`git status` 干净 | `api/search.uts` `2A06B067…` · `api/job.uts` `D26511AF…` · `pages/jobs/job-list.uvue` `BF536935…` · `pages/jobs/job-detail.uvue` `6300C2FD…` |

两轮之间逐字节**不同**的，只有这 4 个文件的 sha —— 换句话说，术后相对术前确实换了实现，
而不是又跑了一遍同一棵树。

## 三步取证（两轮完全同序）

1. `job-list`：深链 `--pagePath pages/jobs/job-list`；页身份 = launch 日志里的「进入页面」行 + a11y 文案 `招聘`
2. `job-detail`：**从列表点第一张卡**进详情（该页 `onLoad` 取 `options['id']`，深链需真实职位 id；
   无人值守取证里按 T13-B 态的先例以页面独有文案 `职位详情` 认页）；`input tap` 的生效与否**每次以设备侧事实判定**，不看退出码
3. `search`：深链进入 → `input tap` 输入框 → `input text abc` → 回车；**注入生效的判据**是 a11y 里输入框
   `text=` 属性真的等于 `abc`（uvue 的 `<input>` 值在 `text=`，不在 `content-desc=` —— 只读 desc 会把成功误判成失败）

每步都产：截图 + `uiautomator dump`（内容 + 逐节点 `bounds`）+ 窗口内 logcat 崩溃计数 + 机检行。

## 机检行（原样）

**术后（分支面 `d3e6cd2b`）**

```
T14_1A_job-list-1 entered=pages/jobs/job-list expectOk=True png=9F57BF5987C3E539 a11y=9FEA6AF2E64F7E8F fatal=0 anr=0 apis=200,200,200,200
T14_1A_job-list-2 entered= expectOk=True png=2BAC8CB6A38B1533 a11y=A4CC6761E66E76AE fatal=0 anr=0 apis=200,200,200,200
T14_1A_search-3 inject='abc' ok=True
T14_1A_search-3 entered=pages/search/search expectOk=True png=3CC053DC0B86DD47 a11y=0AC7B24A438F6298 fatal=0 anr=0 apis=200,200,200
```

**术前（master 面，同一驱动）**

```
T14_1A_job-list-1 entered=pages/jobs/job-list expectOk=True png=46AAA6622D9ED4CE a11y=9FEA6AF2E64F7E8F fatal=0 anr=0 apis=200,200,200,200,200,200
T14_1A_job-list-2 entered= expectOk=True png=564F4F334028F947 a11y=A4CC6761E66E76AE fatal=0 anr=0 apis=200,200,200,200,200,200
T14_1A_search-3 inject='abc' ok=True
T14_1A_search-3 entered=pages/search/search expectOk=True png=2E2A8D60F6C51BC3 a11y=0AC7B24A438F6298 fatal=0 anr=0 apis=200,200,200,200
```

## 对照结果

| 页面 | 节点数（术前/术后） | a11y **内容 + 逐节点 bounds** | 像素差（`scripts/lib/png-diff.mjs --ignore-top-rows 100`） |
|---|---|---|---|
| `pages/jobs/job-list` | 20 / 20 | **逐字节一致**（两边 sha256 同为 `9FEA6AF2E64F7E8F`） | `different=0 ratio=0 changed=false` |
| `pages/jobs/job-detail` | 18 / 18 | **逐字节一致**（两边 sha256 同为 `A4CC6761E66E76AE`） | `different=0 ratio=0 changed=false` |
| `pages/search/search` | 37 / 37 | **逐字节一致**（两边 sha256 同为 `0AC7B24A438F6298`） | `different=0 ratio=0 changed=false` |

`--ignore-top-rows 100` 只让开状态栏 —— 那里的时钟与实时网速是应用控制不了的读数
（`Compare-ScreenFrames` 头部记的同一血账），100px 之外**一格像素都没差**。

结论：**零漂移**。收紧前后真机逐页渲染与可达性树完全一致，且两轮 `fatal=0 anr=0`。

## 业务链路确实跑通了（不是空转）

- `/api/jobs?page=1&page_size=20` → **200**（学员账号；接口落在 `CapabilityRequired(job.apply)` 下，
  非学员账号会 403 —— 本轮前先由维护者在真机登录一次学员账号）
- 点卡进详情后 `/api/jobs/:id` → **200**，详情页渲染出**映射后的字段**：
  `专业叉车维修员 / A公司 / 💰 6000-9000 元/月 / 📍 广州 / 🕐 2026-09-02 发布 / 26 天后可再次投递`
  ⇒ `getJobDetailApi` → `getMapped<JobPosting>` → `buildJobPosting` 在**真实服务端数据**上跑通
- `/api/search?keyword=abc&credential_id=1` → **200** ⇒ 收紧后的 `searchAllApi` 出口跑通

## 一次被本门自己拦下来的事故（如实记录）

首轮「术后」取证**作废重跑**：当时我在同一工作树上做 `apiBatchAContract` 的**变异实测**
（把 `buildJobPosting` 的 `region` 改成字面量），而该轮的 `cli launch` 恰在此时编译 ——
于是术后详情页 a11y 里出现 `MUTANT-4242 @ [127,670][1045,724]`（术前同位置是 `广州`）。
**是 ①a 的逐节点对照把它揪出来的**，随后：确认工作树已还原（`region`/`keyword` 回退两处 sha 归位）→ 重跑整轮术后。
上面表里是重跑后的结果。

教训（跨票通用）：**变异实测必须在真机取证结束之后再做**，否则「被测物」与「截图」不在同一棵树上。
