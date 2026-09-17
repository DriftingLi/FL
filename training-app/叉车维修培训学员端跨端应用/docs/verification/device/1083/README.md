# #1083 真机取证（①a）—— 部分完成，明写未主张项

> 采集时间：2026-09-17 · 被验树：`feat/mobile-1083-wrong-question-card` HEAD `c2064303`
> 设备：`f0bae674`（23049RAD8C / Android 15）· 应用：`io.dcloud.uniappx`（apps/`__UNI__1C1D180`）
> 部署判据：`HX_RUN mode=incremental compile=150 deploy=0 total=211 exit=ok`，
> 设备侧 `www` mtime `1789630063 → 1789631318`（相对基线前进）

## 收录物与各自能证明什么

| 文件 | 能证明 | **不能**证明 |
| --- | --- | --- |
| `before-profile.png` | 「我的」页头部为蓝色渐变（`#7EC6F7` 系），用作页身份对照基线 | — |
| `wrong-questions.png` | 目标页已到达：头部为纯白 + `#333333` 标题，与 `pages/profile/wrong-questions.uvue` 的 header 特征一致（截图为 `cli launch app-android --pagePath pages/profile/wrong-questions` 之后所摄） | 页内文字内容（本会话无法读屏，见下） |
| `card-dex-strings.txt` | 本次改动的文案确实在被验树里：从设备拉回的 `pages/profile/components/wrong-question-card/classes.dex` 解出 16 条中文串，含新增的 `最近` / `我上次选的答案：` / `正确答案：` / `解析：` / `暂无解析` / `（无作答记录）`，且 `record-question-img` class 名在其中 | 这些元素在屏幕上的**呈现是否正确** |

## 未主张事项（逐条）

本会话**无法读取屏幕文字**，故下列三项**不由本批产物主张**，需具备视觉能力的会话补做：

1. 带图题是否真的显示了题干配图、无图题是否零占位不塌陷；
2. 点「查看答案与解析」是否展开出三行（我上次选的答案 / 正确答案 / 解析）、再点是否收起；
3. 头部「最近 YYYY-MM-DD HH:mm」是否含时间值。

阻塞原因（现测）：视觉服务配额耗尽（`accounts that have not been recharged can only try 10 times`）；
本机 Windows OCR 在当前 shell 起不来（`Operation is not supported on this platform`）；
`uiautomator dump` 只有底部 tab bar 四个文本节点（uniapp-x 原生渲染不上无障碍树）；
应用自打的「进入页面:」行已不在设备 logcat 缓冲内。

**结论：本批不是 ①a 通过证据**，只证明「目标页已到 + 被验树上确有本票代码」。按仓库纪律，
不拿合法色带当「四项展示正确」，故本票在补齐上列三项前保持 `## 进度：90%`，不开 PR 主张验收。
