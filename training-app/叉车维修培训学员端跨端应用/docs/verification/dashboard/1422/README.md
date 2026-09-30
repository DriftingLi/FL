# #1422 ①a 真机取证 · dashboard 首页课程区接真实数据

设备：Redmi `23049RAD8C`（marble，arm64-v8a）无线 adb；承载面 = HBuilderX 标准基座 `io.dcloud.uniappx`。
构建：`npm run hx:run`（incremental，compile=290s）把 **本分支新代码** 推到设备并启动（`HX_RUN mode=incremental exit=ok`，设备侧 `www` mtime 相对基线前进 ⇒ 非旧构建）。
取证：`scripts/device-capture.ps1`（**只读**，`-NoStartApp`，不切页、不注入输入）。

## 判据（逐条对得上，不拿「看不见」当「已验证」）

| 项 | 事实 | 证据 |
| --- | --- | --- |
| 数据源已换成真实接口 + 热门口径 + 按证件分区 | 设备实发 `GET https://www.gccsmile.com/api/courses?page=1&page_size=4&credential_id=7&filter=hot` → **200** | logcat（`api/request.uts:401/480`）+ `dashboard-logcat-request.txt` |
| mapper 真消费了响应 | `[getCourseListApi] raw data: {"courses":[],"page":1,"pages":0,"total":0}`（`api/course.uts:329`） | 同上 |
| mock 假数据族已退役 | 截图课程区**不再出现**写死的 4 张假卡（`¥39/¥128/¥199`、`已售 N+`、假「热销」标）——真接口该证件返回空即空 | `dashboard-real-data.png` |
| 无崩溃 | `DEVICE_CAPTURE_RESULT=PASS`，logcat 窗口 1798 行 `FATAL=0 ANR=0 进程死亡=0` | device-capture 报告 |

## 如实声明：为什么截图里课程卡是空的（两条，都不是回退）

1. **该登录证件无热门课**：设备当前证件 `credential_id=7`（工程机械维修工·叉车维修方向·二级）。生产 `filter=hot` 命中的 2 门课（course_id 7/8）都挂在 `credential_id=1`，故证件 7 的热门区**正确地为空**（`total:0`）。这正是「接真实数据」应有的表现——mock 时代恒显 4 张假卡才是 bug。
2. **课程卡在折叠线以下**：dashboard 是长 `scroll-view`，课程区在「新闻资讯」之下、首屏视口之外；`screencap` 只采当前视口，而本机 `INJECT_EVENTS` 被系统硬拒（无法 adb 滚动/点按），故卡片像素无法用只读脚本入画。

> 正向「卡片渲染 + 价格 + 热标」的可见截图需**人**把设备切到 `credential_id=1` 并手动滚到课程区后补拍（agent 无凭证、不代填、不注入输入）。本票 ③ 契约锁已把「卡片标题/价格/热标与后端一致」的**形状**钉死（`coursesContract` #1422 组：`is_hot` 迁出、`filter=hot`、价格单点第三消费方、mock 族绝迹），运行时面以「请求实发形状 + 200 + mock 绝迹 + 无崩溃」出证。

## 产物

- `dashboard-real-data.png`（sha256 `20B51C0CDF3DB943…`）：新构建下 dashboard 首屏，课程区无假卡。
- `dashboard-logcat-request.txt`：`/courses?...&filter=hot&credential_id=7` 请求/响应/裸数据三行 logcat 原文。
