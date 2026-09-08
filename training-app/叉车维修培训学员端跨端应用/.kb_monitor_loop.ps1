# 知识库流水线监控循环：每600秒执行一次
# 只读监控，不改服务器任何文件
$log = "$env:TEMP\kb_monitor_loop.log"
Set-Content -Path $log -Value "monitor loop started" -Encoding UTF8
while ($true) {
    $out = & ssh -o ConnectTimeout=15 -o StrictHostKeyChecking=no -i C:\Users\ZHENG\.ssh\pve-04-colleague -p 2204 root@183.36.195.104 "bash /root/kb_orchestrator.sh" 2>&1 | Out-String
    $out = $out.Trim()
    "===== $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss') =====" | Add-Content -Path $log -Encoding UTF8
    if ([string]::IsNullOrWhiteSpace($out)) {
        "SSH_FAIL(empty)" | Add-Content -Path $log -Encoding UTF8
    } else {
        $out | Add-Content -Path $log -Encoding UTF8
    }
    Start-Sleep -Seconds 600
}