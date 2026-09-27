## 改了什么 / 为什么改

关联 #1062 / PR #1069（已合并）。本 PR **只入库 3 张 ①a 取证截图**，不改任何代码。

起因：PR #1069 的证据段只交了「截图 + 自造脚本的文本判据」，复盘时发现两个缺口，本 PR 补齐：

1. **缺仓标准的「机检行」**。ADR-0016 要求 ①a = 「入仓截图 **+ 机检行**」，而上一轮没走仓里 `scripts/device-capture.ps1` 这条只读载体。本次补跑，机检行如下（原文）：

   ```
   [device-capture] DEVICE_CAPTURE_RESULT=PASS
   [device-capture]   页面 pages/ai-assistant/ai-assistant=SKIP  前台=io.dcloud.uniappx/io.dcloud.uniapp.appframe.activity.UniPortraitPageActivity  logcat窗口行数=318  FATAL=0  ANRin包=0
   ```

   **如实读这条 PASS**：它只覆盖只读四断言（前台 Activity 可取 + 截图非空 + 无 `FATAL EXCEPTION` + 无 `ANR in`）；**页面项是 `SKIP` 而不是 `PASS`**（`PASS=0 FAIL=0 SKIP=1`）—— 因为 `device-capture.ps1` 的切页分支默认关闭且 ADR-0008 明记该分支不工作，脚本刻意不把它做成假绿。该结果已由脚本自身贴成 #1069 的 prefilter 评论（`<!-- prefilter:device-capture -->`，刻意不用验收门那套标记）。
2. **入库成品没验过**。上一轮只 OCR 了**源帧**，**没有 OCR 入库后的 JPEG**（q75/720 压缩+缩放后可能不可读）。本次三张图**都对压缩成品复跑 OCR** 复核可读。

## 重取的绑定对象

- **树**：合并后的 `master`（`e29e203a`）—— 即真正发布的那棵树，而不是分支 head。为此新开 worktree 后跑 `npm run hx:run` 把 master 构建部署到设备。
- **设备**：`f0bae674`（2510DRK44C / annibale）。
- **前提复测**：生产端点本次现测仍为空 ⇒「平台模型为空」前提成立，守卫可复现：
  `GET /api/ai-assistant/models` → `data: []`；`GET /api/ai-assistant/modes` → `{normal:null,expert:null}`。

## 产物（`docs/verification/ai-assistant/1069/`）

| 文件 | 内容 | 字节 |
| --- | --- | --- |
| `03-ai-assistant-empty-model-toast-after.jpg` | 守卫 toast 在屏（master 构建；会话式抓帧） | 71,940 |
| `04-ai-assistant-toast-dismissed-after.jpg` | **同一页同一态**、toast 已消失（对照帧） | 63,098 |
| `05-ai-assistant-page-carrier-current.jpg` | `device-capture.ps1`（仓载体、只读）对当前前台采的页面图 | 62,953 |

三张均为 JPEG q75 / 宽 720（本机无 webp 编码器，按 ADR-0008 退路；单张 ≤150KB）。

## 文本判据（截图与文本成对取，ADR-0008 ①a 补遗 6）

`03` 的 OCR 逐字读出：

```
平台模型未配置，请联系管理员；专业版可
在「对话设置」中配置自定义模型
```

与源码字符串 `平台模型未配置，请联系管理员；专业版可在「对话设置」中配置自定义模型`（34 字）逐字一致 —— 两行完整渲染、分号正确、末字「型」在屏。`04` / `05` 同页但**不含**这两行（对照成立）。

判据工具：**本地** Windows OCR（`Windows.Media.Ocr` / `zh-Hans-CN`，无外部配额）。本会话的可视通道（vision proxy / `vision_glance`）配额已用尽，故文本判据只能走本地 OCR；`uiautomator dump` 方法上抓不到 toast（实测启动耗时 2.5–3.3 秒 > toast 的 1.5 秒寿命，**不是**「toast 不在 a11y 树里」的结论）。

## 改动类型

- [ ] feat（新功能）
- [ ] fix（缺陷修复）
- [ ] refactor（重构）
- [ ] style（样式/格式）
- [x] docs（文档 / 取证产物）

## 影响范围 / 风险点

- **非运行时面**：改动集只有 3 个 `.jpg`（不含 `*.uvue` / `*.uts` / 三份 json）⇒ 按 ADR-0008 **免 ①②④**，`pr-evidence` 亦在运行时面判据处早退。本 PR 不改代码、不影响线上。
- 设备侧**无残留数据**：取证只做「点一个快捷提问行」这一步交互，守卫命中即早退（不发消息、不建会话）。
- 未勾「触及 agent 观测不到的能力面」（指纹 / 权限弹窗 / 真机上传 / 厂商 ROM 交互）—— 本次纯文案取证，未触及。

## 验收证据

免（未命中运行时面）。

## 自检清单

- [x] 提交信息遵循 Conventional Commits
- [x] 一个分支只做一件事，可独立 revert
- [x] 未把其他会话/模块的改动夹带进来（相对 `origin/master` 只有 3 个新文件）
- [x] 已关联对应 Issue（#1062 / PR #1069）
