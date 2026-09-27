已随 **PR #1069** 合并（squash → master `e29e203a`，2026-09-16）收口。

**交付**：`pages/ai-assistant/ai-assistant.uvue` 的空态文案改为可执行引导 —— 「平台模型未配置，请联系管理员；专业版可在「对话设置」中配置自定义模型」；新增守护 `utils/aiEmptyStateActionableContract.test.js`（5 例，含字数上限与变异自检）；①a 入仓截图 `docs/verification/ai-assistant/1069/`。

**门**：③ 79 suites / 1284 tests 全绿 · ④c `KOTLIN_ALL_RESULT errors=0 classes=1300 files=98` · ①a 真机（`f0bae674`）出证、**①b 未触发**（`CAPABILITY_SURFACE touched=no`，纯文案改动未命中能力面）。PR 上 `pr-evidence` / `mobile-test` / `ci-summary` 均绿。

**两条与票面不同的事实**（已写进正文「校正」段）：

1. 票面「这三处文案被 3 个契约测试锁着」**不成立** —— 实测没有任何测试锁这两句，票面引的 `aiDiagnosisSourcesContract.test.js` 在 master 上不存在（它是未合并 PR #1046 的新增文件）。故「同步 3 处契约测试」无从同步，改为**新增**一条守护。
2. 票面给的默认句有两处不成立：真实入口名是**「对话设置」**（不是「设置」）；自定义模型的入口与配置面**都在专业版门内** ⇒ 只写「去配置自定义模型」对**未解锁学员**是死路一条（正是本票要防的缺陷）。按维护者裁定改用「先给联系管理员、再说专业版路径」。

**一条真机实测发现**（①a 的额外收获）：`uni.showToast` 的文本区是**两行 × 18 字**上限 —— 首版 37 字的文案，末字「型」不渲染（OCR 三帧一致）；压到 34 字后复测逐字完整。该上限已写进代码注释并落成守护（`≤36`）。
