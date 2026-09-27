# #1335 ①a 真机成对取证 —— course 域 4 处 catch-mock 整体退役（术前 ↔ 术后）

- **Issue**：#1331（#1265 批⑥）
- **执行人**：agent 执行 · **日期**：2026-09-27
- **设备**：Xiaomi `23049RAD8C`（marble），无线调试 adb `192.168.10.54:43211`，基座 `io.dcloud.uniappx`（HBuilderX run 模式增量部署）
- **术前锚点**：`master@2fc40aca`（其运行时面对 `3b59bb6b` 逐字节相同 —— `2fc40aca` 仅 md 文档）
- **术后锚点**：本分支 head `0fd41a44`（`c61c395a` 合并 `2fc40aca`），部署机读行 `HX_RUN_DEPLOY deployed=true www=…1790509572->1790510182`

## 结论一览（逐状态结果表）

| 状态 | 术前（`2fc40aca`） | 术后（`0fd41a44`） | 判定 |
| --- | --- | --- | --- |
| 断网 · 列表页 `pages/courses/courses` | 渲染 **6 门假课程**（叉车基本构造 / 液压传动系统 / 叉车驾驶操作 / 日常维护保养 / 货物装卸作业 / 故障排查与应急），无失败态 → `before-offline-courses.jpg` | 「**课程加载失败，请检查网络后重试**」+ `重试` 按钮，零假课程 → `after-offline-courses.jpg` | **必红成立**（同一断网下术前演假、术后落真实失败态） |
| 断网 · 详情页 `course-detail?id=1` | 渲染 **mock 详情**（理论 16 学时 / 实操 8 学时 / 关联证书「有效期 365 天」——真数据这两栏为 `-` 且无此证书块）→ `before-offline-detail.jpg` | 「**课程详情加载失败**」+ `重试` → `after-offline-detail.jpg` | **必红成立** |
| 断网 · 商城 `pages/mall/mall` | 渲染 **假商品**（「已有0人参与学习」×4 + 货物装卸作业）→ `before-offline-mall.jpg` | 「**商品加载失败，请检查网络后重试**」+ `重试` → `after-offline-mall.jpg` | **必红成立** |
| 恢复网络 → 点 `重试`（重试可用性） | — | 按钮节点（a11y `desc='重试'`，bounds 中心 `(540,1024)`）点击 8s 后恢复 **真实课程列表**（内燃叉车动力装置…液压系统原理与维护，32 行）→ `after-retry-courses.jpg` | **重试入口真实可用** |
| 在线 · 列表页（必不红） | 真实课程列表（内燃叉车动力装置（内燃机）/ 场（厂）内机动车辆基础 / 故障诊断与排除进阶…）→ `before-online-courses.jpg` | 同一真实列表（32 行）→ `after-online-courses.jpg` | **必不红成立**（成功路径行为不变） |
| 在线 · 详情页（必不红） | 真实详情（叉车基础知识概述 / 理论学时 `-` / 3 章 / 2 小时 / 进度 33.33%）→ `before-online-detail.jpg` | 同一真实详情实体（标题 / 标签 / 简介 / 学时 `-` / 章节 / 时长逐条一致）→ `after-online-detail.jpg` | **必不红成立**（进度 / 收藏两栏的账号态差异见诚实声明第 3 条） |

每张 jpg 同名 `.content-desc.txt` 为该帧的 a11y 文本判据（uvue 文本只走 `content-desc`，截图是判据输入的原始件）。

## 取证手法

- **断网通道**：`settings put global http_proxy 192.168.10.55:18899` 指向 PC 上的选择性中继监听器 —— 只放行 cli 拉运行包的 `127.0.0.1:8000`（否则 launch 永远卡在「同步手机端程序文件」），**其余域 accept 后立即 close** ⇒ 应用侧全部请求落 `request system error`（等价断网，Wi-Fi 与无线调试 adb 不受影响）。取证结束 `settings delete global http_proxy`，回读 `null`。
- **进页方式**：`cli launch app-android --pagePath <页> [--pageQuery id=1] --project … --deviceId 192.168.10.54:43211`；页身份 = 应用日志 `进入页面:"pages/…"` 行；落定 = `Wait-NavSettled`（进页行 + ≥12s + 相邻帧稳定，**fail-closed**，不落定不出图）。
- **文本判据**：每帧一次 `uiautomator dump`，取全部非空 `content-desc` 逐行入 `<帧名>.content-desc.txt`；重试按钮定位用新鲜 dump 的精确正则 `^重试$`（首轮用 `重试` 误匹配到含该词的整句文本节点，该轮帧作废重拍，见诚实声明第 4 条）。
- **入仓件**：原始 PNG → 720w / q88 JPEG；请求行、页面进入行、`using mock data` / `load failed` 行按帧归集到 `before-machine-lines.txt` / `after-machine-lines.txt`。

## 机器行（console 日志证据）

**术前**（`before-machine-lines.txt`，断网支 4 条 mock 行）：

