## ①a 合并后重取（回执）

合并后复盘发现上一轮证据有两处缺口，本次补齐，并把绑定对象换成**合并后的 master**：

1. **缺仓标准的「机检行」** —— 上一轮没走仓里 `scripts/device-capture.ps1` 这条只读载体。
2. **入库成品没验过** —— 上一轮只 OCR 了**源帧**，没 OCR 入库后的 JPEG（q75/720 压缩缩放后可能不可读）。

**绑定对象**：树 = `e29e203a`（合并后的 master，即真正发布的那棵树）；设备 = `f0bae674`（2510DRK44C / annibale）。
**前提复测**（本次现测，非引用旧结论）：`GET /api/ai-assistant/models` → `data: []`；`GET /api/ai-assistant/modes` → `{normal:null,expert:null}` ⇒「平台模型为空」前提成立、守卫可复现。

### 机检行（仓载体原文）

```
DEVICE_CAPTURE_RESULT=PASS
  页面 pages/ai-assistant/ai-assistant=SKIP  前台=io.dcloud.uniappx/io.dcloud.uniapp.appframe.activity.UniPortraitPageActivity  logcat窗口行数=318  FATAL=0  ANRin包=0
```

**如实读这条 PASS**：它只覆盖只读四断言（前台 Activity 可取 + 截图非空 + 无 `FATAL EXCEPTION` + 无 `ANR in`）；**页面项是 `SKIP` 而不是 `PASS`**（`PASS=0 FAIL=0 SKIP=1`）—— `device-capture.ps1` 的切页分支默认关闭、ADR-0008 明记该分支不工作，脚本刻意不把它做成假绿。该结果已由脚本自身贴成上一条 prefilter 评论。

### 文本判据（截图与文本成对取，ADR-0008 ①a 补遗 6）

`03` 的 OCR 逐字读出：`平台模型未配置，请联系管理员；专业版可` / `在「对话设置」中配置自定义模型` —— 与源码字符串（34 字）**逐字一致**：两行完整渲染、分号正确、末字「型」在屏。`04`/`05` 是**同一页同一态**但**不含**这两行的对照。

**三张图都对压缩成品复跑过 OCR**（补上「只验源帧、未验入库成品」的缺口）。判据工具是**本地** Windows OCR（`Windows.Media.Ocr` / `zh-Hans-CN`，无外部配额）；本会话可视通道配额已用尽，且 `uiautomator dump` 方法上抓不到 toast（启动 2.5–3.3 秒 > toast 的 1.5 秒寿命，**不是**「toast 不在 a11y 树里」的结论）。

### 产物

3 张图随 **PR #1072** 入库到本 PR 的证据目录（同一压缩规格：JPEG q75 / 宽 720 / 单张 ≤150KB）：

- `docs/verification/ai-assistant/1069/03-ai-assistant-empty-model-toast-after.jpg`（toast 在屏）
- `docs/verification/ai-assistant/1069/04-ai-assistant-toast-dismissed-after.jpg`（对照）
- `docs/verification/ai-assistant/1069/05-ai-assistant-page-carrier-current.jpg`（仓载体只读页面图）

### 一条与工具链有关的实况（非缺陷）

`device-capture.ps1` 的入库步骤**主动跳过**了，日志原文：

```
入库：当前分支 chore/1062-postmerge-evidence 不等于 PR head feat/mobile-ai-1062-empty-state，
跳过入库（有意为之：绝不把截图提交到别的分支）
```

PR #1069 已合并、其 head 分支已删 ⇒ 载体拒绝把截图提交到别的分支，这是**该守的行为**，我没有绕过它；截图改由 PR #1072 落库。
