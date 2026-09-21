# 多 Agent 并发与 git 隔离（可选）

> 多个会话/自动化工具同时操作仓库前必读；单人单会话可跳过。从 AGENTS.md 拆出（2026-09-09），内容为权威版本。
>
> **主树位置（2026-09-13 起）：`D:\FL`。** 原主树 `E:\FL` **已废弃 —— `E:` 盘现已不存在**（`Test-Path E:\` 实测 = `False`，2026-09-21），**别再把它当备胎去找**，它不再是工作树 —— 它所在卷是 **USB 外接 exFAT SSD**，只读 `chkdsk E:` 判 `exit=3`（**文件与目录 + 卷位图损坏，按维护者决定保留未修**，见文末「卷健康与新工作放置」），且实测从 E: 读取只有 **~1.1 MB/s**。
> 搬迁方式：`robocopy` 整树复制（**不是移动**，E: 保留为备胎），`0 FAILED / 0 Mismatch`；对账项 = HEAD `3127e88b` / 25 个本地分支 / 89 个改动 / 54 个未跟踪 / 12 条 stash **逐项一致**、`git fsck` 干净、抽样文件字节一致。落点相关 PR：#945。
> 判据（**每次现测，不按记忆走**）：`git -C D:\FL rev-parse --abbrev-ref HEAD` 能返回。（原判据的后半句「且 `E:\FL` 上没有活负载」**已作废** —— `E:` 盘不存在，2026-09-21 起没有备胎可查。）
>
> ⚠️ **本文件与 `handoff-验收门-2026-09-11.md` 里其余写死 `E:\FL` 的行，绝大多数是「2026-09-13 在 E: 上测得的**历史/现测事实**」**（exFAT 硬链接节、卷健康节、worktree 坏项的判据、重复副本读数、`E:\FL` 上的游离提交）—— **那些不要跟着改，改了就是篡改记录**。会误导人照做的只有「导航句」和「真实脚本里的绝对路径」，已单独改过。

多个 agent 会话（含 aider / 自动化监控）若**共用同一份 checkout 操作 git，会互相踩踏**：并发会话可能擅自 `checkout master` 并清掉分支，把另一会话刚 `commit`、尚未 `push` 的提交甩成无引用游离状态；`add`/`commit`/`push` 也会抢 `.git/index.lock`。

**单人单会话开发时不需要 worktree**，直接在主目录切分支即可。只有当多个 AI 会话/自动化工具同时操作同一个仓库时，才需要用 worktree 隔离：

- **一会话一 worktree 一分支**：动代码前先 `git worktree add ../wt-<task> -b feat/<task> origin/master`，全程在 `../wt-<task>` 内改、提交、push、开 PR；HEAD 与 index 天然隔离，避免并发 checkout/reset 互踩。

- **提交前验分支归属**：`git branch --show-current` + `git log --oneline origin/master..HEAD`，确认 HEAD 与待推提交属本会话任务，不在别的会话占用的分支上落 commit。

- **发现游离提交先取证、勿覆盖**：工作区被并发切走、自己的 commit 在 `git branch` 里消失时，用 `git log --oneline <sha>` + `git diff <sha> origin/master -- <paths>` 只读确认内容是否已在 master；已在则不 push、不 `reset`，交 gc 自然回收，绝不 `cherry-pick` 造冲突。

- **绝不** **`git add -A`**：共享工作区常混有其他会话/工具的未提交改动（如 `forum.uts`、`.aider-desk/`、`.monitor/`），只 `git add <本次文件>`。

- **共享文件的最小面：`pages.json` / `manifest.json` / `platformConfig.json`**
  （2026-09-13 补，血账）**这三个文件即使自己只改一行，也不要 `git add` / `git commit`。** 判据是
  `git status --short -- <路径>` **本身是否已是 ` M`**：是 ⇒ 别人正持着它，此时只改工作区、交付给维护者，或等它干净。
  理由有三条：① 它们**已被别的会话改着**时，`add` 会把别人的在飞改动一起提交（实测发生过）；
  ② 它们是「运行时面 / 打包面」的判据来源 —— 误提交会让 PR 凭空命中 ④b 云打包门（见 `docs/adr/0008`）；
  ③ HBuilderX 自己会往里写 `condition`（GUI 选的启动页，注释自述「仅开发期间生效」，属本地开发配置、禁止提交）。
  适用面比「加一行探针页」宽得多：凡是动这三个文件，一律走「改工作区 → 交给维护者」，不走「自己提交」。

  ⚠️ **判据的边界（2026-09-13 实测补）**：光看 ` M` 会**误判**。HEAD 落在**落后 `origin/master`** 的分支上时，凡是 master 前进过的文件都会显示成 ` M`，而**没人在改它**。实测当日：相对 HEAD 有 **89 个 ` M`**，其中只有 **36 个**真的与 `origin/master` 内容不同。
  所以判据要**两条同时成立**：`git status --short -- <路径>` 是 ` M` **且** `git diff --name-only origin/master -- <路径>` 非空。只看前者会挡住本该做的改动（当日因此一度放弃改一个字节与 `origin/master` 完全一致、根本没人持有的文件）。

- **提交前验分支归属的代价**（2026-09-13 补，血账）第 11 行已写明「查什么」；本条记的是**不查的代价**：主树被别的会话切走时，`git commit` 会把提交落进**别人的分支历史**，补救要「新建独立分支指向该提交 + 把别人的分支 `reset --mixed` 退回」——delta 为零，但过程不必要，且 `reset` 是改写别人历史的动作。实测：一条探针提交落进了别人的分支。

用完 worktree 后记得清理：`git worktree remove <dir>` + `git branch -D <branch>`。

## Windows 上用 worktree 的注意事项

Windows 本机（`E:\` 盘）上 worktree 可用，但有几处与 Linux 不同，照下面做：

- **一 worktree 一分支一会话**：`git worktree add D:\FL\wt-<task> -b feat/<task> origin/master`（也可放与主树同级的 `D:\wt-<task>`），全程在该目录内改、提交、push、开 PR；用完 `git worktree remove <目录>` + `git branch -D feat/<task>`。放在主树同级而非盘符根，便于一眼看清是哪个会话的目录。**目录名不要以 `.` 开头**——理由与判据见下一条。

- **⚠️ 建 worktree 走闸门，不要裸 `git worktree add`**（2026-09-19 加，#1185）：用
  `pwsh training-app/叉车维修培训学员端跨端应用/scripts/new-worktree.ps1 -Task <票号>`。
  它在**创建处**做两件裸命令做不到的事：① 参数校验（`-Task` 不得含路径分隔符）；② **建完立刻实测判据** ——
  在那个新目录里跑 `jest --listTests`，**必须真的列出套件**（计数 0 即失败并 `exit 3`，提示改名）。
  为什么必须实测而不是「校验目录名」：下一条的机制是**段首**为 `.` 或 `{}()+?.^$` 才踩坑，而默认命名
  `wt-<task>` 的段首恒为 `w` ⇒「枚举坏名字」在默认命名下是**恒不触发的死代码**；实测 `wt-(wip)1185` /
  `wt-{wip}1185` 的 `--listTests` 分别列出 **104 / 104** 个套件（都正常）。**建完实测**对任何命名方案都成立。
  它同时修掉一个我踩过的坑：仓库根从 `git rev-parse --git-common-dir` 取，**不**从脚本位置上溯 ——
  否则在 worktree 里调它会算出**那个 worktree**，把新 worktree 嵌进另一个 worktree 里（实测建出
  `D:\FL\wt-1185\wt-(wip)1185`）。

- **worktree 目录名不要以 `.` 开头**（2026-09-17 实测，血账；2026-09-18 补可机检判据 + 更正机制，见 #1144）：项目放在 `D:\FL\.wt-<task>` 时 `npm run test:unit` 报
  `No tests found … testMatch: … - 0 matches`（同一条命令在主树与 `D:\FL\wt-<task>` 下能列出全部 76 个套件；`git worktree move .wt-1082 wt-1082b` 后立刻恢复）。
  它**不报错、也不提示配置问题**，只是「一个测试都找不到」，很容易被读成「测试坏了」。⇒ 会话 worktree 用 `D:\FL\wt-<task>`（无点），**不要**用 `D:\FL\.wt-<task>`。
  **判据（可机检，在该 worktree 的项目目录里跑，`node_modules` 按下一条备好）**：`npx jest --config jest.config.unit.js -i --listTests | Measure-Object` ⇒ **计数必须非 0**；为 0 就是踩了本条。
  ⚠️ 它**静默**：点目录下 `--listTests` 空输出且 `exit 0`，只有带 `--testPathPattern` 真跑才报 `No tests found, exiting with code 1` ⇒ **别拿 exit code 当判据**。
  **对照数字（2026-09-18 实测，base `7e2a8efb`，同一份内容 / 同一命令 / 只换目录名）**：`D:\FL\wt-1144` **97** 条 vs `D:\FL\.wt-1144probe` **0** 条；绝对值随 HEAD 走（#1134 会话读到 主树 79 / `.wt-1134` 0 / 改名后 96），**「点目录恒 0」才是结论**。
  **机制更正（2026-09-18 定位到源码；旧记的「haste-map 爬取跳过点开头的目录」已被实测证伪）**：同目录把 `testMatch` 换成 `**/*.test.js` 立刻找到 **98** 个套件 ⇒ 文件全被扫到了，坏的是**绝对路径 glob 的匹配**：
  `testMatch` 的 `<rootDir>` 展开成 Windows 绝对路径后要过 `jest-util` 的 `replacePathSepForGlob`（= `path.replace(/\\(?![{}()+?.^$])/g, '/')`，jest 27.5.1）——它**故意不转换**后跟 `{}()+?.^$` 的反斜杠（怕吃掉 glob 元字符）⇒ **点段前的分隔符被留成 `\`**（`--showConfig` 实测 `D:/FL\.wt-1144probe/…`，非点目录是干净的 `D:/FL/wt-1144/…`），picomatch 把 `\.` 读成「转义的 `.`」、**分隔符随之消失** ⇒ 永远匹配不上真实路径。同理**段首是 `{}()+?.^$` 之一的目录名也会踩**（`(wip)` 链式实测同为 0 命中）。POSIX 下 `path.sep` 是 `/`、该函数是空操作 ⇒ **Windows-only 的坑**。
  现存点目录 worktree 还有 `.wt-1071` / `.wt-1087` / `.wt-1087b` / `.wt-1111` / `.wt-ai-feature` / `.wt-base` / `.wt-courses`（`git worktree list` 现测）⇒ 在里面跑 ③ 就是假绿/假红；**改名会牵动别会话正在用的 worktree，属破坏性操作，须协调后再做**（本票不做）。

- **`node_modules` 不要每个 worktree 重装**：目录联接（junction）共享主树那一份，省掉每个 worktree 2–5 分钟的 `npm ci`：
  ```
  cmd /c mklink /J <worktree>\training-app\叉车维修培训学员端跨端应用\node_modules <主树>\training-app\叉车维修培训学员端跨端应用\node_modules
  ```
  若确实需要孤立依赖（例如验证 lockfile 变更），才在 worktree 内 `npm ci --ignore-scripts`。

- **junction 要先删再删目录**：PowerShell 7 的 `Remove-Item -Recurse -Force` 只删联接本身、不跟进主树（本机 PS 7.6.5 实测；Windows PowerShell 5.1 不保证），但**删 worktree 时先单独拆掉联接更稳**：`cmd /c rmdir <worktree>\training-app\叉车维修培训学员端跨端应用\node_modules`，再 `git worktree remove`。Windows PowerShell 5.1 下 `Remove-Item -Recurse` 是否会跟进 junction 并删掉主树内容，**未证实**——所以不要靠它。

- **HBuilderX 仍要唯一项目名（worktree 解决不了）**：HBuilderX 按**项目名**解析，worktree / 完整 clone 里的项目目录 basename 与主树相同 ⇒ 跑 HBuilderX 前要把该项目目录改成**唯一名**。改名期间 `git diff --name-only` 会误报整目录删除（删除 + 未跟踪新增），所以**任何 git 操作前必须先把项目目录名改回**。

  ⚠️ **但「改回来」这一步在本机会卡住，成因与解（2026-09-17 实测补，血账）**：本文件与 `hx-run.ps1` / `auto-screenshot.ps1` 的收尾建议都写「先 `cli project close --path <项目>` 结束会话」——
  **HBuilderX 5.23 没有 `project` 这个子命令**（实测 `cli project close --path …` → `命令'project'不存在或缺少参数`），故这条配方**在本机不可用**。
  而「真运行」（`launch app-android`，不带 `--compile`）的会话**不自己收口**，且派发它的包装 `pwsh` 与其 `cli.exe` 子进程**把项目目录当作 CWD、并攥着 `.ci-verify\launch-*.out`**
  ⇒ **整个项目目录既改不回来也删不掉**（`Rename-Item` 报 `The process cannot access the file because it is being used by another process`；`git worktree remove` 同理）。
  **唯一的非破坏解**是本文件已写的另外半句「发起下一次 launch 把它顶掉」：**任何一次新的 `cli launch`** 都会让旧会话退出、句柄随之释放（实测：卡住期间 `pwsh` 30040 + `cli.exe` 32676 一直活着；这两个进程属于**你自己那次 launch**，但这不改变纪律）。
  **禁止**用 `Stop-Process` / `taskkill` 绕过（AGENTS.md 反模式「kill `cli` 或 HBuilderX 主程序」；`hx-run.ps1` 的契约测试 C2 也钉死脚本内不得引入强杀）。
  ⇒ **实操**：在 worktree 里跑真运行（🟡/🔴，即 ①a 取证那类）**之前**先想清楚「这个项目目录还要不要改名 / 删 worktree」；改名必须发生在**任何 HBuilderX 步骤之前**，
  而改回来只能等到**下一次 launch 之后**（或维护者在 HBuilderX GUI 里点「停止」）。
  **退路**：真的必须在同一会话里继续交付时，不要与锁硬碰 —— 证据文件仍可正常**读/复制**（只有改名与删除被拒），可用本文件下面「游离提交：完整配方」把产物按**正确入库路径**提交，交付不受影响；
  被卡住的 worktree 待旧会话退出后再 `改名 → git worktree remove` 收尾。

- **`stash` 是全仓共享的**：所有 worktree 共用同一个 stash 栈。别在多 worktree 之间长期留 stash，用完即 `pop`/`drop`；否则另一会话 `/clear` 后无法分辨哪条 stash 是自己的。

- **worktree 元数据也在 `.git` 下**：`.git/worktrees/` 与 `git worktree list` 全局共享，不是每个 worktree 一份。

### worktree 元数据异常：先实测，再决定要不要修（2026-09-13 重写）

**先实测，别照配方动手。** 历史症状是 `E:\FL\.git\worktrees` **列不出内容**（`Get-ChildItem` 返回 0 项），导致 `git worktree add` 连续失败。**但 2026-09-13 复测该症状已不复现**：枚举正常返回 1 项、`git worktree list` 正常返回 2 条。所以第一步永远是这条只读探测：

```powershell
Get-ChildItem E:\FL\.git\worktrees -Force           # 能列出条目 = 枚举正常
cd E:\FL; git worktree list                         # 能列出全部 worktree = 注册面正常
git worktree add E:\wt-probe -b chore/wt-probe HEAD # 唯一的实证：能建 = 没问题（试通后 worktree remove）
```

**三条判据全过 ⇒ 无需修复**，下面那条破坏性配方**不要执行**。

#### 2026-09-13 二次实测定案：不是 ACL，且**不可修**（本节结论以此为准）

**先做的那次「试通」做了，结果是失败**（原文）：

```
git -C E:\FL worktree add --detach E:\wt-aclprobe origin/master
  → fatal: could not create leading directories of '.git/worktrees/wt-aclprobe': Permission denied
```

**症状精确定性**：把它当「枚举为空」或「ACL 被拒」都不对。真实形态是**这一个目录项打不开、也删不掉**：

| 操作 | 结果 |
|---|---|
| `dir E:\FL\.git` / `Get-Item` 读属性 | **成功**（名字 `worktrees`、9 个纯 ASCII 字符、`Attributes=Directory`、无 reparse point） |
| `Get-ChildItem <该目录>` / 往里建文件 | `Access is denied` |
| `rd` / `Remove-Item -Recurse` / `ren` / `attrib -r -s -h -a` / 加 `\\?\` 前缀绕规范化 | **全部** `Access is denied` |
| `icacls`（含 `/reset`）/ `fsutil reparsepoint query` | `Access is denied` |
| 往 `.git` 本身建文件（对照） | **成功** ⇒ 坏的只有这一个项，不是 `.git` 被锁 |

**为什么「ACL 路线」根本不成立**：本卷是 **exFAT，没有 ACL**（同级 `.git\refs` 的 `icacls` 读数就是「No permissions are set」）；`takeown` 返回的就是结论本身：`ERROR: File ownership cannot be applied on insecure file systems; there is no support for ACLs.` ⇒ **`icacls /reset` 这类配方在本卷上不可能起作用**（下面是旧配方，保留仅为说明它为何失效）。

**不是 exFAT 通病的对照实验**（同日实测）：同卷另一个仓库 `E:\_spike` 上 `git worktree add` **成功**（exit 0，建完干净移除）；`D:`（NTFS）上也**成功**。⇒ worktree 功能没坏、exFAT 也不必然坏，**坏的是 `E:\FL\.git` 里这一个目录项**。

**与卷级损坏的关系（同日只读 `chkdsk E:` 的判决）**：`exit=3`，`Windows found errors on the disk, but will not fix them …` + `Corruption was found while examining files and directories` + `Corruption was found while examining the volume bitmap`。⇒ 这是**卷级元数据损坏**的一处症状，不是孤例（详见文末「卷健康与新工作放置」节）。**可修路径只剩 `chkdsk E: /f`，而它要卸载整卷**。

#### ⚠️ 旧配方的实害（此前没写，务必先读）

旧配方的「优先」路径是 `Remove-Item -Recurse -Force E:\FL\.git\worktrees` + `git worktree prune`。它当时声明「低风险」的依据是**「该目录枚举为空、只有主树一条注册」** —— 该前提当时确实不成立：

`E:\FL\.git\worktrees\wt-tabbar-align` 曾是**活 worktree**（`E:\wt-tabbar-align`）的登记目录，而活 worktree 的 `.git` 是个**文件**，内容写着 `gitdir: E:/FL/.git/worktrees/wt-tabbar-align`。**删掉那个目录 = 把该 worktree 孤儿化**（`.git` 指向不存在的路径）。

**2026-09-13 复测更新（状态已变，务必看这一条）**：`E:\FL\.git\worktrees` 现在**枚举为 0 条**，`git worktree list` 只剩主树一条 ⇒ 上面那个活 worktree 已被移除，**当前没有活 worktree 需要保护**。但**规矩不变**：这条判据每次都要现测，**不能按记忆走** —— 「有输出就别删」仍然是唯一的判据。

所以：**只要 `git worktree list` 里除主树外还有别的 worktree，就绝对不要删 `.git\worktrees`。** 先检查（有输出就别删）：

```powershell
cd E:\FL; git worktree list | Select-Object -Skip 1
```

#### 真要修时的安全顺序（**2026-09-13 作废重写：旧配方的第 2 步已被证伪**）

1. 先按上面的只读探测确认「确实坏了」，并**把报错原文留证**；
2. ~~重置该目录 ACL：`icacls "E:\FL\.git\worktrees" /reset /T /C`~~ —— **作废**：本卷是 exFAT、**没有 ACL**，`icacls` 连读都读不了（`Access is denied`），该命令不可能起作用（见上表与 `takeown` 的原话）。同理 `takeown` / `ren` / `attrib` / `rd` 全部已被实测否决；
3. **剩下的可修路径只有一条：`chkdsk E: /f`**，而它**要卸载整卷**。卸载前必须：停掉所有会话与 HBuilderX、确认没有进程的 cwd 在 `E:`；并且**先备份只在此处、且未推送的数据**（exFAT 无日志，`chkdsk /f` 修位图/目录项时可能截断或把不可解析项丢进 `FOUND.000`）；
4. 修完重跑一次 `git worktree add E:\wt-probe -b chore/wt-probe HEAD` 试通，再 `git worktree remove E:\wt-probe`。

✅ **待办状态（2026-09-13 实测结案）**：`docs/agents/handoff-验收门-2026-09-11.md` 六、待办里那条「ACL 是否自愈未证实」**已由试通结案，结论是「没自愈、且非 ACL 可修」** —— `git worktree add` 仍然 `Permission denied`，与卷级损坏同源。
⇒ **多会话隔离继续用独立 clone（放 `D:`，见文末一节）；本仓主树里 `git worktree` 不可用**，替代手法是**游离提交**：临时索引 `read-tree <base>` + `update-index --cacheinfo` 只替换本次文件 + `commit-tree -p <base>`，全程不碰工作树、默认索引与 HEAD（2026-09-13 实测用它跑完了整票改动与一次跨分支同步合并）。

⚠️ **上面那句「主树里 `git worktree` 不可用」是 `E:\FL` 的结论（exFAT + `.git/worktrees` 坏项），不是 `D:\FL` 的**：2026-09-15 在 `D:\FL` 主树实测 `git worktree add D:\wt-850-docs -b docs/xxx origin/master` **一次成功**（检出 1615 个文件，`git worktree list` 正常列出三条）。⇒ **主树在 `D:` 时首选独立 worktree 做隔离**；只有「工作树已被别的会话占着、来不及另开」或「只提交一两个文件」时才用下面的游离提交。

#### 游离提交：完整配方（2026-09-15 补全）

**适用场景**：共享工作树里有别的会话的未提交改动（此时 `git checkout` / `git merge` 会被 git **正确地**拒绝），而你只需要**提交本次文件**，或**把 master 同步进自己的分支**。

```powershell
# 1) 只提交本次文件 —— 不碰工作树 / 默认索引 / HEAD
$tmp = "$env:TEMP\my-task-index"; Remove-Item $tmp -Force -ErrorAction SilentlyContinue
$env:GIT_INDEX_FILE = $tmp                       # ← 关键：临时索引，别污染共享 index
git read-tree <base>                             # 用基线（本分支 tip）填充临时索引
foreach ($f in $files) {                         # 只替换本次改动的文件
  $sha = (git hash-object -w $f).Trim()
  git update-index --add --cacheinfo "100644,$sha,$relPath"
}
$tree   = (git write-tree).Trim()
$commit = (git commit-tree $tree -p <base> -F msg.txt).Trim()
Remove-Item Env:\GIT_INDEX_FILE
git update-ref refs/heads/<branch> $commit       # 本地分支跟上
git push origin "${commit}:refs/heads/<branch>"  # 不用 checkout 就能推
```

```powershell
# 2) 把 master 同步进自己的分支 —— 三方合并在「索引内」完成，工作树全程不动
$env:GIT_INDEX_FILE = $tmp
git read-tree -m (git merge-base origin/master <branch>) <branch> origin/master   # 无冲突 ⇒ exit 0
$tree   = (git write-tree).Trim()
$commit = (git commit-tree $tree -p <branch> -p origin/master -F merge-msg.txt).Trim()
Remove-Item Env:\GIT_INDEX_FILE
```
（2026-09-15 实测：用 1) 提交了 4 张真机取证截图、用 2) 把当时领先 6 个提交的 master 并进分支并推送，全程没碰并发会话的工作树。**冲突时 `read-tree -m` 会直接失败**（不产生半成品）⇒ 那时才需要另开 worktree，不要在共享树里解冲突。）

**两条纪律（2026-09-15 均实际踩到）**：

1. **不要往共享的默认索引里 `git add`**：一旦暂存，别的会话 `git commit` 会把你的文件一起带走。要么一开始就用 `GIT_INDEX_FILE`，要么发现后立刻 `git reset HEAD -- <paths>` 撤回（只动这两条路径，不碰别人的暂存）。
2. **不留未跟踪残留**：取证产物（截图等）若只落在共享工作树而不入库，别的会话日后 `git merge master` 会报 `untracked working tree files would be overwritten by merge`。**入库确认后再删工作树副本**：逐个比对 `git hash-object <file>` 与 `git rev-parse origin/master:<path>` 一致，再删。

#### 重复副本的现状与清理纪律（2026-09-13 实测）

- 实测 `E:\` 下有 **11 个同仓库克隆**，约 **1443 MB**（旧文档记的「约 618 MB / 892 MB」已过时）。**2026-09-13 复测：`git worktree list` 只剩主树，`.git/worktrees` 枚举为 0 条；我在 `E:\` 顶层看到 9 个 `_g*` 克隆 + 1 个 `wt-ai-single`；主树 `E:\FL` 约 308 MB，`_g1` 94 MB、`_g624c` 128 MB、`_g2` 169 MB。** 文件数与旧记录不一致（旧记 11 个克隆 vs 现见 10 个目录）⇒ **要清理时现测，不按本条数字行事。**
- **不能按目录名盲删**：其中 **9 个持有未推送提交或脏文件**（`_g624c` 577 未推送 + 440 脏、`_spike` 411 脏）。删前必须逐个体检 `git status` 与 `git log --branches --not --remotes`，并查该分支的 PR 是否已合并（`gh pr list --head <分支> --state all`）—— **不要凭分支名或印象判断**。
- **候选的正确判据**（首版按目录名扫，把无关仓库与活 worktree 都列成了待删项）：① `.git` 是**目录**（worktree 的 `.git` 是**文件**，`Test-Path` 对两者都为真，所以必须区分）；② 路径不在 `git worktree list` 里；③ `origin` 归一后与主树同仓库（`git@github.com:Owner/Repo.git` 与 `https://github.com/Owner/Repo.git` 视为同一个）。按此判据，`E:\deepseek-harness`（**别的仓库**）与 `E:\wt-tabbar-align`（**活 worktree**）必须排除。
- 主树 `E:\FL` 上的游离提交 `5446e6f`（重复 #849）**勿 push / 勿 reset**。

> 红线：**坏没坏要先实测**（上面的只读探测）。确认坏了之后也**不要**为了绕过它去删 `.git` 下的其他内容；只有本条明确列出的目录可删，且需管理员执行。**任何情况下都不要 `Stop-Process` 杀 HBuilderX**（见 `docs/adr/0008` 的单实例约定）。

## `E:` 盘是 exFAT：写入工具会失败（2026-09-13 实测）

`E:\` 与 `E:\FL` 所在卷的文件系统是 **exFAT**，而 exFAT **不支持硬链接**：

```powershell
Get-Volume | Where-Object DriveLetter -eq 'E' | Select DriveLetter, FileSystemType   # → exFAT
New-Item -ItemType HardLink -Path E:\_t.txt -Target E:\_gh\README.md                  # → Hard links are not supported
```

后果：**以「硬链接 + 原子改名」落盘的写入工具在该卷上会直接失败**，报 `EISDIR: illegal operation on a directory, link ... -> ...`。
症状是**对任何路径都失败**（含仓库根、含新建文件），很容易被误读成路径或权限问题。

**处置（按序取用）**：

1. **在 NTFS 卷上写，再拷回去**：写文件用 `E:\` 之外的暂存目录（`C:\` / `D:\` 都是 NTFS），再 `Copy-Item` 到目标路径 —— 字节原样、`*.ps1` / `*.js` / `*.mjs` 的 LF 不受影响。
2. **改已有文件**：把「精确旧文本 → 新文本」的替换对写成文件，用一个 10 行的 Node 脚本做**字面量拼接**（先断言锚点唯一），比在 shell 里做正则转义可靠。
   ⚠️ 拼接时**必须用函数式替换**（`s.replace(old, () => next)`）。用字符串替换会把替换文本里的 ``$` `` / `$'` / `$&` / `$1` 当成替换模式展开 —— 实测有一次把 PowerShell 脚本的**后半份整个复制了一遍**。
   ⚠️ **锚点也要按目标文件的 EOL 归一**：`*.yml` 没被 `.gitattributes` 钉 LF（见下节），在 Windows 工作树里是 **CRLF**。若拼接脚本只把**替换文本**转成 CRLF、**锚点**仍写成 LF，多行锚点会**匹配 0 次**，而失败信息是「锚点不唯一（found 0）」—— 看着像锚点写错，其实是换行符不一致。单行锚点不受影响，所以这个坑**只在多行锚点上露头**，且会让「注入自检」静默失效（实测踩到过一次：以为验证过了，其实是注入根本没生效）。

## 卷健康与新工作放置：新克隆 / 新 worktree 放 `D:`（2026-09-13 实测 + 维护者决定）

**现状（只读 `chkdsk E:`，2026-09-13 15:15，耗时约 19 分钟，未改盘）**：

```
Windows found errors on the disk, but will not fix them
because disk checking was run without the /F (fix) parameter.
Corruption was found while examining files and directories.
Corruption was found while examining the volume bitmap.
→ exit=3（500 GB 卷 / 1,421,107 个文件 / 203,182 个索引 / 125 GB 可用 / 0 KB bad sectors）
```

**物理层与范围**：

- `E:` 挂在 **USB 外接 SSD** 上（`SSD AS2235`，477 GB，BusType=USB，Health 正常，**0 KB bad sectors**）⇒ 是**元数据损坏**，不是介质要坏；exFAT 无日志，最常见的成因是**未 eject 即拔 / 掉电 / USB 桥接丢写**。
- 损坏**不止 `.git\worktrees` 一处**：报错集中在 `E:\比赛\区块链\…\GZ036 国赛10套题-前后端代码\projects\project2\SupplyChain_finish\front\node_modules\`（rxjs / core-js / babel-runtime 一类**可重装的依赖树**，几百条）。
- 仓库本体**未被波及**：`git -C E:\FL fsck` 干净（exit 0，9 秒）。

**决定（维护者，2026-09-13）**：**不跑 `chkdsk E: /f`**（要卸载 E:，而 E: 上挂着 14 个仓库、HBuilderX、Android SDK 与在跑会话）⇒ 属「**已接受未验证风险**」：**位图损坏保留未修**。配套纪律是按风险面收缩写入：

- **新克隆 / 新 worktree / 新数据集放 `D:`** —— 实测 `D:` 是 **NTFS + 内置 NVMe**（`WD Blue SN580 1TB`，373 GB 可用），`git worktree add` 正常（同日实测 exit 0）。**不在 `E:` 上开新的重写入工作**。
- **不整体迁移主树 `E:\FL`**（最小影响；迁移要重导 HBuilderX 项目、会踩上面「HBuilderX 按项目名解析」那条）。
- 「先 eject 再拔」这条习惯是唯一能防复发的一环。

**判据（每次现测，不按本条记忆行事）**：

```powershell
chkdsk E:                                                  # 只读；仍报 "will not fix them" + 卷位图损坏 ⇒ 未修
git -C E:\FL worktree add --detach E:\wt-probe HEAD        # 仍 Permission denied ⇒ 坏项还在
Get-Volume -DriveLetter E | Select DriveLetter,FileSystem  # exFAT
```

3. **不要**把仓库整体搬到 `C:` 来规避（空间与 clone 体积都不划算），也不要指望用它解决 `.git\worktrees` 那个坏目录项（见上一节：换成路径也不解决，因为坏项在 `.git` 里面）。**新工作放 `D:`** 见下一节。
