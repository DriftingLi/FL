$p = 'D:\FL\.tmp-1261-pr-body.md'
$t = [System.IO.File]::ReadAllText($p)
$from = '**恢复路径**：先在 HBuilderX GUI'
$to = '历史读数所在的日志路径为 `.ci-verify/kotlin-all.log`（该读数绑定 `fa6881e8`，**不覆盖本 head** ⇒ 本行不得据此判过；重跑后原样回填本行）。**恢复路径**：先在 HBuilderX GUI'
$n = ([regex]::Matches($t, [regex]::Escape($from))).Count
$t = $t.Replace($from, $to)
$t = $t -replace "`r`n", "`n"
[System.IO.File]::WriteAllText($p, $t, (New-Object System.Text.UTF8Encoding($false)))
"替换处数=$n  CR=" + ([regex]::Matches([System.IO.File]::ReadAllText($p), "`r")).Count + "  含 kotlin-all.log=" + ([System.IO.File]::ReadAllText($p)).Contains('.ci-verify/kotlin-all.log')
