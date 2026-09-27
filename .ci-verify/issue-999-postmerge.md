## #999 合入后真机复核（2026-09-15 16:15，接手会话补做）

本票关闭时唯一未做的一项（设备 `f0bae674` 当时从 USB 掉线）。设备恢复在线后补做。

**构建**：合并后的主线 `cc11466a`（本票修复所在的那条主线）+ **两行未提交的 log-only 探针**
（`console.log('[#999-probe] …')`，仅输出日志、不改任何逻辑；取证后已撤回，工作树与 `cc11466a` 零差异）。

**实测（logcat）**：点首屏宫格第 1 格「故障咨询」（`fault_diagnosis`，`freePreview` **限免、不扣积分**）：

```
[#999-probe] doStreamChat featureKey=[fault_diagnosis]     ← 键非空（修复前此处恒为空串）
sse event=message dataLen=34 / 102 / 151 / 155 / 187 / 73 / 121 / 32   （共 8 片）
sse event=sources dataLen=9968                              ← 诊断适配器专属事件
sse event=done    dataLen=4
```

**结论**

1. **本票的修复在合并后的主线上成立**：`featureKey` 送达且非空（`fault_diagnosis`），不再是空串。
2. **诊断通道确实被走到**：收到 `sources`（9968 字节）。
3. ⚠️ **关于「看 sources」这条判据本身的一处更正（重要，供后来者）**：后端发射点带守卫 ——
   `if sources := service.DiagnosisSourcesFrom(ctx); len(sources) > 0 { sendEvent("sources", …) }`
   （`backend/internal/api/ai_assistant.go`）。即 **`sources` 只在 RAG 真的回了 `answer_sources` 时才发**
   ⇒ 它是**充分条件，不是必要条件**。本次**前 3 次**点击（同一格、同一 prompt）都只收到 `message…done`、
   **没有** `sources`，而那些轮次的回答是「暂无法给出可靠维修方案（缺车型/故障码）」类内容 —— RAG 未命中即不发来源。
   **所以「看不到 sources」不能推断键没生效**；后续复核请以「请求体 `feature_key` 非空」为必要条件，
   或按「多试几轮 + 看回答形态（诊断 SOP 模板 vs 通用助手口吻）」来判。

日志留存（本机，未跟踪）：`.tmp-921-dev/probe-999-after2.log`（探针行可用 `sse event=` / `doStreamChat featureKey` 检索）。
