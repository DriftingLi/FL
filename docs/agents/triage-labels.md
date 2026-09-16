# Triage Labels

Skills 使用五个 canonical triage roles。这个文件把这些 roles 映射到此 repo issue tracker 中实际使用的 label 字符串。

| Label in mattpocock/skills | Label in our tracker | Meaning                                  |
| -------------------------- | -------------------- | ---------------------------------------- |
| `needs-triage`             | `needs-triage`       | Maintainer 需要评估这个 issue            |
| `needs-info`               | `needs-info`         | 等待 reporter 提供更多信息               |
| `ready-for-agent`          | `ready-for-agent`（后端 / 前端票）· **`ready-for-mobile-agent`（移动端票）** | 已完整说明，准备交给 AFK agent 接手      |
| `ready-for-human`          | `ready-for-human`    | 需要人工实现                             |
| `wontfix`                  | `wontfix`            | 不会处理                                 |

**移动端变体**：`ready-for-agent` 这一个 role 在本仓有**两套 label 字符串**，按票所属技术栈取用 ——
`training-app/叉车维修培训学员端跨端应用`（uni-app-x 移动端）的票用 **`ready-for-mobile-agent`**
（先例 #926 / #920 / #939 / #971 / #988 / #998 / #999），其余用 `ready-for-agent`。
两个 label 在 tracker 里同时存在，取用其一，不要同时挂。

## 重复票的处置（canonical 五角色里**没有** duplicate）

**不新增第 6 个 label** —— 重复用**动作**表达，两件都做：

1. `gh issue close <n> --reason "not planned"`；
2. 一条**指向原票**的评论，写清「**哪一半**被吸收、**吸收到哪张票**」（先例 **#1045**：其客户端命题随 PR #1059 在专项页结构性消解 ⇒ 并入 #1044、客户端空态残部指向 #1062）。

**不要为「关票时摘标签」立规矩**：本仓**关闭票里仍挂着 `ready-for-*` 的有 267 张**（几乎是全部历史）—— 那是既成惯例；而关闭的票本就「无歧义地离开 frontier」（frontier 查询一律 `--state open`）⇒ 标签在关闭票上是**惰性**的，摘它既无收益、又要动 267 张票。**标签保持原样，不要流转。**

**机检口径**：重复票 = `state=CLOSED` + `stateReason=NOT_PLANNED` + 评论含 `#<原票号>`。**不要**靠标题里的「（重复）」字样判断，也**不要**靠标签（本仓不摘）。

**开票前先查承载面**：重复票多半不是「想错了」，而是**没查就开了** —— #1045 出生即死，就是开票时没看 #1059 正在改同一个文件的同一段。开票/认领前先看有没有票或 PR 已经在动同一个文件或同一个判定面；有，就**等**（挂 blocking）或**并入**，不要并行开第二张。

当某个 skill 提到 role（例如 "apply the AFK-ready triage label"）时，使用此表中对应的 label 字符串。

编辑右侧列，使其匹配你实际使用的 vocabulary。
