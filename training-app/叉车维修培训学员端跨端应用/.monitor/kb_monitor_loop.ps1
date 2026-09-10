$log = "E:\FL\training-app\叉车维修培训学员端跨端应用\.monitor\kb_monitor.log"
$failCount = 0
while ($true) {
    $stamp = Get-Date -Format "yyyy-MM-dd HH:mm:ss"
    $out = ssh -o ConnectTimeout=15 -o StrictHostKeyChecking=no -i C:\Users\ZHENG\.ssh\pve-04-colleague -p 2204 root@183.36.195.104 "bash /root/kb_orchestrator.sh" 2>&1
    $joined = $out -join "`n"
    if ($LASTEXITCODE -ne 0) {
        $failCount++
        $msg = "[$stamp] SSH_FAIL($failCount/3) :: $joined"
    } else {
        $failCount = 0
        $msg = "[$stamp] OK :: $joined"
    }
    Add-Content -Path $log -Value $msg
    if ($joined -match "PIPELINE_DONE") { break }
    Start-Sleep -Seconds 600
}