```
19:27:47.506 [getCourseListApi] network failed, using mock data: … at api/course.uts:375   ← 商城
19:37:38.579 [getLevelsApi] network failed, using mock data: … at api/course.uts:440      ← 列表筛选条
19:37:38.594 [getCourseListApi] network failed, using mock data: … at api/course.uts:375   ← 列表
19:46:09.487 [getCourseDetailApi] network failed, using mock data: … at api/course.uts:454 ← 详情
```

**术后**（`after-machine-lines.txt`，**零** mock 行，页面级失败态行）：

```
20:00:14.285 [mall] load failed: … at pages/mall/mall.uvue:169
20:04:10.931 [course-detail] load failed: … at pages/courses/course-detail.uvue:238
20:07:41.754 [courses] load failed: … at pages/courses/courses.uvue:189
```

两侧断网窗口内 `[request] <<< FAIL errMsg= request system error` 行数、时间窗一致（同一触发器），差异只在**错误的去向**：术前进 mock 兜底，术后上抛到页面失败态。

## 诚实声明

1. **设备侧写操作逐条**（全部为可逆设置项，均已恢复）：`settings global http_proxy` 置位 / 删除（多轮，收尔回读 `null`）；`private_dns_mode/hostname` 试验性置位 + 恢复 `null`（MIUI `DnsManager` 强制 `PrivateDnsConfig{off}`，该方案实测无效，已弃用）；`svc wifi disable/enable` 试验（**自断无线调试通道**，由维护者在手机上重开无线调试恢复，端口变为 `…:43211`）；`settings global stay_on_while_plugged_in 3` + 结束时 `delete`；`input keyevent 224` 唤屏；`input tap` ×若干（含 1 次误中文本节点的无副作用触碰与 `^重试$` 按钮）；`uiautomator dump` / `screencap`（只读 + 临时文件即时删除）。
2. **只验了「网络失败」支**：①a 的必红窗口 = 断网（`request system error`）。5xx 支未在真机逐状态拍摄 —— 两支在 `api/request.uts` 走同一条 reject 通道（本票把 4 个 `.catch` 整段删除，5xx 与断网同样上抛），5xx 侧的判别力由 ③ 的注入变异对承担：把 `.catch` 装回去 ⇒ 两把锁同时红（`Expected: 0, Received: 1` + `not.toContain('getMockCourseList()')`），摘除后 135 suites / 2628 用例全绿。
3. **在线详情帧的账号态差异**：术前帧显示「学习进度 33.33% / 已完成 1/3 章 / ♥已收藏」，术后帧为「开始学习 / ♡未收藏」——来源是取证窗口 20:08:16 设备侧发生了一次指纹重登（触碰者不可归因，推定人为；refresh 应答 `user_id=8`）与学习态变化，**非数据源差异**：两侧课程实体字段（标题 / 入门·维修标签 / 简介 / 理论·实操学时 `-` / 章节数 3 / 课程时长 2 小时 / 三章标题与 40 分钟）逐条一致，见两份 `*online-detail.content-desc.txt`。
4. **被作废并重拍的轮次**（全部如实记录，入仓文件均为成功轮的产物）：
   - `before-offline-detail` 首轮：HBuilderX 单实例排队把进页拖过 300s 上限，`Wait-NavSettled` fail-closed 判 NOT-SETTLED ⇒ 超时提到 480s 补拍成功；
   - `after-online-courses` 首轮：上一运行会话被顶掉时基座自灭（日志止于「正在启动uni-app x调试基座…已停止运行」），480s 无进页行 ⇒ 重拍成功；
   - `after-retry-courses` 首轮：`Find-Bounds` 用 `重试` 误匹配到整句文本节点（点击落在不可点节点，恰逢断网恢复期的登录过渡页，拍到应用级过渡帧）⇒ needle 改 `^重试$` 后整轮重拍成功。
   - 期间断网窗口内 auth 守卫曾把应用导向登录页（`进入页面:/pages/login/login`、`/pages/recruiter/login`，20:04:52 / 20:08:11）——属**既有认证层对网络失败的反应**，在每帧落定之后发生，未污染任何入仓帧；本票未触及认证层。
5. **未验证面**：①b（未命中能力面）、②（未命中 MP-WEIXIN 面）、④a / ④b 免门理由见 PR 正文；跨端其它平台（H5/iOS）不在本票射程。

## 产物清单（24 文件）

| 文件 | 说明 |
| --- | --- |
| `before-offline-{mall,detail,courses}.jpg` + `.content-desc.txt` | 术前断网三帧（假数据可见 = bug） |
| `before-online-{courses,detail}.jpg` + `.content-desc.txt` | 术前在线两帧（真数据基线） |
| `after-offline-{mall,detail,courses}.jpg` + `.content-desc.txt` | 术后断网三帧（失败态 + 重试） |
| `after-retry-courses.jpg` + `.content-desc.txt` | 术后恢复网络点重试 → 真实列表 |
| `after-online-{courses,detail}.jpg` + `.content-desc.txt` | 术后在线两帧（成功路径不变） |
| `before-machine-lines.txt` / `after-machine-lines.txt` | 两侧请求行 / 进页行 / mock·失败行归集 |
