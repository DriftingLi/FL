Write-Output 'BEFORE-HOLD'
$psi = [System.Diagnostics.ProcessStartInfo]::new()
$psi.FileName = 'pwsh'
$psi.Arguments = '-NoProfile -Command Start-Sleep 40'
$psi.UseShellExecute = $false
# 不做任何重定向 ⇒ 后代继承当前子进程的 stdout 句柄（即门那侧的管道写端）
[void][System.Diagnostics.Process]::Start($psi)
Write-Output 'AFTER-HOLD'
exit 0
