$log = 'C:\Users\ZHENG\AppData\Local\Temp\kb_monitor_log.txt'
$failCount = 0
while ($true) {
    $ts = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'
    try {
        $output = (& ssh -o ConnectTimeout=15 -o StrictHostKeyChecking=no -i 'C:\Users\ZHENG\.ssh\pve-04-colleague' -p 2204 root@183.36.195.104 "bash /root/kb_orchestrator.sh" 2>&1 | Out-String).Trim()
        $exitCode = $LASTEXITCODE
        if ($exitCode -ne 0) {
            $failCount++
            Add-Content -Path $log -Value "[$ts] SSH_EXIT=$exitCode FAIL_COUNT=$failCount"
            if ($output) { Add-Content -Path $log -Value "[$ts] $output" }
        } else {
            $failCount = 0
            Add-Content -Path $log -Value "[$ts] $output"
        }
        if ($output -match 'PIPELINE_DONE') {
            Add-Content -Path $log -Value "[$ts] PIPELINE_DONE_EXIT"
            break
        }
        if ($failCount -ge 3) {
            Add-Content -Path $log -Value "[$ts] THREE_CONSECUTIVE_SSH_FAILURES"
        }
    } catch {
        $failCount++
        Add-Content -Path $log -Value "[$ts] EXCEPTION: $_"
        if ($failCount -ge 3) {
            Add-Content -Path $log -Value "[$ts] THREE_CONSECUTIVE_SSH_FAILURES"
        }
    }
    Start-Sleep -Seconds 600
}
