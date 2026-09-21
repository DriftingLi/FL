# ①a 真机取证 · PR #1245（issue #1237：收藏落点表迁到 `utils/favoriteLanding.uts`）

## 被测树与环境

- 分支 `refactor/1237-favorite-landing-uts`，tip `31f6ff2c`（= `origin/master` `2e8c926b` + 本 PR 1 个提交）。
- 设备：小米 `23049RAD8C`（product `marble`）。⚠️ **本次入口是 TCP `192.168.0.212:43057`** —— USB 序列号
  `b32d8398` 当时**不在 `adb devices` 列表里**（本仓记过的「同一台机器两个入口」坑位），故 `hx:run` 显式传了当前序列号。
- 部署：`npm run hx:run -- -Device 192.168.0.212:43057`，机检行**原文**：

```
HX_RUN_DEPLOY deployed=true www=/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www=1789962071->1789974085 pid_before=io.dcloud.uniappx=32519 pid_after=io.dcloud.uniappx=28706 foreground=io.dcloud.uniappx reason=资源已落到设备：/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www 的 mtime 相对基线前进
HX_RUN mode=incremental compile=280 deploy=0 total=395 exit=ok
```

- 进页路径：`我的 → 收藏夹`（`pages/profile/favorites`）；点击坐标取自 `uiautomator dump` 的 `content-desc` bounds。

## 通过判据（逐条量出来的）

本票**把整张落点表从 `.uvue` 搬进 `utils/favoriteLanding.uts`**，故五类条目**全部真机点过**：

| # | 条目类型 | 落点（logcat 机检原文） | 渲染 | 产物 |
| --- | --- | --- | --- | --- |
| 1 | `chapter`「内燃机·故障与维修」 | `进入页面:/pages/courses/chapter-view?course_id=8&chapter_id=23` —— **双键齐**（ADR-0014） | ✅ 章节页标题/学习中/正文小节 | `02-chapter-view-after-tap.png` |
| 2 | `question`「叉车作业时可以载人行驶、站在货叉上作业。」 | `进入页面:/pages/practice/practice-do?mode=single&question_id=3` | ✅ 判断题 + 选项 + 已收藏 | `03-practice-do-after-tap.png` |
| 3 | `featured`「欢迎来到内容精选」 | `进入页面:/pages/featured/featured-detail?id=1` | ✅ | `04-featured-detail-after-tap.png` |
| 4 | `course`「内燃叉车动力装置（内燃机）」 | `进入页面:/pages/courses/course-detail?id=8` | ✅ | `05-course-detail-after-tap.png` |
| 5 | `topic`「测试markdown渲染」 | `进入页面:/pages/forum/forum-detail?id=20` | ✅ | `06-forum-detail-after-tap.png` |

五条 logcat 原文（按时间序）见 `logcat-landings.txt`；列表初始态见 `01-favorites-list.png` + 同名 `.content-desc.txt`。

## 未验证面（如实声明，不声称）

- **未用「手指」复核**：本次取证全部是 `adb input` 注入（先例：`adb` 注入在本设备可注入性每次现测）。
- **未逐个点五类之外的条目**：`收藏夹` 列表里还有第 2 个章节点，未点（同类型已验）；表外类型无法造夹具。
- **筛选 chips（全部/课程/章节/帖子/资讯/题目）本次未逐个点**：本票未改 chips（`typeOptions` 一字未动），
  它由 `utils/personalActivityContract.test.js` 的「前提仍成立」一条以接线断言钉着。
- **`course_id <= 0`（章节缺所属课程）在真机上造不出夹具**（#1148 结论：章节收藏恒带有效课程 ID）⇒
  该分支只有行为断言覆盖（`guards.md` 判据②的成对取证里有它：变异 M2/M4 都判红）。

## 产物清单

| 文件 | 说明 |
| --- | --- |
| `01-favorites-list.png` + `.content-desc.txt` | 我的 → 收藏夹 列表初始态（行类型与坐标来源） |
| `02-chapter-view-after-tap.png` | 点章节点 ⇒ `chapter-view`（双键）渲染 |
| `03-practice-do-after-tap.png` | 点题目点 ⇒ `practice-do` 渲染 |
| `04-featured-detail-after-tap.png` | 点资讯点 ⇒ `featured-detail` 渲染 |
| `05-course-detail-after-tap.png` | 点课程点 ⇒ `course-detail` 渲染 |
| `06-forum-detail-after-tap.png` | 点帖子点 ⇒ `forum-detail` 渲染 |
| `logcat-landings.txt` | 上面五条的 `进入页面` 机检原文（按时间序） |

口径：移动端 `docs/adr/0008-移动端验收门与证据.md`（①a 由 agent 出证）、
`docs/adr/0016-真机门的人工性收缩与按批取证.md`（①a 按一次分支收口）。
决策与收口点：根仓库 `docs/adr/ADR-0018-移动端P1通用能力.md` 补遗（#1237 更正落点表位置与守护分类）。
