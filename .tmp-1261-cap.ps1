param(
  [Parameter(Mandatory=$true)][string]$Name,
  [string]$Out = "D:\FL\.tmp-1261-capture"
)
$adb = "D:\android-sdk\platform-tools\adb.exe"
$dev = "adb-b32d8398-ul54S3._adb-tls-connect._tcp"
New-Item -ItemType Directory -Force -Path $Out | Out-Null
# a11y：走设备侧落盘 + pull（避免 PowerShell 管道把 XML 弄坏）；**不用 --compressed**（长 content-desc 会被丢成空串）
& $adb -s $dev shell uiautomator dump /sdcard/cap.xml | Out-Null
& $adb -s $dev pull /sdcard/cap.xml "$Out\$Name.xml" | Out-Null
& $adb -s $dev shell screencap -p /sdcard/cap.png | Out-Null
& $adb -s $dev pull /sdcard/cap.png "$Out\$Name.png" | Out-Null
Write-Output "=== $Name ==="
$raw = Get-Content "$Out\$Name.xml" -Raw
try {
  [xml]$x = $raw
  $x.SelectNodes("//node") | Where-Object { $_.text -or $_.'content-desc' } | ForEach-Object {
    $t = $_.text; $d = $_.'content-desc'
    if (-not $t) { $t = '' }
    if (-not $d) { $d = '' }
    "{0} | text='{1}' | desc='{2}'" -f $_.bounds, $t, $d
  }
} catch {
  Write-Output "XML 解析失败：$($_.Exception.Message)"
  Write-Output $raw.Substring(0, [Math]::Min(600, $raw.Length))
}
