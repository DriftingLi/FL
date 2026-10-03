# ①a 真机取证 — #1472 论坛行内渲染

设备：Redmi（arm64-v8a，HBuilderX 5.26 基座），App-Android，登录生产后端。
取证日期：2026-10-02。取证会话产物原址 `.scratch/1472-4a/`，本目录为入库副本。

## 截图对应的验收判据

| 文件 | 证明的 AC | 读数 |
| --- | --- | --- |
| `01-forum-detail-render.png` | ⑩-1 成对记号不再原样显示 · ⑩-3 零形状 · ⑩-4 有序编号从 1. 起 | 标题/无序列表/有序列表（`1.` `2.`）/代码块 `print("hello world")` 等宽着色，无 `#`/`-`/`` ` `` 泄漏 |
| `01-forum-detail-ui-dump.xml` | ⑩-1 文本判据（uiautomator 未压缩） | `content-desc` 中 `**` / `~~` / 反引号 计数 = **0**（成对记号已被渲染消费） |
| `02-detail-scroll-inline-code.png` | ⑩-5 行内代码 · C3 字符空格胶囊 | 行内代码胶囊前后留空格，未吞相邻字符 |
| `03-detail-scroll-headings.png` | 分级标题渲染 | 一级/多级标题层级正确 |
| `04-link-modal-confirm.png` | ⑩-8 协议闸（白名单 + 确认弹窗） · ⑩-9 站内链接也弹确认 · 分词器递归 | 正文 `see **bold** and [manual](…) end` 内联渲染（粗体+链接同现），点 `manual` 弹「即将离开应用访问：gccsmile.com」取消/打开 |
| `04-link-modal-ui-dump.xml` | ⑩-8 | 弹窗为原生 `showModal`，文本不入 a11y 树属预期；以截图为准 |
| `05-after-open-browser.png` | ⑩-8 App 分支（原生 Intent 拉起系统浏览器） | 「打开」后 `com.android.browser` 落地 gccsmile 公司页 |
| `06-after-delete-testpost.png` | 取证卫生 | 取证用测试帖已删除，生产无残留 |

## 诚实边界

- **mp-weixin 端（② 门）论坛页链接点击未在此目录取证**：本环境 `miniprogram-automator` 逐页导航不可用（恒报 `Uncaught [object Object]`），② 门仅覆盖到 App 冷启动落地页干净（见 `docs/verification/mp-weixin/1472/`），**论坛页 markdown 帖的渲染/点击那一格待人工在微信开发者工具内手动确认**。
