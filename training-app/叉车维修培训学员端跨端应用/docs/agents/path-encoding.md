# 路径编码四条边界（中文目录名）

移动端项目目录名保持中文（裁定见根 [`ADR-0106`](../../../../docs/adr/ADR-0106-中文目录名保留与编码面收口.md)，不属移动端 ADR 体系）。中文名在 Windows 链路上必过四类编码面，每条一句「症状 → 解」。**解码唯一真源** = [`scripts/lib/level-detect.ps1`](../../scripts/lib/level-detect.ps1) 的 `ConvertFrom-GitQuotedPath`；调用点归属表 = `utils/levelDetectContract.test.js` 的 `GIT_PATH_REGISTRY`（守卫 `utils/ps1GitPathScan.js`）。**别在调用点复制解码实现。**

## 1. git `core.quotepath` 转义
**症状**：`git diff --name-only` / `ls-files` / `status --porcelain` 把含非 ASCII 的路径整条包引号 + `\ooo` 八进制 ⇒ `-match '\.uvue$'`、`^pages/` 静默失配；#1853 的改动页清单恒空，门静默判绿。
**解**：二选一——① `-c core.quotepath=false`（只免非 ASCII，引号与反斜杠仍会转义）；② 过 `ConvertFrom-GitQuotedPath` 解码。**要按仓库根锚定（`^pages/`）时 `--relative` 必须与解码成对出现**：实测仅 `--relative` 未解码 = 0 页、两者齐备 = 1 页（#2874）。`git rev-parse --show-toplevel` 与 `git worktree list --porcelain` 不吃 quotepath，不必解码（归属表 `allow-quotepath-immune` 档）。

## 2. pwsh 控制台码页
**症状**：`[Console]::OutputEncoding` 默认 gb2312 ⇒ 子进程（jest / cli / node）的 UTF-8 stdout 进内存即按 GBK 解码，中文用例名与 `●` 变乱码（#1526）；重定向日志再读回时不给 `-Encoding`，同样按宿主默认解（#2027）。
**解**：入口钉 `[Console]::OutputEncoding = UTF8` 与 `$OutputEncoding = UTF8`（Process 起子进程再用 `StandardOutputEncoding=UTF8`）；日志读回显式 `-Encoding utf8`；机读判据行只输出 ASCII，中文全文落 UTF-8 文件（先例 `scripts/wireless-debug.ps1` 的 `-Ascii` 形态）。

## 3. JVM `file.encoding`
**症状**：Corretto 17 在中文路径下把 `file.encoding` 取成 OEM 码页（本机 GB18030），kotlinc 的中文诊断按该码页写 stdout，采集层按 UTF-8 解 ⇒ 不可解码字节烤成 U+FFFD，证据日志落盘不可逆（#2161：fffd 18 处 → 0）。
**解**：调 java / kotlinc 前加 `-Dfile.encoding=UTF-8`，与 ①② 两条同批钉死（唯一真源 `scripts/lib/evidence-selfcheck.ps1` 的 `Get-KotlinJvmUtf8Args`）。不用 `-Dstdout.encoding` / `-Dstderr.encoding`（JDK 18+ 才认），不用 `JAVA_TOOL_OPTIONS`（会往 stderr 打 Picked up 污染产物）。

## 4. adb 链路的非 ASCII 判据
**症状**：adb 的错误原文经 pwsh（OEM 码页）写 stdout，到 Node 侧成 `\uFFFD\u0492\u04B2\u04B2 adb.exe` 一类字节 ⇒ `!/找不到 adb\.exe/` 恒不匹配，任何错误都判成「解析到了」，用例假绿（#1027）。
**解**：判据不在 JS 里匹配中文——布尔/ASCII token 在 pwsh 侧算好（`_ADB_MISSING` / `SHARED_FOUND`），JS 只比 `True` / `False`；adb 路径解析唯一真源 `scripts/lib/env-check.ps1` 的 `Resolve-AdbExeLocal`；`install` / `push` 成败看 ASCII token（`Success` / `pushed|skipped`），不看中文错误。

## 改脚本前的三步
1. **输出 git 路径？先归类**：按 `GIT_PATH_REGISTRY` 五档（`quotepath-off` / `decoded` / `allow-count` / `allow-quotepath-immune` / `allow-narrative`）登记新调用点，否则 L9 判红。
2. **跑一次契约腿**：`npx jest --config jest.config.unit.js utils/levelDetectContract.test.js`（守卫真红比对付红便宜）。
3. **要动解码逻辑**：只改 `scripts/lib/level-detect.ps1` 一处，并同批跑 `utils/levelDetectQuotepathBehavior.test.js`。
