# 决定性最小对照：验证 *.uts / *.uvue text eol=lf 的真实效果
# 判据：模拟 Windows（core.autocrlf=true）检出后，工作区文件里有没有 CR
$ErrorActionPreference = 'Stop'
$base = "D:\FL\.tmp-1137-proof"
Remove-Item -Recurse -Force $base -ErrorAction SilentlyContinue
New-Item -ItemType Directory -Force -Path $base | Out-Null

function New-Repo([string]$name, [bool]$withRule) {
  $r = Join-Path $base $name
  New-Item -ItemType Directory -Force -Path "$r\training-app\utils" | Out-Null
  # 内容用 LF 存进库里（与真实仓库一致：主干 blob 全是 i/lf）
  [System.IO.File]::WriteAllText("$r\training-app\utils\format.uts", "const a = 1`nconst b = 2`nconst c = 3`n")
  [System.IO.File]::WriteAllText("$r\training-app\utils\thing.uvue", "<template>`n  <view />`n</template>`n")
  [System.IO.File]::WriteAllText("$r\README.md", "# doc`nline`n")
  if ($withRule) {
    [System.IO.File]::WriteAllText("$r\.gitattributes", "# doc`n*.md text eol=lf`n`n# uni-app-x 源码统一 LF`n*.uts text eol=lf`n*.uvue text eol=lf`n")
  }
  Push-Location $r
  git init -q 2>&1 | Out-Null
  git config user.email "t@t.t"; git config user.name "t"
  git config core.autocrlf false      # 入库时按原样（LF）存
  git add -A 2>&1 | Out-Null
  git -c core.autocrlf=false commit -q -m "init" 2>&1 | Out-Null
  Pop-Location
}

# 克隆并强制 Windows 行为（autocrlf=true），然后看工作区真实字节
function Test-Clone([string]$name) {
  $src = Join-Path $base $name
  $dst = Join-Path $base "$name-clone"
  git clone -q $src $dst 2>&1 | Out-Null
  Push-Location $dst
  git config core.autocrlf true
  Remove-Item -Recurse -Force "$dst\training-app" -ErrorAction SilentlyContinue
  git checkout -- training-app 2>&1 | Out-Null
  $res = @()
  foreach ($rel in @("training-app\utils\format.uts", "training-app\utils\thing.uvue", "README.md")) {
    $p = Join-Path $dst $rel
    $b = [System.IO.File]::ReadAllBytes($p)
    $cr = ($b | Where-Object { $_ -eq 13 }).Count
    $lf = ($b | Where-Object { $_ -eq 10 }).Count
    $res += [pscustomobject]@{ file = (Split-Path $rel -Leaf); CR = $cr; LF = $lf; verdict = $(if ($cr -gt 0) { "CRLF（会被多行锚点匹配失配）" } else { "LF ✅" }) }
  }
  Pop-Location
  return $res
}

New-Repo "before" $false
New-Repo "after"  $true

"=== A) 无 *.uts/*.uvue 规则（= 当前主干），autocrlf=true 检出 ==="
Test-Clone "before" | Format-Table -AutoSize | Out-String
"=== B) 有 *.uts/*.uvue text eol=lf（= 本 PR），autocrlf=true 检出 ==="
Test-Clone "after" | Format-Table -AutoSize | Out-String
"=== 附加：after 场景里 .uts 的属性判定 ==="
Push-Location (Join-Path $base "after-clone")
git check-attr text eol -- "training-app/utils/format.uts"
Pop-Location
