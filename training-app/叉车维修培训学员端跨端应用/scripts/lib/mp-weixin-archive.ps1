<#
 ② 门「截图入库」的判据面 —— **单点真源**（2026-09-23）。

 为什么单独成库：入库函数里有一处**空安全**陷阱，属「不抽出来就只能靠人眼」的那类 ——

   `Measure-Object` 对**空管道不产出对象**（不是产出 Sum=0 的对象！）⇒ 直接取 `.Sum` 在
   `Set-StrictMode -Version Latest` 下抛「在此对象上找不到属性"Sum"」。
   实测场景：探针没拿到截图（失败路径）⇒ `$shots` / `$made` 为空 ⇒ `Publish-ScreenshotArchive` 在
   while 条件那一行就抛，被调用点的 try/catch 兜成 `[warn] 截图入库失败（不影响门结论）`。
   它不影响门结论（C19 的 try/catch 是故意的），但把「入库」整段打成了异常 —— 属真缺陷。

 谁测它：`utils/mpWeixinArchiveSumBehavior.test.js` **真执行**本文件（dot-source 后调函数），
 而不是断言门脚本的源码文本 —— 文本断言在「判据被摘掉、只留一句字面量」时照样绿（本仓 C20 的教训）。
#>

<#
 入库图合计字节（**空安全**）。
 @param Items 形如 `@{ Path=…; Bytes=… }` 的集合（可为空 / $null）
 @returns [int] 合计字节；空集合返回 0（**不抛**）
#>
function Get-MadeTotalBytes {
    param($Items)
    $sum = @($Items | Measure-Object -Property Bytes -Sum)
    if ($sum.Count -gt 0) { return [int]$sum[0].Sum }
    return 0
}
