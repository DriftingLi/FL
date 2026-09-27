$ErrorActionPreference = 'Continue'
$cap = "D:\FL\.tmp-1261-capture"
$repo = "D:\FL\wt-1240\training-app\叉车维修培训学员端跨端应用\docs\verification\forum\1261"
New-Item -ItemType Directory -Force -Path $repo | Out-Null
$adb = "D:\android-sdk\platform-tools\adb.exe"
$dev = "adb-b32d8398-ul54S3._adb-tls-connect._tcp"

$jobs = @(
  @{ src = '01-forum-create-text';          dst = '01-forum-create-text';          title = '发帖页 · 纯文本档（**无** tab / 无工具栏 / 无边界提示行；图片区是**单个** `＋` + `n/9`）' },
  @{ src = '02-forum-create-markdown';      dst = '02-forum-create-markdown';      title = '发帖页 · Markdown 档（编写|预览 tab + 工具栏**恰好五枚** + 边界提示行；图片区仍是单个 `＋` + `n/9`）' },
  @{ src = '03-forum-create-toolbar-insert';dst = '03-forum-create-toolbar-insert';title = '点工具栏 `###` 后（正文由 `E01code` 变 `### E01code`，计数 0/10000 → 11/10000）' },
  @{ src = '04-forum-create-preview';       dst = '04-forum-create-preview';       title = '预览档（源串 `### ` 消失，块渲染出 `E01code`；工具栏收起）' },
  @{ src = '05-image-entry-picker';         dst = '05-image-entry-picker';         title = '点图片区 `＋` 之后 —— 弹出**含两个来源的选择器**（拍摄 / 从相册选择 / 取消）⇒ 不再直达相机' },
  @{ src = '05b-album-picker';              dst = '06-album-picker';               title = '选「从相册选择」后 —— uni-app x 图片选择器（`完成(0/9)`，额度 9 与 `count` 一致；**无应用内权限弹窗**）' },
  @{ src = '06-after-upload';               dst = '07-after-upload';               title = '选定 1 张并「完成」后回到应用 —— 缩略图出现（带 `×`）+ 计数 `1/9` ⇒ **选图 → 回应用 → 上传成功**' }
)

foreach ($j in $jobs) {
  $xmlPath = Join-Path $cap "$($j.src).xml"
  if (-not (Test-Path $xmlPath)) { Write-Output "缺 XML：$xmlPath"; continue }
  [xml]$x = Get-Content $xmlPath -Raw
  $nodes = $x.SelectNodes('//node')
  $texts = New-Object System.Collections.Generic.List[string]
  $descs = New-Object System.Collections.Generic.List[string]
  foreach ($n in $nodes) {
    if ($n.text) { $texts.Add([string]$n.text) }
    if ($n.'content-desc') { $descs.Add([string]$n.'content-desc') }
  }
  $out = New-Object System.Collections.Generic.List[string]
  $out.Add("# $($j.title)")
  $out.Add("# 源：未压缩 uiautomator dump（adb shell uiautomator dump + adb pull；**不用** --compressed）")
  $out.Add("")
  $out.Add("## text=")
  foreach ($t in $texts) { $out.Add($t) }
  $out.Add("")
  $out.Add("## content-desc=")
  foreach ($d in $descs) { $out.Add($d) }
  $txtPath = Join-Path $repo "$($j.dst).content-desc.txt"
  [System.IO.File]::WriteAllLines($txtPath, $out, (New-Object System.Text.UTF8Encoding($false)))
  Write-Output "写出 $($j.dst).content-desc.txt（text $($texts.Count) 条 / desc $($descs.Count) 条）"

  # PNG -> JPEG（宽 720、q75，与上一批同口径；本机无 WebP 编码器）
  $png = Join-Path $cap "$($j.src).png"
  if (Test-Path $png) {
    try {
      Add-Type -AssemblyName System.Drawing -ErrorAction Stop
      $img = [System.Drawing.Image]::FromFile($png)
      $w = 720; $h = [int]([Math]::Round($img.Height * $w / $img.Width))
      $bmp = New-Object System.Drawing.Bitmap($w, $h)
      $g = [System.Drawing.Graphics]::FromImage($bmp)
      $g.InterpolationMode = [System.Drawing.Drawing2D.InterpolationMode]::HighQualityBicubic
      $g.DrawImage($img, 0, 0, $w, $h)
      $codec = [System.Drawing.Imaging.ImageCodecInfo]::GetImageEncoders() | Where-Object { $_.MimeType -eq 'image/jpeg' }
      $ep = New-Object System.Drawing.Imaging.EncoderParameters(1)
      $ep.Param[0] = New-Object System.Drawing.Imaging.EncoderParameter([System.Drawing.Imaging.Encoder]::Quality, 75L)
      $jpg = Join-Path $repo "$($j.dst).jpg"
      $bmp.Save($jpg, $codec, $ep)
      $g.Dispose(); $bmp.Dispose(); $img.Dispose()
      $kb = [int]((Get-Item $jpg).Length / 1KB)
      Write-Output "写出 $($j.dst).jpg（${w}x${h}, ${kb} KB）"
    } catch {
      Copy-Item $png (Join-Path $repo "$($j.dst).png") -Force
      Write-Output "JPEG 转换失败（$($_.Exception.Message)）⇒ 退回落 PNG"
    }
  }
}

# logcat：实数统计（不写「E=0」）
$pidRaw = (& $adb -s $dev shell "pidof io.dcloud.uniappx" 2>&1) -join ' '
$appPid = ($pidRaw -split '\s+')[0].Trim()
$log = Get-Content (Join-Path $cap 'logcat-upload.txt') -Raw
$lines = $log -split "`n"
$eAll = @($lines | Where-Object { $_ -match ' E ' })
$appE = @()
if ($appPid) { $appE = @($eAll | Where-Object { $_ -match "\s$appPid\s" }) }
$kw = 'forum|markdown|toolbar|image|picker|upload|input|textarea|chooseImage'
$appEHit = @($appE | Where-Object { $_ -match $kw })
$lo = New-Object System.Collections.Generic.List[string]
$lo.Add("# ①a 采数窗口的 logcat（**先 `adb logcat -c` 再取数**：窗口 = 进入相册选择 → 选 1 张 → 完成 → 回应用上传成功）")
$lo.Add("# 取数：adb logcat -d（全量捕获，非 grep 后的残片）")
$lo.Add("")
$lo.Add("总行数 = $($lines.Count)")
$lo.Add("E 级行（全量，含系统噪声）= $($eAll.Count)")
$lo.Add("app 进程（pid=$appPid）E 级行 = $($appE.Count)")
$lo.Add("其中命中本次改动关键词（$kw）的条数 = $($appEHit.Count)")
$lo.Add("")
$lo.Add("## app 进程 E 级行逐条")
if ($appE.Count -eq 0) { $lo.Add("（无）") } else { foreach ($l in $appE) { $lo.Add($l.TrimEnd()) } }
[System.IO.File]::WriteAllLines((Join-Path $repo 'logcat.txt'), $lo, (New-Object System.Text.UTF8Encoding($false)))
Write-Output "写出 logcat.txt（总 $($lines.Count) 行 / E 级 $($eAll.Count) / app E $($appE.Count) / 关键词命中 $($appEHit.Count)）"
