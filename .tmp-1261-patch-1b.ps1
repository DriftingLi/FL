$p = 'D:\FL\.tmp-1261-pr-body.md'
$t = [System.IO.File]::ReadAllText($p)
$from = '维护者原话：「Q1 做了」'
$to = '维护者原话：「长按 `###` 弹『标题』」'
$n = ([regex]::Matches($t, [regex]::Escape($from))).Count
$t = $t.Replace($from, $to)
$t = $t -replace "`r`n", "`n"
[System.IO.File]::WriteAllText($p, $t, (New-Object System.Text.UTF8Encoding($false)))
"替换=$n CR=" + ([regex]::Matches([System.IO.File]::ReadAllText($p), "`r")).Count + " 含新原话=" + ([System.IO.File]::ReadAllText($p)).Contains('长按 `###` 弹『标题』')
