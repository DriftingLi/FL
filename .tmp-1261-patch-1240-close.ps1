$p = 'D:\FL\.tmp-1261-1240-body.md'
$t = [System.IO.File]::ReadAllText($p)
$lines = $t -split "`n"
$out = New-Object System.Collections.Generic.List[string]
$para = '三段**全部落地并已合入 master**：**P1（逻辑面）**（PR #1254 / `a57d51f1`）、**P2（渲染接入）**（PR #1257 / `50524f66`）、**P3（输入区形态）**（PR #1261 —— 形态回退提交 `f0ef4b8e` + ①a 证据批次 `ebb732cd`，**squash 合入 `eb526a68`**）。P3 的证据链：`36ba80c6`（首轮）→ `fa6881e8`（评审修复后）→ **形态回退**（ADR-0025 ⑨ 取代 ⑦）→ `2f4f74e1` / `ebb732cd` **第二次分支收口**重取 ①a（7 份状态 + logcat 入仓）；③ 由 CI 过、④c `errors=0 freshness=fresh`（#1272 的修复随 sync 进入后为真读数）、② 实测未命中 MP-WEIXIN 面而免、①b 由维护者签收。'
$next = '下一步：无 —— 本票随 PR #1261 合入 master（`eb526a68`，2026-09-22）而关闭。**①b 的「执行人」栏用的是维护者原话「Q1 做了」，由 agent 代录（未自拟）**；未覆盖面（回复栏、「我的动态」回复原文、列表卡片 80 字预览）已如实登记在 `docs/verification/forum/1261/README.md` 与 ADR-0025 的对应条目。'
$hitP = $false; $hitN = $false; $hitG = $false
foreach ($l in $lines) {
  if ($l -eq '## 进度：95%') { $out.Add('## 进度：100%'); $hitG = $true }
  elseif ($l.StartsWith('三段**全部落地')) { $out.Add($para); $hitP = $true }
  elseif ($l.StartsWith('下一步：')) { $out.Add($next); $hitN = $true }
  else { $out.Add($l) }
}
$t2 = ($out -join "`n") -replace "`r`n", "`n"
[System.IO.File]::WriteAllText($p, $t2, (New-Object System.Text.UTF8Encoding($false)))
"进度=$hitG 三段=$hitP 下一步=$hitN  CR=" + ([regex]::Matches([System.IO.File]::ReadAllText($p), "`r")).Count
