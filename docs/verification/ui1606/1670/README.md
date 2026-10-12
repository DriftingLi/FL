# #1606 批2 学员端 14 页 ①a 逐页取证（PR #1670）

- 机型：小米 23049RAD8C（Android）· 真机 · 无线 adb
- 执行：agent 执行（ADR-0016 修订口径 · 2026-09-16）
- 节奏：一次分支收口（按批取证）
- 每页判据：页头「对话/积分明细/数据报告/职位详情/在线简历/附件简历」等 N03 页头件在位 + 核心内容区渲染非白屏

| # | 页面 | 证据 | 结果 |
|---|------|------|------|
| 01 | profile/settings（含 §8④ 清缓存 cell） | [01-settings.png](01-settings.png) | ✅ |
| 02 | profile/personal-info | [02-personal-info.png](02-personal-info.png) | ✅ |
| 03 | profile/wrong-questions | [03-wrong-questions.png](03-wrong-questions.png) | ✅ |
| 04 | points/points-detail | [04-points-detail.png](04-points-detail.png) | ✅ |
| 05 | points/task-center | [05-task-center.png](05-task-center.png) | ✅ |
| 06 | ai-assistant/ai-feature | [06-ai-feature.png](06-ai-feature.png) | ✅ |
| 07 | ai-assistant/ai-settings | [07-ai-settings.png](07-ai-settings.png) | ✅（真机多点 tap 探测 + OCR 复核：经「对话设置」可达） |
| 08 | ai-assistant/custom-models | — | ⛔ 产品门控不可达：app-nav-bar 模板从不渲染 rightIcon prop（AiChatNav 传 right-icon='⋯' 为死 prop，右侧菜单真机不渲染）；goManageModels 受 proUnlocked 门控（#1024 设计门）。定性记录于 issue，不作为页头件缺陷 |
| 09 | practice/practice | [09-practice.png](09-practice.png) | ✅ |
| 10 | practice/practice-report | [10-practice-report.png](10-practice-report.png) | ✅（practice 页「数据报告」入口） |
| 11 | jobs/job-detail | [11-job-detail.png](11-job-detail.png) | ✅（招聘页职位卡查看详情） |
| 12 | resume/resume-edit | [12-resume-edit.png](12-resume-edit.png) | ✅（简历总览在线简历入口） |
| 13 | resume/resume-attach | [13-resume-attach.png](13-resume-attach.png) | ✅（简历总览附件简历入口） |
| 14 | practice/practice-do | [14-practice-do.png](14-practice-do.png) | ✅（practice 页法规卡进入答题） |

## 机检行

```
Test Suites: 171 passed, 171 total  Tests: 3289 passed (rebase 前基线)
learnerUiContract.test.js 64/64 绿（H1 白名单/H2 scss/H5 主色/页头归位/右槽语义）
recruiterUiContract.test.js 41/41 绿 · batch2UiContract.test.js 17/17 绿
```
