# KB Pipeline monitor (single-shot, scheduled task ef08d295, every 10 min)
# 只读监控 pve-04 知识库流水线：解析→转换→嵌入→图注→重转换→重灌→完成
# 不改服务器任何文件。输出写入 .kb-pipeline-monitor.log，供助手/用户查阅。
$LogFile = 'E:\FL\training-app\叉车维修培训学员端跨端应用\.kb-pipeline-monitor.log'
$FailFile = 'E:\FL\training-app\叉车维修培训学员端跨端应用\.kb-ssh-fail.count'
$TaskName = 'ef08d295'

$sshCmd = 'ssh -o ConnectTimeout=15 -o StrictHostKeyChecking=no -i C:\Users\ZHENG\.ssh\pve-04-colleague -p 2204 root@183.36.195.104 "bash /root/kb_orchestrator.sh"'
$ts = Get-Date -Format 'yyyy-MM-dd HH:mm:ss'

try {
    $out = Invoke-Expression $sshCmd 2>&1 | Out-String
    $out = $out.Trim()
    if ([string]::IsNullOrWhiteSpace($out)) { $out = '(empty output)' }

    # SSH 成功，重置失败计数
    Set-Content -Path $FailFile -Value '0'

    if ($out -match 'PIPELINE_DONE') {
        Add-Content -Path $LogFile -Value ("[{0}] PIPELINE_DONE|{1}" -f $ts, $out) -Encoding UTF8
        # 全量知识库上线，删除本监控任务
        Unregister-ScheduledTask -TaskName $TaskName -Confirm:$false -ErrorAction SilentlyContinue
        exit 0
    }
    if ($out -match 'ERROR') {
        Add-Content -Path $LogFile -Value ("[{0}] ERROR|{1}" -f $ts, $out) -Encoding UTF8
        exit 0
    }
    if ($out -match 'TRANSITION:') {
        Add-Content -Path $LogFile -Value ("[{0}] TRANSITION|{1}" -f $ts, $out) -Encoding UTF8
        exit 0
    }
    if ($out -match 'RELAUNCH_') {
        Add-Content -Path $LogFile -Value ("[{0}] RELAUNCH|{1}" -f $ts, $out) -Encoding UTF8
        exit 0
    }
    Add-Content -Path $LogFile -Value ("[{0}] PROGRESS|{1}" -f $ts, $out) -Encoding UTF8
} catch {
    $fail = 1
    if (Test-Path $FailFile) {
        $prev = Get-Content $FailFile -Raw
        if ($prev -match '^\d+$') { $fail = [int]$prev + 1 }
    }
    Set-Content -Path $FailFile -Value $fail
    Add-Content -Path $LogFile -Value ("[{0}] SSH_FAIL#{1}|{2}" -f $ts, $fail, $_.Exception.Message) -Encoding UTF8
}
exit 0