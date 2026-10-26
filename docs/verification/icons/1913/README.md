# #1913 状态形变白名单 —— 证据

无头 Edge 156 对 **`vite build` 产物**取证（`vite preview` + puppeteer-core `--no-save`；
学员端用假令牌 + `/auth/me` 桩过守卫）。

| 文件 | 看什么 |
| --- | --- |
| `01-before.png` | 侧栏底部「收起侧栏」按钮（展开态）：图标是 `panel-left-close` |
| `02-mid-70ms.png` | 点击后 70ms —— 侧栏正在收、图标在换向 |
| `03-settled.png` | 结算态：图标成 `panel-left-open`，文字标签消失 |
| `morph-trace.json` | **主证据**：点击后 600ms 内逐帧采样 `.collapse-icon path` 的 `d` |

## 为什么主证据是 JSON 而不是两帧像素

侧栏收起时按钮本身在位移，固定裁剪框拍到的「中间帧」无法区分「图标在形变」与「元素在移动」。
所以直接采 `d`：

- 正常模式：600ms 内出现 **60 个不同形状** —— 确实是一帧一帧在长；
- `prefers-reduced-motion: reduce`：只有 **2 个形状**（源 → 目标，中间无过渡）—— 降级生效。

复现：`cd frontend && npm run build && npx vite preview --port 4175`，再用 puppeteer-core 驱动本机
Edge（脚本不入库，理由同 #1911/#1912：依赖本机浏览器路径与预览服务）。

## 没有被截图覆盖的点（如实记录）

- **收藏 / 举报 / 删除**（`UiActionChip`）：形变接线由 `ui-components.spec.ts` 的单测锁定
  （激活态名字切到 `chip-fav-filled`、`morph` 开关打开；心形 false），但**没有实拍** ——
  论坛/课程页需要真数据，桩数据下这些药丸不渲染。
- **上传结果**（`FileUpload`）：同理，需要真上传链路；接线是「同一个 `UiIcon` 换 name」，
  失败重试回 `loader-circle`。
- 这两处如果要实拍，得连真后端或写一份种子数据，属后续可补项。
