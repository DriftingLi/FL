for ($i = 1; $i -le 300; $i++) {
    if ($i -eq 150) { Write-Output 'MIDDLE-MARKER 导出 android 成功，路径为：D:\x' }
    Write-Output "LINE-$i"
}
Write-Output 'LAST-MARKER 导出 android 成功，路径为：D:\y'
# 末行后立刻退出，不 sleep
