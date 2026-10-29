# 移动端 UI hint 例外（按需层）

常驻层「前端 UI 约定」只留指针，两条**不要清**的 hint 例外落在这里（2026-10-29 #2105 决议出局，落库 #2116）。

页面保持整洁的通则、Tailwind 共存四条规则（R1–R4）与「不得触碰的边界」的权威版本在根 [`ui-conventions.md`](../../../../docs/agents/ui-conventions.md)，本线不抄——那几条对移动端**不适用**（2026-10-03 现测：未引入 Tailwind、无 `src/`、无残值页面、无 `--color-brand-*` 消费点）。

清理 hint 时**跳过这两条例外（不要清）**：

1. 论坛发帖 / 回复输入区的**属地披露提示**——文案单点 `utils/forumDisplay.uts` 的 `FORUM_REGION_NOTICE`；属地这个事实只在点发布那一刻才产生，事前告知比事后解释便宜（根 ADR-0045）。
2. Markdown 档的**边界提示行**——同文件的 `FORUM_MARKDOWN_BOUNDARY_NOTICE`，**只讲边界、不列能力清单**（移动端 ADR-0025 ⑥-5），依据是根 ADR-0046「判据放在作者看得见的地方」。Web 侧同款叫 `FORUM_MARKDOWN_HINT`，**两端措辞不同是故意的**，别去「统一」它。

两条都由共享输入区组件 `pages/forum/components/forum-markdown-input.uvue` 渲染（发帖与回复同一份形态，ADR-0025 P3），样式类 `.region-notice` / `.boundary-notice`。**删 hint 时同步删掉它的 CSS class 与 scoped 规则**，别留死代码。
