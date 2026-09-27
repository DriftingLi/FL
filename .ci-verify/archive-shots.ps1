<#
.SYNOPSIS
    把 ①a 取证截图按本仓纪律压到入库规格（宽 ≤720、单张 ≤150KB）。

.DESCRIPTION
    本机**没有** webp 编码器（cwebp / ffmpeg / magick 均未安装）⇒ 按 ADR-0008 的既定退路走
    **JPEG q75**；压缩阶梯与 `scripts/device-capture.ps1` 的 `Compress-Screenshot` 同构：
    q75/720 → q60/720 → q75/560 → q60/420 → q60/360（逐级降到 150KB 以下为止）。
    只负责压缩与落盘，不做任何视觉判读 —— 内容判据另行 OCR（local-ocr.ps1）。

    用法（从 pwsh 调外层 pwsh 时**数组参数会被折叠**，一次只传一个 -Source）：
      pwsh -File archive-shots.ps1 -Source <一张图> -OutDir <目录>
#>
param(
    [Parameter(Mandatory = $true)][string]$Source,
    [Parameter(Mandatory = $true)][string]$OutDir,
    [int]$MaxBytes = 153600
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'
Add-Type -AssemblyName System.Drawing
New-Item -ItemType Directory -Force -Path $OutDir | Out-Null

function Resize-Bitmap {
    param([System.Drawing.Image]$Image, [int]$MaxWidth)
    if ($Image.Width -le $MaxWidth) { return [System.Drawing.Bitmap]$Image }
    $h = [int][Math]::Round($Image.Height * ($MaxWidth / $Image.Width))
    $bmp = New-Object System.Drawing.Bitmap $MaxWidth, $h
    $g = [System.Drawing.Graphics]::FromImage($bmp)
    $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
    $g.SmoothingMode = [System.Drawing.Drawing2D.SmoothingMode]::HighQuality
    $g.PixelOffsetMode = [System.Drawing.Drawing2D.PixelOffsetMode]::HighQuality
    $g.DrawImage($Image, 0, 0, $MaxWidth, $h)
    $g.Dispose()
    return $bmp
}

function Save-Jpeg {
    param([System.Drawing.Bitmap]$Bitmap, [string]$Path, [int]$Quality)
    $codec = [System.Drawing.Imaging.ImageCodecInfo]::GetImageEncoders() | Where-Object { $_.MimeType -eq 'image/jpeg' }
    $ps = New-Object System.Drawing.Imaging.EncoderParameters 1
    $ps.Param[0] = New-Object System.Drawing.Imaging.EncoderParameter ([System.Drawing.Imaging.Encoder]::Quality), ([long]$Quality)
    $Bitmap.Save($Path, $codec, $ps)
    $ps.Dispose()
}

$ladder = @(
    @{ Q = 75; W = 720 }, @{ Q = 60; W = 720 },
    @{ Q = 75; W = 560 }, @{ Q = 60; W = 420 }, @{ Q = 60; W = 360 }
)

if (-not (Test-Path -LiteralPath $Source)) { echo "[skip] 缺文件: $Source"; exit 2 }
$name = [System.IO.Path]::GetFileNameWithoutExtension($Source)
$img = [System.Drawing.Image]::FromFile($Source)
try {
    $done = $false
    foreach ($a in $ladder) {
        $bmp = Resize-Bitmap -Image $img -MaxWidth $a.W
        $out = Join-Path $OutDir ("{0}.jpg" -f $name)
        Save-Jpeg -Bitmap $bmp -Path $out -Quality $a.Q
        $len = (Get-Item $out).Length
        if ($bmp -ne $img) { $bmp.Dispose() }
        if ($len -le $MaxBytes) {
            echo ("OK   {0}  q{1}/w{2}  {3} B  => {4}" -f $name, $a.Q, $a.W, $len, $out)
            $done = $true
            break
        }
        echo ("...  {0}  q{1}/w{2}  {3} B（超 {4} B，降级）" -f $name, $a.Q, $a.W, $len, $MaxBytes)
    }
    if (-not $done) { echo ("WARN {0} 降到阶梯底仍未达标" -f $name) }
} finally { $img.Dispose() }
