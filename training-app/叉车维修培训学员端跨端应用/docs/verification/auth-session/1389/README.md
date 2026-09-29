# #1389 · uni-app x App 端容器的 cookie jar 取证（ADR-0030 ④-4）

**结论（实测，判据成立）**：**不带。** uni-app x 的 App-Android 容器在本次环境下
**不维持 cookie jar、后续请求不自动附带 `Cookie:`**——因此移动端实际走的是**请求体通道**。

这条**推翻** ADR-0030 `①` 那条公理（「App、H5 端会自动带上 cookie」）。那句出自
**uni-app（vue 版）** 的 uni.request 参数表，uni-app x 自己的表里**没有它**；
现在真机读数把它判掉了。回写见 issue #1389（本产物只出证，不改 ADR 正文）。

## 判据与读数

| 步 | 请求 | 服务端回显（= 它收到的 Cookie 头） | 读法 |
| --- | --- | --- | --- |
| step1 | `GET https://httpbin.org/cookies/set?probe=jar1389`（302 → `/cookies`） | `{"cookies":{}}` | 连**重定向那一跳**都没带上刚下发的 cookie |
| step2 | `GET https://httpbin.org/cookies`（**独立第二次请求**） | `{"cookies":{}}` | 跨请求同样为空 ⇒ jar 不成立 |

- 原始读数：`01-probe-readings.txt`（logcat 原样，未改写；两条均来自 `App.uvue` 探针）
- 对照组：`02-control-pc.txt` —— 同一 URL、同一 302 链，在带 cookie 会话的客户端上回显
  `{"probe":"jar1389"}` ⇒ 探针形状有效、`Set-Cookie` 的路径匹配成立 ⇒ 真机那两个空对象**不是假阴性**。

## 环境与可复现性

- 设备 `23049RAD8C`（marble）· `arm64-v8a` · Android 15 / SDK 35 · 无线 adb
- 运行时 HBuilderX `5.23.2026080626` 标准基座 `io.dcloud.uniappx`，增量部署（`pid 30176 → 3611`）
- 探针在 `App.uvue` 的 `onLaunch`，**冷启动自动触发**：无 UI 交互、无事件注入
  （本机型的 `INJECT_EVENTS` 被拒，见 `scripts/device-capture.ps1` 头注）
- 复现：检出本分支（`prototype/1389-cookie-jar`）→ `scripts/hx-run.ps1 -Project <本目录> -Device <serial>`
  → `adb -s <serial> shell logcat -d -v epoch | grep probe1389`

## 残留（本产物不覆盖的部分，如实标注）

1. **单机型、单运行时版本**。机制在 DCloud 运行时（基座 DEX 里 `uts/sdk/modules/DCloudUniNetwork`
   含 `CookieInterceptor` 类，但静态看不出它是否被装进 `uni.request` 的那个 client），
   不像厂商 ROM 行为 ⇒ 跨机型复发概率低，但**未在第二台真机复算**。加固项：在另一台 arm 真机
   跑同一探针（非阻塞）。
2. **未测 H5 面**。H5 端就是浏览器，cookie 语义与本结论无关；本项目若将来发布 H5，需另判。
3. **未测生产链路上 `www.gccsmile.com` 那枚 httpOnly refresh**（探针走的是公共 echo 服务）。
   本结论判的是「容器会不会带 cookie」这件**通用**事，不是「那枚特定 cookie 是否匹配 Domain/Path」；
   前者为假 ⇒ 后者无从成立。
