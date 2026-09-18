# #1011 真机取证（①a）—— **通过**：入口存在且真的能进「练习记录」页

> 采集时间：2026-09-18 19:25–19:32 · 被验树：`feat/mobile-1011-practice-records-entry`
> （本票提交 `279b29ba` + `origin/master` 合并 ⇒ 取证时 head = `6f16038b`）
> 设备：`b32d8398`（23049RAD8C / Android 15 / `marble`）· 应用：`io.dcloud.uniappx`（apps/`__UNI__1C1D180`）

## 结论

**①a 通过。** 本票要修的是「页面已注册但全 App 无入口 ⇒ 用户不可达」，真机上**逐字观察**到了：
「我的」宫格出现第 9 项「练习记录」，点它之后**确实进入** `/pages/profile/practice-records`。

### 部署判据（本树确实落到设备）

```
HX_RUN mode=incremental compile=90 deploy=0 total=103 exit=ok
HX_RUN_DEPLOY deployed=true www=/sdcard/Android/data/io.dcloud.uniappx/apps/__UNI__1C1D180/www
              =1789730035->1789730949  pid_before=io.dcloud.uniappx=5564 → pid_after=io.dcloud.uniappx=7538
              foreground=io.dcloud.uniappx
```

④ 本地编译门（④c，**在合并 master 之后重跑**）：`KOTLIN_ALL_RESULT errors=0 classes=1400 files=108`，
导出路径落在**本工作树** `D:\FL\wt-1011\…\unpackage\resources\app-android`。

### 机检行（`check-1011.py`，退出码 0，全文见 `check-1011.txt`）

```
[A] 「我的」宫格第 9 项「练习记录」，顺序成立（节点下标 20 → 36，末项）   PASS
[B] 工具宫格原 8 项全在（新增而非替换）                                  PASS
[C] 点入口后到达 practice-records（页身份 + 统计卡齐全）                  PASS
[D] 列表/空态如实报告                                                     INFO
check-1011: OK (0 条断言未过)
```

### 逐字证据

「我的」页（`01-profile-grid-a11y-strings.txt`）工具宫格 9 项：

```
学习记录 / 收藏夹 / 错题本 / 笔记本 / 学练计划 / 学习资料 / 就业在线 / 任务中心 / 练习记录
```

点「练习记录」之后的页面（`02-practice-records-a11y-strings.txt`）：

```
‹ 练习记录
0 今日做题 · 2 累计做题 · 50% 正确率 · 0 连续天数
全部 / 单选题 / 多选题 / 判断题
共 2 条记录   对 1 · 错 1
✗ 其他 练习 2026-09-17
✓ 其他 顺序练习 2026-09-17
没有更多了
```

页面切换的运行时证据（logcat 逐字，与上面是同一次点击）：

```
09-18 19:30:48.968 I console : 进入页面:/pages/profile/practice-records 。[{创建dom元素个数:40个,…onReady总耗时:225ms}]
```

### 本批**能证明**

1. 「我的」工具宫格**新增了第 9 项「练习记录」**，且**原有 8 项一个都没少**（不是顶掉某一项）。
2. 点它**确实跳转**到 `/pages/profile/practice-records`（logcat `进入页面:` + 目标页渲染同时成立）。
3. 目标页是**功能完整**的（统计卡 / 题型筛选 / 分页列表 / 汇总），即「接线后用户可用」，不是只通了个空壳。

### 本批**未主张**（写实，不粉饰）

1. **空态文案「当前证件下暂无练习记录」本批未观察到** —— 本账号当前证件下**有 2 条练习记录**，
   页面走的是列表态（`没有更多了`）。该文案的价值场景是「切到没练过的证件」，
   需要换证件或造夹具才能复现，**本批不做**，故**只主张「文案已改」这一代码事实，不主张其真机呈现**。
2. 筛选（全部/单选/多选/判断）与分页交互本批未逐项点击取证。
3. 未主张「最近错误时间」等服务端口径相关的展示（不属本票）。

### 手法坑位（供后续会话复用）

- **`--pagePath pages/profile/profile` 深链没生效**：`cli launch app-android --pagePath pages/profile/profile`
  之后前台停在**首页**（a11y 显示课程商城/题库练习/热门课程 + tabBar）。改用**点 tabBar 的「我的」**才到位。
  （对照：#1083 用 `--pagePath pages/profile/wrong-questions` 是生效的 ⇒ 深链是否生效**每次现测**，别照抄。）
- **宫格项在 a11y 里是两个节点**：图标（`content-desc="&#128202;"`）+ 文案（`content-desc="练习记录"`），
  外面才是可点的 `ViewGroup`（本次 `bounds=[35,1475][288,1672]`）。**点父 ViewGroup 的 bounds 中心**最稳；
  另外「前一个节点」不是上一项文案而是图标实体 —— 判顺序要拿**已知 9 项的下标单调递增**来判，
  不能拿相邻节点当判据（首版就是这个错，已修）。
- **`input tap` 前先 `input keyevent KEYCODE_WAKEUP`**：屏幕超时会吞掉点击（本轮第一次点击就是这样丢的）。
- `uiautomator dump` 会间歇性返回 0 个文本节点（同帧重试即正常）—— 本票判据是文案级，重试一次即可。

---

## 第二轮（夹具补齐，2026-09-18）—— 「当前证件下暂无练习记录」空态

> 被验树：`origin/master` `114a2095`（与设备上部署的那棵树**逐文件一致**，判据见
> `../1083/README.md` 第二轮开头的 `git diff` 读数）

做法（不需要任何登录凭证）：在首页「选择学习阶段」把证件切到 **低压电工证**（该证件下没有练习记录），
再走「我的」→ 工具宫格第 9 项「练习记录」。真机 a11y 逐字：

```
‹ 练习记录
0 今日做题 · 0 累计做题 · 0% 正确率 · 0 连续天数
全部 / 单选题 / 多选题 / 判断题
共 0 条记录
当前证件下暂无练习记录        ⇒ #1011 新增文案，真机观察到了
```

运行时佐证：切换证件打 `PATCH /api/me/credential {"credential_id":2}` → **200**（切到低压电工证）。
看完后**已切回**「叉车司机N1证」（末态已核：首页证件位显示 `叉车司机N1证`）。
产物：`03-practice-records-empty.png` + `03-practice-records-empty-a11y-{dump.xml,strings.txt}`。

⇒ **#1011 三条判据（入口存在 / 跳转成立 / 空态文案）现在全部有真机证据**，上文「本批未主张」第 1 条
**正式撤回**。
