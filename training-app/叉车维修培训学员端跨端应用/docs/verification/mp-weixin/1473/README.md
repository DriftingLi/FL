# ② 微信开发者工具取证 — #1473 回复半屏浮层（小程序端）

工具：微信开发者工具 Stable 2.02.2608070；工程 = `wt-1473/.../unpackage/dist/build/mp-weixin`（已逐项比对文件树确认为 #1473 构建，非主树）。
取证日期：2026-10-03。自动化探测（`mp-weixin-check`）因 `sdk-version-missing` 环境项未跑通，改由人工经 Computer Use 在开发者工具内亲眼取证（票面口径：小程序端结构上不能逐样式断言，证据形状 = 整页截图 + console 零报错）。

## 判据（⑪-1 / ⑪-2 的小程序端渲染）

| 文件 | 证明 | 读数 |
| --- | --- | --- |
| `01-detail-no-panel-baseline.png` | 对照基线 | 帖子详情未开浮层，底部常驻触发条 |
| `02-reply-panel-raised-mask.png` | ⑪-1/⑪-2 浮层渲染 | 点触发条 → 半屏浮层从底升起：纯文本/Markdown tab + 输入框 + IP 提示 + 图片＋ + 收起/发送；上方内容被半透明遮罩压暗；路由 `pages/forum/forum-detail`；console 计数 `0 error / 18 warning` |
| `03-console-errors-only-clean.png` | console 零报错 | Errors-only 过滤无红色 error（仅一条 devtools 自身「不校验合法域名」分组标题，非 error） |

## 诚实边界

- 桌面模拟器无真实软键盘，「面板贴键盘上沿、标题栏未被顶走」的像素判据归 **Android ①a**（见 `docs/verification/forum/1473/`）；本端只验浮层**渲染 + 零报错**，与票面「小程序端不许诺机检面板位置」一致。
- 取证过程未提交回复、未点上传/发布、未改代码/配置。
