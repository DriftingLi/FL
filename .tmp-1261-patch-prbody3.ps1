$p = 'D:\FL\.tmp-1261-pr-body.md'
$t = [System.IO.File]::ReadAllText($p)

$old1 = '**①b**：命中能力面，**待签** —— 回退后只剩「长按出中文名」一条需要人眼）'
$new1 = '**①b**：命中能力面，**已由维护者签收**（agent 代录、**未自拟**）—— 维护者原话：「Q1 做了」（回退后①b只剩「长按出中文名」一条需要人眼；其余含写路径均已机检））'
$n1 = ([regex]::Matches($t, [regex]::Escape($old1))).Count
$t = $t.Replace($old1, $new1)

$old2 = '> ⚠️ **本 PR 现在不可合并 —— 两条待办**：（1）**④c 待新导出后重跑**（本机 ④c 会拿旧导出编译出假绿，机制与恢复路径见上 ④ 行；已立票 **#1272**）；（2）**①b 待签** —— 形态回退后**只剩「长按工具栏按钮出中文名」一条需要人眼**（其余含**选图 → 回应用 → 上传成功**的写路径都已机检，见 ① 行判据 6/7/8）：由你给出那一句原文，我代录进 ① 行后合并。'
$new2 = '> ⚠️ **本 PR 现在只差 ④c 一件**：**①b 已签**（维护者原话「Q1 做了」，agent 代录；① 行判据 6/7/8 已把入口侧与**选图 → 回应用 → 上传成功**的写路径机检掉）；**④c 待本 head 的有效导出后重跑** —— 本机 ④c 会拿旧导出编译出假绿（机制见上 ④ 行，已立票 **#1272**），而 HBuilderX 的本地 appResource 导出在这版里只剩 GUI 一条路（`cli publish` 已不存在）：需在 HBuilderX 里打开**本 PR 的 worktree 项目** `D:\FL\wt-1240\training-app\叉车维修培训学员端跨端应用` 执行「发行 → 原生App-本地打包 → 生成本地打包App资源」（注意先关掉**同名**的其它 worktree 项目，否则导出会落到别处 —— 本轮就发生过一次）。刷新后本行 / ④ 行随即回填真实读数。'
$n2 = ([regex]::Matches($t, [regex]::Escape($old2))).Count
$t = $t.Replace($old2, $new2)

$t = $t -replace "`r`n", "`n"
[System.IO.File]::WriteAllText($p, $t, (New-Object System.Text.UTF8Encoding($false)))
"替换 ①行=$n1 警示=$n2  CR=" + ([regex]::Matches([System.IO.File]::ReadAllText($p), "`r")).Count
