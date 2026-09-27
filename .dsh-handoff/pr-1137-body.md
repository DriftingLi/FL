## 背景

`#1137` 的根因是 `.gitattributes` 漏钉 `*.uts` / `*.uvue`。**同一个坑位在这个文件里已经被记过两次** —— `*.ps1`（第 25–28 行）与 `deploy/env.defaults`（第 7–10 行），理由都写在注释里：契约测试读**源文本**做多行锚点匹配，Windows 上 `core.autocrlf=true` 把文件检出成 CRLF ⇒ 锚点必然失配。`*.uts` / `*.uvue` 是第三处，此前没钉。

实测先例就是本票：`contributionStatusContract` 的「漏一个取值」用例在本机恒红。比假红更糟的是**静默失效** —— 锚点永不命中时，守护以为在守、其实没守。

## 改什么

`.gitattributes` 追加一行规则（+8 行含注释，无其它改动）：

```
*.uts text eol=lf
*.uvue text eol=lf
```

**无 renormalize 风险**：全仓 `git ls-files --eol` 里 **0 个** `i/crlf` / `i/mixed`，且无 `* text=auto` ⇒ 本钉只改检出行为，**不改动任何已提交内容**（这一点在 #1137 的 triage 里已实证，本次复核仍成立）。

## 验收证据

免（未命中运行时面）—— 改动集只有根 `.gitattributes`（`git show --stat` = 1 file changed, 8 insertions(+)，deletion 0），不含 `*.uvue` / `*.uts` / training-app 三份 json。

判据是「**它要达成的那个行为在真实链路上成立**」，故用真实仓库 + 最小对照取证，不靠断言：

**① 真实仓库（`.gitattributes` 已带本改动，`core.autocrlf=true` 模拟 Windows 检出）**

| 步骤 | 读数 |
| --- | --- |
| 改前基线（worktree 现状，无本钉） | **203** 个 `*.uts`/`*.uvue` 全为 `i/lf **w/crlf**`，`attr/`（无规则） |
| 置 `core.autocrlf true` + 删除 `training-app` 后重新检出 | 203 个全为 `i/lf **w/lf** attr/text eol=lf` |
| 真实字节（`utils/format.uts`） | `CR=0  LF=135` |
| `git status --porcelain` | **仅** `M .gitattributes` —— 零幻影 diff（真实换行差异本应报 203 个文件 modified） |
| `git check-attr text eol` | `text: set` / `eol: lf` |
| 事后复原 | `git config --unset core.autocrlf`（只改本 worktree 的 `.git/config`，未动主树与其它 worktree） |

**② 最小隔离仓库对照**（排除大仓噪声；先证「没规则会坏」再证「有规则会好」）

| 场景 | `format.uts` | `thing.uvue` | `README.md`（阳性对照） |
| --- | --- | --- | --- |
| A 无 `*.uts`/`*.uvue` 规则（= 当前主干） | CR=3 **CRLF** | CR=3 **CRLF** | CR=2 CRLF |
| B 有规则（= 本 PR） | **CR=0 LF ✅** | **CR=0 LF ✅** | **CR=0 LF ✅** |

B 场景里 `README.md`（`*.md` 一直在钉）同样翻成 LF ⇒ 证明这套机制确实在跑、对照组有效；A 场景证明旧行为确实会产出 CRLF。

## 关联

- 承载 issue：#1137
- 未改动 ①②③④ 门语义与 `pr-evidence` 校验逻辑
- 本改动**不**影响 CI 的 `mobile-test`（`ubuntu-latest` 上 `core.autocrlf` 默认关，本来就不出 CRLF；该门对 Windows-only 破坏的盲区是另一件事，本 PR 不触及）
