# AI 回答：句内图升块 + 气泡走行内 runs（顺带修记号外露）

- 状态：已采纳（2026-11-09，#2366 实施同批写入；修订 ADR-0056 决定 1 与 2）

## 背景

ADR-0056 把句内图交给 `stripInlinePlain` 退化 alt、气泡直出块文本，留下两处缺口：句内图在手机上不可见（票面要求升块），气泡文字不经 runs（论坛 / 精选已于 #2356 走 runs）。直出还有一个既有缺陷：三处记号会以外露字符收尾 —— `**注意 *务必* 戴手套**` 留 `**`、`***务必***` 留 `*`、多反引号行内码留 `` ` ``。

## 决定

1. 句内图升块落在**壳层**（`utils/aiAnswerBody.uts#splitInlineImageParagraphs`），不进解析器：解析器切分会让章节档（28 篇）与精选档同步改形，破「块数组逐字不变」的 AC。按图切段 `前文 ![图](u) 后文` ⇒ paragraph / image / paragraph；同段多图按序各成一块，不合并。
2. 可切分判据 = 分词器自证：每段非图文本过 `splitInlineRuns` + `inlineRunsPlainText`，按序拼回 alt 得 `re`；整行同法得 `whole`；`re == whole` 才切。不另立第二份记号正则（两表必漂移，ADR-0056 已有同源判据先例）。
3. 边界：alt 为空不切；一行内任一图不可切则整行不切（逐行取交集）；跨图记号（粗体 / 链接 / 行内码 / 删除线 / 斜体各一例）拼回必不等 ⇒ 五格都不切，并把图**折回 alt**（`foldImagesToAlt`）而不是留着标记交分词器 —— 链接与行内码两格会把包着图的整段读成 URL，那是比 alt 更糟的外露（fail-closed 优先于票面 prose 的「安全」标注）。
4. `SUBSET_MEMBERS_AI_ANSWER` 增 `MEMBER_INLINE`，气泡传 `inline-axis="plain"` ⇒ 正文文字经 nested runs 出，与论坛 / 精选同刻度。连带能力面变化：AI 面有序列表 `level` 0→1（`LIST_LEVEL_OL`），手机上 1./2. 由「•」变编号。
5. 分词器三处残留同批修：`***x***` 走 bold+italic；`**…**` 改手动扫描，让内层单 `*` 成为斜体、不再提前结束；行内码按反引号串长配对（``a`b`` 合法）。乘号 `2 * 3`、词内下划线 `a_b_c` 由既有 flanking 规则保持。

## 后果

- 一句一图由 1 块 / 0rpx 变 3 块 / 48rpx（`mt-24`×2，与「整行图独立成块」同几何），两图 5 块 / 96rpx；站内闸不变：所有 image 块（含句内）统一经 `resolveFileUrl` + `SITE_IMAGE_ABS_PREFIX`，站外退化 alt。
- 契约镜像（`markdownContract.test.js` 的 AI 声明表）、`SHELL_IMPORTS`、`markdownSubsetBehavior` 的「AI 档不声明 inline」断言同批改（`markdownToolbarBehavior` 不受影响）；ADR-0056 后果段的「属后续（另票）」挂账随之兑付。

## 不采用的备选

- 解析器内切分 / 给 `items` 塞图 URL / 用第二份记号正则判安全：前者让章节与精选块数组一起改形（见决定 1），中者拆语义且只改 runs 时句内图仍不可见，后者与分词器输出两表必漂移。
