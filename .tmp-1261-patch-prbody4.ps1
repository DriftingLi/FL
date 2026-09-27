$p = 'D:\FL\.tmp-1261-pr-body.md'
$t = [System.IO.File]::ReadAllText($p)

$lines = $t -split "`n"
$out = New-Object System.Collections.Generic.List[string]
$row4 = '- ④ 本地编译门（默认 ④c `npm run build:kotlin-all`） — 执行人：agent 执行 · 日期：2026-09-22 · 复测对象：`49f7195c`（含形态回退提交 `f0ef4b8e`；本次跑在 **#1272 的修复**随 master sync 进入本分支之后）· 结论（含产物）：`.ci-verify/kotlin-all.log` 的 `KOTLIN_ALL_RESULT errors=0 classes=1517 files=120 input=unpackage\resources\app-android freshness=fresh` —— publish 步实测命中正向标记「导出 android 成功」、且导出目录 mtime 晚于 publish 基准 ⇒ **新鲜度判据通过**（不是拿旧导出编译）。**据实更正**：我此前在 #1272 写的根因「publish 命令在 v5.24 已不存在」**是误诊** —— 修复者实测该命令存在且可用，当时只是**主程序未就绪**时 CLI 回了一句通用文案（`命令…不存在或缺少参数`）；真正的缝是「脚本靠两条文案判失败 + 只看有没有 `.kt` 而不判新鲜度」，已由 **PR #1275**（#1272）以 fail-closed 双判据修掉，并随本次 sync 进入本分支。'
$warn = '> ✅ **四门齐备、可合并**：①＝①a（**第二次分支收口**重跑，7 份状态入仓）+ ①b（**维护者签收**，原话「Q1 做了」由 agent 代录、未自拟）／②＝免（未命中 MP-WEIXIN 面）／③＝CI 绿／④c＝`errors=0 freshness=fresh`。**同步 master 时带进来的其它移动端改动**（#1263 的 forgot-password / login / register）**未触及本 PR 的论坛承载面**：`git -c core.quotePath=false diff --name-only ebb732cd..HEAD -- pages/forum …` 为空 ⇒ ①a 证据继续成立（口径同 #1237 的先例）。'

$hit4 = $false; $hitW = $false
foreach ($l in $lines) {
  if ($l.StartsWith('- ④ 本地编译门')) { $out.Add($row4); $hit4 = $true }
  elseif ($l.StartsWith('> ⚠️')) { $out.Add($warn); $hitW = $true }
  else { $out.Add($l) }
}
$t2 = ($out -join "`n")
$t2 = $t2 -replace "`r`n", "`n"
[System.IO.File]::WriteAllText($p, $t2, (New-Object System.Text.UTF8Encoding($false)))
"row4=$hit4 warn=$hitW  CR=" + ([regex]::Matches([System.IO.File]::ReadAllText($p), "`r")).Count + "  has freshness=fresh=" + ([System.IO.File]::ReadAllText($p)).Contains('freshness=fresh')
