<#
.SYNOPSIS
    ①a 会话式取证：抓「平台模型为空」那道守卫弹出的 toast（真机）。

.DESCRIPTION
    只做两件事：① 点一个**快捷提问**行（它走 `sendText` → `onInputSend`，无需 IME），
    ② 密集连拍截图，把那条 1.5 秒的 toast 抓下来。

    为什么点快捷提问而不是打字：`sendText(text)` 内部就是 `inputText = text` + `onInputSend(text)`
    ⇒ 一次 `input tap` 就能触发守卫，绕开 `input text` 的非 ASCII 限制与 IME 状态问题。

    为什么不用 `uiautomator dump` 抓 toast 文本：实测 dump 启动耗时 **2.5-3.3 秒** > toast 的
    1.5 秒寿命，方法上就抓不到（不是「toast 不在 a11y 树里」的结论）。故文本判据走**本地 OCR**
    （`local-ocr.ps1`，Windows.Media.Ocr，无外部配额），与截图成对取（ADR-0008 ①a 补遗 6）。

    坐标一律取 `uiautomator dump` 的 bounds，**不按截图目测**（目测会静默点空，见 ADR-0008 ①a 补遗 2）。
    据 ADR-0008 ①a 补遗 3：**设备归本次取证专用**时，会话式取证可用 `input`；`device-capture.ps1`
    内的只读红线不受影响（本脚本不是那个载体）。
#>
param(
    [string]$Device = 'f0bae674',
    [string]$AdbExe = 'D:\android-sdk\platform-tools\adb.exe',
    [string]$OutDir,
    [string]$PromptMatch = '如何高效阅读书籍',
    [int]$Burst = 8
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
if (-not $OutDir) { throw '必须给 -OutDir' }
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

function Invoke-Adb {
    # 注意：参数名**不能**叫 $Args —— 那是 PowerShell 的自动变量，会被静默吞掉
    # （首版踩到：adb 收到 0 个参数、打印 help，于是「在 AI 助手页」恒为 False）。
    param([string[]]$AdbArgs)
    return (& $AdbExe -s $Device @AdbArgs 2>&1 | Out-String)
}

function Get-UiXml { return (Invoke-Adb @('exec-out', 'uiautomator', 'dump', '--compressed', '/dev/tty')) }

function Get-BoundsOf {
    param([string]$Xml, [string]$Needle)
    foreach ($n in [regex]::Matches($Xml, '<node\b[^>]*>')) {
        $s = $n.Value
        $cd = ''
        if ($s -match 'content-desc="([^"]*)"') { $cd = $Matches[1] }
        if ($cd -like "*$Needle*" -and $s -match 'bounds="\[(\d+),(\d+)\]\[(\d+),(\d+)\]"') {
            return [pscustomobject]@{
                Left = [int]$Matches[1]; Top = [int]$Matches[2]
                Right = [int]$Matches[3]; Bottom = [int]$Matches[4]; Desc = $cd
            }
        }
    }
    return $null
}

function Save-Screencap {
    param([string]$Path)
    # exec-out 是二进制：不能过 Out-String，必须走原始字节重定向
    & $AdbExe -s $Device exec-out screencap -p > $Path
}

echo "=== 1) 取当前 UI（确认在目标页 + 拿快捷提问坐标）==="
$xmlBefore = Get-UiXml
$onPage = $xmlBefore -like '*AI 助手*'
echo "在 AI 助手页: $onPage"
if (-not $onPage) {
    echo "[fail] 当前不在 AI 助手页 —— 先导航到 pages/ai-assistant/ai-assistant 再跑本脚本"
    exit 2
}

$box = Get-BoundsOf -Xml $xmlBefore -Needle $PromptMatch
if (-not $box) {
    echo "[fail] 没找到快捷提问「$PromptMatch」的节点 —— 页面结构可能变了"
    exit 2
}
$x = [int](($box.Left + $box.Right) / 2)
$y = [int](($box.Top + $box.Bottom) / 2)
echo "快捷提问 '$($box.Desc)' bounds=[$($box.Left),$($box.Top)][$($box.Right),$($box.Bottom)] ⇒ 点 ($x,$y)"

echo ""
echo "=== 2) 点击 → 立即连拍（toast 默认 1.5s）==="
$stamp = Get-Date -Format 'HHmmss'
& $AdbExe -s $Device shell input tap $x $y | Out-Null

$shots = @()
for ($i = 1; $i -le $Burst; $i++) {
    $p = Join-Path $OutDir ("burst-{0}-{1:d2}.png" -f $stamp, $i)
    Save-Screencap -Path $p
    $shots += [pscustomobject]@{ N = $i; Path = $p; Bytes = (Get-Item $p).Length; At = (Get-Date).ToString('HH:mm:ss.fff') }
}

echo "=== 连拍清单（字节数变化即状态变化：含 toast 的帧与对照帧不同）==="
$shots | ForEach-Object { "  {0:d2} {1,9} B  {2}  {3}" -f $_.N, $_.Bytes, $_.At, (Split-Path $_.Path -Leaf) }

$nonEmpty = @($shots | Where-Object { $_.Bytes -gt 0 }).Count
echo ""
echo "TOAST_CAPTURE_RESULT device=$Device page=ai-assistant shots=$nonEmpty/$Burst out=$OutDir"
echo "（文本判据请对含 toast 的帧跑 local-ocr.ps1；a11y 树抓不到 toast，见脚本头部说明）"
