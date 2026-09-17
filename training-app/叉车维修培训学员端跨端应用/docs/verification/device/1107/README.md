# #1107 ①a 真机取证记录（证件下发口径核对与登记）

## 被验对象

- 票：#1107（`refactor(mobile-credential)`）；根仓口径：ADR-0056 §2 / ADR-0057
- 分支：`feat/1107-mobile-credential-scope`
- 被验树：`c7f9c799`（本 PR 的代码提交；本记录提交只新增 `docs/verification/**`，不含运行时面）
- 设备：`b32d8398` / `23049RAD8C` / USB；`adb` = `D:\android-sdk\platform-tools\adb.exe`
- 页面：`pages/search/search`、`pages/courses/courses`、`pages/mall/mall`、`pages/practice/practice`
  （走 `cli launch app-android --pagePath` 深链切页；**未注入任何输入事件**）

## 机检行

```
AUTO_SCREENSHOT_RESULT ok=True pages=4 skipped=0
导航已落定（search 207s / courses 150s / mall 170s / practice 168s；相邻两次采样差异 0% ≤ 0.5%）
```

## 请求侧判据（本次改动的实质）

`config/env.uts` 的 `ENABLE_DEBUG_LOG = IS_DEV` 在真运行下打开，`api/request.uts:228` 会打印请求行
（logcat `[request] >>>`）——**这是「源码里有传参点」之外唯一能证明「请求真的带上了」的判据**：

| 端点 | logcat 原文（节选） | 对应改动 |
| --- | --- | --- |
| `GET /api/courses` | `…/api/courses?page=1&page_size=12&credential_id=1` | `getCourseListApi` 新增显式下发（课程页 19:46:50、商城页 19:49:44 各一次） |
| `GET /api/tags` | `…/api/tags?credential_id=1` | `getTagsApi` 新增显式下发（练习总览 19:52:37） |
| `GET /api/catalog/tree` | `…/api/catalog/tree`（**无** `credential_id`） | 口径表「不分区」行：移动端只消费 `specialties`，传了是 no-op |

## 产物与判据

| 产物 | 判据 |
| --- | --- |
| `search-after.jpg` / `courses-after.jpg` / `mall-after.jpg` / `practice-after.jpg` | 目标页落定后 `adb exec-out screencap`；720 宽 JPEG q75（合计 276.5 KB，限 1.5 MB）；四页 sha 互不相同、`HashConflicts` 为空 |
| `logcat-app.txt` | 过滤后的应用日志（`[request] >>>` 行 + `进入页面:` 行；22 行 / 7.8 KB） |

### 逐页目视核对（agent 读图，不是「已入仓」充数）

| 页 | 帧内可见 | 与请求侧配对 |
| --- | --- | --- |
| courses | 「课程」标题 + 课程列表**非空**（内燃叉车动力装置（内燃机）认证 共7个课时 / 场（厂）内机动车辆基础 / 故障诊断与排除进阶 / 货叉操作技能训练 …） | 19:47:03 帧 ← 19:46:50 `GET /api/courses?…credential_id=1` |
| mall | 「商城」+ 必买清单 + 商品列表**非空**（同一批证件 1 的课程） | 19:49:57 帧 ← 19:49:44 `GET /api/courses?…credential_id=1` |
| practice | 「题库练习」+ 标签题数（法规 280 / 结构 126 / 液压 68 / 电气 75 / 制动 97） | 19:52:50 帧 ← 19:52:37 `GET /api/tags?credential_id=1` |
| search | 搜索页正常渲染（输入框 + 搜索历史），无回归 | 本页改动仅为变量改名；未输入关键词 ⇒ 本轮**没有** `/search` 请求 |

## 未主张事项（写实）

1. **`POST /practice-mode/progress` 的 body 传参没有运行期证据**：要答完一题才触发保存，本轮不注入输入事件
   （`device-capture.ps1` 的只读红线；该设备 `INJECT_EVENTS` 历史性被拒）。该条只有契约锁 +
   与 Web `api/practiceMode.ts` 同口径的源码判据。
2. **`/search` 无请求侧日志**：搜索需要输入关键词。该页改动是变量改名（显式下发行为自 #979 M6 起已在），
   本帧只证「页面未回归」；传参由 `utils/searchContract.test.js` + `utils/credentialScopeContract.test.js` 双锁覆盖。
3. **未做像素级基线对比**：`docs/verification/` 下没有这四页的历史基线，本轮**未**建立（避免拿本轮 after 自证基线）
   ⇒ 只主张「目标页落定 + 帧内内容如上」，不主张「与某历史版本逐像素一致」。
4. **取证期间只结束了本会话自己的常驻 `cli.exe`（PID 26532）**：为了把项目目录名改回（HBuilderX **按项目名解析**，
   多 worktree 同名会互相误命中）。按移动端 `docs/adr/0008-移动端验收门与证据.md` ①a 手法补遗第 8 条执行 ——
   逐个核对 `CommandLine`、只结束引用本项目路径的进程，**未碰 `HBuilderX.exe` 主程序、未碰其它会话**。
