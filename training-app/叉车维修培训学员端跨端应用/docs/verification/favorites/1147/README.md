# ①a 真机验证截图 · PR #1147（#1089 PR-B：我的收藏落点 + FavoriteItem.course_id）

设备：小米 `23049RAD8C`（serial `b32d8398`），Android，**HBuilderX 调试基座**（`npm run hx:run` 增量运行 →
`cli.exe launch app-android --project <项目> --deviceId b32d8398 --pagePath pages/profile/favorites` 深链进页）。
留档口径：移动端 `docs/adr/0008-移动端验收门与证据.md`（①a 由 agent 出证）+ `0016`（①a 按「一次分支收口」跑一次）。

**被测树**：分支 `fix/1089-favorite-landing`，head `6351ed66`（= 本 PR 的 head；含 master 合并）。
**本次改动涉及的落点分支**：chapter（双键）、question、featured。

## 图片

| 文件 | 构建 | 说明 |
| --- | --- | --- |
| `01-favorites-list-chapter-and-question-rows.png` | `6351ed66`，14:51 | 「我的收藏」列表：**章节**行（`内燃机·总体构造与工作原理`，2026-09-18）+ **题目**行（`叉车作业时可以载人行驶、站在货叉上作业。`）都在列表里（夹具为自然存在的数据，未新造） |
| `02-chapter-view-rendered-after-tap.png` | 同上，14:52 | 点**章节**行 → 进入 `chapter-view`：标题 + 「学习中 预计 1小时10分」+ **章节正文完整渲染**，「正在加载...」已消失 |
| `03-question-practice-do-after-tap.png` | 同上，14:53 | 点**题目**行 → 进入 `practice-do` 单题模式：判断题题干 + 对/错选项 +「已收藏」态 + 提交答案 |

`ui-dump-favorites.content-desc.txt` 是 `uiautomator dump --compressed` 抽出的可见文案与 **bounds**
（点击坐标一律取 bounds 中心，**不按截图目测** —— ADR-0008「①a 取证手法补遗」第 2 条）。

## 判据（都是量出来的，不靠「像不像」）

1. **chapter 落点两键生效**：点章节行后**没有**停在「正在加载...」。
   该缺陷的原始形态是 URL 只有 camelCase `chapterId`（死键）⇒ `chapter-view` 的
   `onLoad` 读不到 `course_id`/`chapter_id` ⇒ `loadDetail` 提前 return ⇒ 永久「正在加载...」。
   本次实测**正文渲染完成**（截图 02），说明 `course_id` 与 `chapter_id` 都到了。
2. **question 落点新增分支生效**：截图 03 是 `practice-do` 单题页（改前该类型为静默 no-op，点了没有任何反应）。
3. **点击判据按画面变化判**（不看退出码）：`input` 类命令失败也常返回 0；本次每次点击都以
   `uiautomator dump` 的文案变化为准（列表 → 章节页 → 列表 → 单题页）。
4. **`input` 可注入性本次现测通过**：`adb shell input tap` 全程无 `SecurityException`
   （ADR-0008 第 3 条要求每次现测，2026-09-13 记过系统硬拒、09-15 记过可用）。

## 如实声明的边界（未做/未覆盖的，不在此宣称）

- **「资讯」（featured）落点未逐条真机验证**：本账号收藏里没有 featured 条目，未为它单独造夹具；
  其 URL 等式由 `utils/favoritesLandingContract.test.js` 的断言覆盖（`/pages/featured/featured-detail?id=`），
  **不据此声称真机验过**。
- **`course_id <= 0` 分支未在真机造出**（需要一条所属课程缺失的章节收藏）：同样由契约测试的
  「不跳转 + `showToast`」断言覆盖。
- **首屏加载偏慢（观察，非缺陷）**：点击后 **+3s** 时仍显示「正在加载...」，**+11s** 时正文已渲染完成。
  本次不是「永久加载」（原始缺陷形态），但与「秒开」差距明显，值得记一笔供后续观察。
- 未跑 ② 微信门 / ④b 云打包（本批未动三份 json、无新增页面 ⇒ 按现测免）；①b 未签（收藏不属能力面）。
