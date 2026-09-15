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

当某个 skill 提到 role（例如 "apply the AFK-ready triage label"）时，使用此表中对应的 label 字符串。

编辑右侧列，使其匹配你实际使用的 vocabulary。
