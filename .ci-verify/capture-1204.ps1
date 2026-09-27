# #1204 ①a 会话式取证（临时脚本，不入库）
# 用法：pwsh -NoProfile -File .ci-verify/capture-1204.ps1 -Name resume -OutDir <工作树证据目录>
param(
    [Parameter(Mandatory = $true)][string]$Name,
    [Parameter(Mandatory = $true)][string]$OutDir,
    [string]$Device = '192.168.0.212:43057'
)
$ErrorActionPreference = 'Stop'
$adb = 'D:\android-sdk\platform-tools\adb.exe'
if (-not (Test-Path $OutDir)) { New-Item -ItemType Directory -Path $OutDir -Force | Out-Null }

# 1) a11y 文本（uvue 的文字在 content-desc；输入框值在 text）
& $adb -s $Device shell "uiautomator dump --compressed /sdcard/ui.xml" | Out-Null
& $adb -s $Device pull /sdcard/ui.xml "$OutDir/$Name.a11y.xml" | Out-Null
$xml = Get-Content "$OutDir/$Name.a11y.xml" -Raw
$vals = [regex]::Matches($xml, '(?:content-desc|text)="([^"]{1,120})"') | ForEach-Object { $_.Groups[1].Value } | Where-Object { $_ -ne '' }
$vals | Set-Content "$OutDir/$Name.a11y.txt" -Encoding UTF8

# 2) 截图
& $adb -s $Device exec-out screencap -p > "$OutDir/$Name.png"

# 3) 机检行
$bytes = (Get-Item "$OutDir/$Name.png").Length
$nodes = ([regex]::Matches($xml, '<node ')).Count
"CAPTURE name=$Name device=$Device nodes=$nodes png_bytes=$bytes a11y_lines=$($vals.Count) at=$(Get-Date -Format o)"
"--- a11y 文本 ---"
$vals | ForEach-Object { $_ }
