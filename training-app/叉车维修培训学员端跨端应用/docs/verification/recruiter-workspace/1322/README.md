# #1322 抽屉对象 prop 名义分裂 CCE —— ①a 真机证据

- 票：[#1322](https://github.com/DriftingLi/FL/issues/1322) `RecruitResumeFiltersReactiveObject cannot be cast to RecruitResumeFiltersProp`
- 分支 / 提交：`fix/1322-filter-drawer-cce` @ `1a308509`（fix）+ `b649a04b`（ADR-0007 回写）
- 设备：Redmi 23049RAD8C（marble）`192.168.10.54:43211`，Android，uni-app x debug 基座
- 页面：`pages/recruiter/resumes` + 组件 `pages/recruiter/components/recruiter-filter-drawer.uvue`
- 执行人：agent 执行（zhengcookie 会话代录）；日期：2026-09-27

## 结论

| AC | 判据 | 结果 |
| --- | --- | --- |
| AC1 | logcat 票面 CCE `cannot be cast to …RecruitResumeFiltersProp` = 0（≥2 轮进入窗口） | ✅ 轮1 42210 行 / 轮2 24017 行，两轮均 0 |
| AC2 | 抽屉级开→输→用→回显截图（此前无任何抽屉级证据） | ✅ 4 态截图 + uiautomator dump 断言，列表重载由 2× `resumes?page=1` 请求证明 |
| AC3 | ④a `npm run build:compile` | ✅ `COMPILE_RESULT errors=0 clean=True`（另见 `.ci-verify/build.log`，19:11 编译成功） |
| AC4 | ③ `npm run test:unit` 全绿 + 锁演进写明理由 | ✅ 135 套 / 2630 例；R7 4→6 改写理由与判别力（术前红 4）见 PR 正文 |
| AC5 | 不引入新报错行 | ✅ 两窗口 `FATAL EXCEPTION = 0`、`E AndroidRuntime = 0` |

## 文件

| 文件 | 内容 | sha256（前 16 位） |
| --- | --- | --- |
| `logcat-summary.txt` | 两轮 logcat 窗口统计、逐态注记、外席会话归因声明 | — |
| `resumes.png` | 轮 2 进入落定截图（页面渲染态，20:33:48） | `7E58DD78C9D509D3…` |
| `resumes-drawer.png` | 开抽屉：默认回显（region 空占位「如：杭州」、薪資占位「留空不限」） | `E7E82C1C0C7F8024…` |
| `resumes-typed.png` | 输入后：`shanghai` + `8000`（dump 文本断言在场） | `116B47BD773AEAF5…` |
| `resumes-applied.png` | 点「应用筛选」后：抽屉已关、列表区（上方触发 1 次 `resumes?page=1` 重载） | `2826D625DBE62F69…` |
| `resumes-echo.png` | 重开抽屉：回显 `shanghai` / `8000`（与父页 filters 一致） | `5AC0EADFCBF7651D…` |

## 复现口径

```
# 窗口重开（每轮判据独立归因）
adb -s 192.168.10.54:43211 logcat -c
# 部署本分支构建并落定目标页
cli launch app-android --pagePath pages/recruiter/resumes --project <worktree>/training-app/叉车维修培训学员端跨端应用 --deviceId 192.168.10.54:43211
# 抽屉链路：筛选(884,173) → region(633,308)/salary-min(424,959) 输入 → 应用筛选(776,2307) → 重开
# 判据
adb -s 192.168.10.54:43211 logcat -d | grep -c "cannot be cast to"   # 期望 0
```

外席会话（同机主树 `fix/1331-course-catch-mock`）与窗口边界详见 `logcat-summary.txt` 的归因声明。
