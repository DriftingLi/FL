# Domain Docs

Engineering skills 探索 codebase 时，应如何消费这个 repo 的 domain documentation。

## Before exploring, read these

- repo 根目录的 **`CONTEXT.md`**，或
- repo 根目录的 **`CONTEXT-MAP.md`**（如果存在）— 它指向每个 context 的一个 `CONTEXT.md`。读取与当前话题相关的每个文件。
- **`docs/adr/`** — 读取与你即将处理区域相关的 ADRs。在 multi-context repos 中，也检查 `src/<context>/docs/adr/` 中的 context-scoped decisions。

如果这些文件不存在，**静默继续**。不要标记缺失；不要提前建议创建。`/domain-modeling` skill（经由 `/grill-with-docs` 和 `/improve-codebase-architecture` 调用）会在 terms 或 decisions 实际被解决时懒创建它们。

## File structure

Single-context repo（大多数 repos）：

```
/
├── CONTEXT.md
├── docs/adr/
│   ├── 0001-event-sourced-orders.md
│   └── 0002-postgres-for-write-model.md
└── src/
```

Multi-context repo（根目录存在 `CONTEXT-MAP.md`）：

```
/
├── CONTEXT-MAP.md
├── docs/adr/                          ← system-wide decisions
└── src/
    ├── ordering/
    │   ├── CONTEXT.md
    │   └── docs/adr/                  ← context-specific decisions
    └── billing/
        ├── CONTEXT.md
        └── docs/adr/
```

## 文档分层：持久 vs 临时（**不要删 ADR / CONTEXT.md**）

这个 repo 的文档分两层，**换任务时只清临时层**：

| 层 | 位置 | 生命周期 |
| --- | --- | --- |
| **持久**（**入库、只增不删**） | `CONTEXT.md`（领域词表）· `docs/adr/`（决策）· `docs/agents/`（工作约定） | 跨任务 / 跨会话 / 跨人；`.gitignore` 对这三者有显式例外（原文：「本地文档（不入库；**ADRs 例外：架构决策记录是核心资产，必须版本化**）」） |
| **临时**（**不入库、可随时删**） | `.scratch/`（handoff 文档、探针脚本、PR 正文草稿、日志） | 单次任务；`.gitignore` 整目录忽略 |

**过时的决策不要删，用「取代」表达**：新 ADR 在「状态 / 领域」行写明取代了哪一条（先例：`ADR-0033` 写明取代 `ADR-0032` 的「历史不持久化 sources」取舍）。删掉旧 ADR 会打断后续票对它的引用 —— 票面、契约测试与代码注释都在按编号引用 ADR。

**反例（不要做）**：任务完成后删除本轮产生的 `CONTEXT.md` / `docs/adr/` 条目来「避免上下文污染」。那不是污染源：真正的污染是**临时产物混进了持久层**，而持久层正是冷启动会话唯一必读的面（issue 评论不是）。要清的是 `.scratch/`。

## Use the glossary's vocabulary

当你的输出命名某个 domain concept 时（issue title、refactor proposal、hypothesis、test name），使用 `CONTEXT.md` 中定义的 term。不要漂移到 glossary 明确避免的 synonyms。

如果你需要的概念还不在 glossary 中，这是一个信号：要么你正在发明项目没有使用的语言（重新考虑），要么确实存在缺口（为 `/domain-modeling` 记录）。

## Flag ADR conflicts

如果你的输出与现有 ADR 矛盾，明确指出，而不是静默覆盖：

> _Contradicts ADR-0007 (event-sourced orders) — but worth reopening because…_
