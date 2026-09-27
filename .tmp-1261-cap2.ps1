param(
  [string]$Match = "",
  [int]$Index = 0,
  [switch]$NoTap,
  [Parameter(Mandatory=$true)][string]$Name,
  [int]$Wait = 3,
  [string]$Out = "D:\FL\.tmp-1261-capture"
)
$adb = "D:\android-sdk\platform-tools\adb.exe"
$dev = "adb-b32d8398-ul54S3._adb-tls-connect._tcp"
New-Item -ItemType Directory -Force -Path $Out | Out-Null
& $adb -s $dev shell uiautomator dump /sdcard/cap.xml | Out-Null
& $adb -s $dev pull /sdcard/cap.xml "$Out\$Name.xml" 2>$null | Out-Null
& $adb -s $dev shell screencap -p /sdcard/cap.png | Out-Null
& $adb -s $dev pull /sdcard/cap.png "$Out\$Name.png" 2>$null | Out-Null
$raw = Get-Content "$Out\$Name.xml" -Raw
Write-Output "=== $Name (xml $($raw.Length) bytes) ==="
try { [xml]$x = $raw } catch { Write-Output "XML 解析失败"; exit 1 }
$all = $x.SelectNodes("//node") | Where-Object { $_.text -or $_.'content-desc' }
Write-Output "--- 有文本/描述的节点 ---"
foreach ($n in $all) {
  $t = $n.text; $d = $n.'content-desc'
  if (-not $t) { $t = '' }; if (-not $d) { $d = '' }
  Write-Output ("{0} | text='{1}' | desc='{2}'" -f $n.bounds, $t, $d)
}
if (-not $Match) { exit 0 }
$hit = @($all | Where-Object { ($_.text -like "*$Match*") -or ($_.'content-desc' -like "*$Match*") })
if ($hit.Count -eq 0) { Write-Output "!! NOT FOUND: $Match"; exit 2 }
$n = $hit[[Math]::Min($Index, $hit.Count - 1)]
$b = $n.bounds
if ($b -match '\[(\d+),(\d+)\]\[(\d+),(\d+)\]') {
  $cx = [int](([int]$Matches[1] + [int]$Matches[3]) / 2)
  $cy = [int](([int]$Matches[2] + [int]$Matches[4]) / 2)
  Write-Output ">> 命中 $($hit.Count) 个，取第 $Index 个 bounds=$b center=($cx,$cy)"
  if (-not $NoTap) {
    & $adb -s $dev shell "input tap $cx $cy" | Out-Null
    Write-Output ">> 已 tap ($cx,$cy)"
    Start-Sleep -Seconds $Wait
  }
}
