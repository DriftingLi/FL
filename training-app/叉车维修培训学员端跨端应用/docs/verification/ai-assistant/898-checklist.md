# PR #898 AI 助手 UI 对齐 · 真机验证人工步骤清单（①b）

> **状态：agent 为 PR #898 备料；下面的步骤尚未被走过一遍，结论栏不得由 agent 代填。**
> 口径见 `docs/adr/0008-移动端验收门与证据.md`「①a 真机自动取证」。
>
> - **①a**（agent 自动）= 逐页截图 + logcat 断言 + 前台 Activity —— `scripts/device-capture.ps1`
> - **①b**（**人**，就是本清单）= 在真机上逐页走查、与原型对比
>
> **本 PR 不含**生物识别 / 运行时权限弹窗 / 真机上传路径改动，故 ①b 的那三类专项**不适用** ——
> ①b 在本 PR 收缩为「**逐页走查 + 与原型对比**」。

## 你只需做的 4 步

| # | 你要做的 | 具体动作 | 期望看到 |
| --- | --- | --- | --- |
| 1 | 把 **PR #898 分支**跑到真机 | 分支 `feat/ai-basic-ui-prototype`；目标设备选已无线连接的这台（`192.168.1.26:46701`，model `2510DRK44C`）。可让 agent 跑 `npm run hx:run`（增量、不重装基座），或用 HBuilderX 直接「运行到手机」 | 应用装上并进到首页 |
| 2 | 进 AI 学 Tab | 点底部「**AI学**」 | 进到 AI 助手页，标题「基础版✨ / 维修助手」 |
| 3 | 逐页走查并与原型对比 | ① AI 助手空态（`ai-basic`）② 切「专业版」页 ③ 版本选择页 ④ 对话设置 ⑤ 自定义模型 | 见下「逐页看什么」 |
| 4 | 把结论填进 PR 正文 ① 行 | 四个字段：`执行人（你的 GitHub handle） / 日期（YYYY-MM-DD） / 复测对象（页面清单） / 结论（含产物）` | `pr-evidence` 由红转绿（它现在**只**红在 ① 行四个字段为空） |

## 逐页看什么（本 PR 的 4 处修复要复验）

| 页面 | 复验点 |
| --- | --- |
| `pages/ai-assistant/ai-basic` | ① **背景是浅蓝**（不是纯白）—— 修复前真机渲染成纯白且编译零 warning ② 功能宫格 **5 个图标**齐全、标签**不折行**（「故障代码查询」原会折成两行）③ 「习题解答」图标是**便签+铅笔**（修复前是红色碎片）④ 底部左侧模型芯片显示「**未配置模型**」（只读、不可点；本环境未配 admin 模型，故非「qwen3.7-max」）⑤ 右上角只有朴素 `⋯`，**无白色胶囊** |
| `pages/ai-assistant/ai-assistant`（专业版） | 头部标题为「专业版」（无 ✨）、背景同浅蓝、宫格同为 5 项；模型芯片**可点**并弹出模型选择器（含描述行） |
| `pages/ai-assistant/version-select` | 未改动；确认版本卡片与「使用基础版」按钮正常 |
| `pages/ai-assistant/ai-settings` | 未改动；确认可进入、无白屏 |
| `pages/ai-assistant/custom-models` | 未改动；确认可进入、无白屏 |

## agent 会自动采的项（不用你动手）

```powershell
# 只读取证：不 kill-server / 不 install / 不清 logcat / 不注入输入事件
pwsh -NoProfile -File scripts/device-capture.ps1 -Device 192.168.1.26:46701 -Pages "pages/ai-assistant/ai-basic"
```

| 自动采的项 | 说明 |
| --- | --- |
| 当前前台页截图 | `adb exec-out screencap -p`（只读） |
| logcat 断言 | 窗口内 `FATAL EXCEPTION` / `ANR in <前台包名>` 计数；窗口起点取**设备时钟**，**不清** logcat 缓冲 |
| 前台 Activity | `dumpsys activity activities` 的 `mResumedActivity` / `topResumedActivity` 原始行 |

## ⚠️ 逐页自动截图目前**用不了**（照实说）

本 PR 期间首次真机实测了 `-AllowAppStart` 的切页分支，结论是**不生效**：

- `am start -n <launcher> -a VIEW -d uniapp://<page> --ez dcloud_open_url true --es dcloud_page <page>` **未让 App 换页**
- 连跑 5 页，`*-after.png` 的 **SHA256 完全相同**（实拍为同一页）
- 原断言（前台/字节数/FATAL/ANR）**覆盖不到「目标页是否真的加载」** ⇒ 当时五页仍各判 `PASS`（静默假阳性）

已处置（本次一并提交）：脚本补「页身份」fail-closed 断言 —— 多页截图哈希相同即判相关页 `FAIL`。
实测复现：同一设备同一次运行由 `DEVICE_CAPTURE_RESULT=PASS` 变为 `FAIL`（`PASS=0 FAIL=3`，exit 1）。

**但这只让失效「响亮地失败」，不等于逐页取证已可用。** 已知可用替代是 HBuilderX 的
`cli launch app-android --pagePath <页>`（实测能进目标页），是否为此更换切页机制**待维护者裁定**。
⇒ **换成机制落地前，第 3 步（逐页走查）只能由人在真机上手动走。**

## 三条注意

1. **必须显式 `-Device`**：这台真机同时以 `192.168.1.26:46701` 与 `adb-f0bae674-KYrkiL._adb-tls-connect._tcp` 两条 transport 出现，脚本在「多设备」时 `exit 2`、不自动猜。
2. **`b18a461`（芯片文案改「未配置模型」）在写本清单时尚未部署到真机** —— 走第 3 步前请先部署分支 head，否则验的是 `56d0fe3`。
3. **签名由你填**：`pr-evidence` 目前**只**红在 ① 行的四个字段为空；你填完即转绿。agent 不得代填、不得写「已通过」；合并也由你执行（`gh pr merge 898 --squash --delete-branch`）。

## 本次未实测的部分（照实说）

- **换切页机制**：未做（待裁定），逐页截图仍不可用。
- **①b 的结论**：没有 —— 本 PR 不含指纹/权限弹窗/上传路径改动，但仍需人走一遍逐页对比。
- **`b18a461` 的真机表现**：未验（未部署）。