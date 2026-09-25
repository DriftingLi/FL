# #652 ①a · 搜索页取证（术前↔术后）

- **PR**：#1327 · **issue**：#652 · **日期**：2026-09-25 · **执行人**：agent 执行
- 方法、树身份、两轮机检行与完整对照表见 [`../jobs/1327/README.md`](../jobs/1327/README.md)（同一驱动、同一轮次产出）。

| 页面 | 节点数 | a11y 内容 + 逐节点 bounds | 像素差（`png-diff.mjs --ignore-top-rows 100`） |
|---|---|---|---|
| `pages/search/search` | 37 / 37 | 逐字节一致（两边 sha256 同为 `0AC7B24A438F6298`） | `different=0 ratio=0 changed=false` |

取证动作：深链进入 → 注入关键字 `abc`（生效判据 = 输入框 a11y `text=` 实测等于 `abc`）→ 回车 →
`GET /api/search?keyword=abc&credential_id=1` 返回 **200**，即收紧后的 `searchAllApi` 出口与
`buildSearchResult` 映射在真机上跑通；两轮 `fatal=0 anr=0`。
