<#
.SYNOPSIS
    本地 OCR（Windows.Media.Ocr，无外部配额）：把截图里的文字判出来。

.DESCRIPTION
    为什么需要它：①a 要求「文本判据与截图成对取」（ADR-0008 ①a 补遗 6），而本机可用的
    可视通道（vision proxy / vision_glance）配额已用尽，toast 又不在 a11y 树里
    （`uiautomator dump` 的启动耗时 2.5-3.3 秒 > toast 的 1.5 秒寿命，方法上就抓不到）。
    Windows 自带 OCR 完全本地、无配额 ⇒ 用它把 toast 文本逐字判出来。

    用法（注意：从 pwsh 调 powershell.exe 时**数组参数会被折叠**，一次只传一个 -Path）：
      powershell.exe -NoProfile -ExecutionPolicy Bypass -File local-ocr.ps1 -Path <一张图>
#>
param(
    [Parameter(Mandatory = $true)][string]$Path,
    [string]$Lang = 'zh-Hans-CN'
)

$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Runtime.WindowsRuntime | Out-Null

$asTaskGeneric = ([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object {
        $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and
        $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1'
    })[0]

function Await {
    param($WinRtTask, [Type]$ResultType)
    $asTask = $asTaskGeneric.MakeGenericMethod($ResultType)
    $netTask = $asTask.Invoke($null, @($WinRtTask))
    $netTask.Wait(-1) | Out-Null
    return $netTask.Result
}

[Windows.Media.Ocr.OcrEngine, Windows.Foundation, ContentType = WindowsRuntime] | Out-Null
[Windows.Storage.StorageFile, Windows.Storage, ContentType = WindowsRuntime] | Out-Null
[Windows.Graphics.Imaging.BitmapDecoder, Windows.Graphics, ContentType = WindowsRuntime] | Out-Null

$engine = $null
try {
    $language = New-Object Windows.Globalization.Language $Lang
    $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage($language)
} catch { }
if (-not $engine) { $engine = [Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages() }
if (-not $engine) {
    $avail = ([Windows.Media.Ocr.OcrEngine]::AvailableRecognizerLanguages | ForEach-Object { $_.LanguageTag }) -join ', '
    echo "OCR_ENGINE_FAIL available=[$avail]"
    exit 2
}
echo "OCR_ENGINE lang=$($engine.RecognizerLanguage.LanguageTag)"

if (-not (Test-Path -LiteralPath $Path)) { echo "[skip] 不存在: $Path"; exit 2 }

$file = Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($Path)) ([Windows.Storage.StorageFile])
$stream = Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
$decoder = Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
$bitmap = Await ($decoder.GetSoftwareBitmapAsync()) ([Windows.Graphics.Imaging.SoftwareBitmap])
$result = Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])

$lines = @($result.Lines | ForEach-Object { $_.Text })
echo "=== $(Split-Path $Path -Leaf)  lines=$($lines.Count) ==="
$lines | ForEach-Object { "  | $_" }
$stream.Dispose()
