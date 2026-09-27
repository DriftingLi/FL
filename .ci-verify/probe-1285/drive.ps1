$ErrorActionPreference = 'Stop'
$dir = 'D:\FL\.ci-verify\probe-1285'
. (Join-Path $dir 'invoke-process-current.ps1')
$pwsh = (Get-Process -Id $PID).Path

# S1+S2：多行（300 行），中间有标记行，最后一行也是标记行，末行后立刻退出
$sw = [Diagnostics.Stopwatch]::StartNew()
$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-File',(Join-Path $dir 'child-lines.ps1')) -TimeoutSeconds 15 -Tag 'S1'
$sw.Stop()
$lines = ([regex]::Matches($r.Output, '(?m)^LINE-\d+$')).Count
$mid = $r.Output -match 'MIDDLE-MARKER'
$last = $r.Output -match 'LAST-MARKER'
Write-Output "RESULT S1 elapsed=$([int]$sw.ElapsedMilliseconds)ms timedout=$($r.TimedOut) exit=$($r.ExitCode) lines=$lines/300 mid=$mid last=$last"

# S3：超时语义保持：子进程睡 60 秒，预算 4 秒
$slow = Join-Path $dir 'child-slow.ps1'
Set-Content -LiteralPath $slow -Value 'Start-Sleep 60' -Encoding utf8
$sw = [Diagnostics.Stopwatch]::StartNew()
$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-File',$slow) -TimeoutSeconds 4 -Tag 'S3'
$sw.Stop()
Write-Output "RESULT S3 elapsed=$([int]$sw.ElapsedMilliseconds)ms timedout=$($r.TimedOut) head=$($r.Output.Substring(0, [Math]::Min(40, $r.Output.Length)) -replace '\r?\n', ' ')"

# S4：子脚本立即退出，但后代握管道 40 秒 —— 预算只有 4 秒，总耗时是否越界？
$sw = [Diagnostics.Stopwatch]::StartNew()
$r = Invoke-Process -FilePath $pwsh -Arguments @('-NoProfile','-File',(Join-Path $dir 'child-hold.ps1')) -TimeoutSeconds 4 -Tag 'S4'
$sw.Stop()
$hasBefore = $r.Output -match 'BEFORE-HOLD'
$hasAfter = $r.Output -match 'AFTER-HOLD'
Write-Output "RESULT S4 elapsed=$([int]$sw.ElapsedMilliseconds)ms timedout=$($r.TimedOut) before=$hasBefore after=$hasAfter"
