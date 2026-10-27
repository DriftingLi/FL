/**
 * 「无代码变更就别再上一遍真机」契约守护（#2027）
 *
 * 立此守护的实测依据（`.ci-verify` 里 12 条历史 `HX_RUN` 机检行，2026-10-27 读取）：
 *   正常增量轮 total 45–161 秒（中位 73 秒）；3/12 次 `exit=env` 吃掉 643 / 906 / 906 秒
 *   = 总墙钟 4514 秒的 **54%**。其中两次 `deployed=false` 的日志里 App 仍在设备上每 30 秒轮询
 *   `/api/notifications/unread-count` ⇒ 部署成立、判据未前进 ⇒ 白等满 `-TimeoutSeconds`。
 *   根因是「代码没变又跑了一遍」。
 *
 * 锁死的形态（谁想"顺手优化"就会红）：
 *   N1 预检必须落成 `scripts/lib/hx-noop.ps1` 的两个函数（纯判定 + 写标记），不许内联进 hx-run.ps1
 *      —— 内联就没法做运行期断言，也无法在别处复用。
 *   N2 **fail-safe 起点是 Noop=$false**：标记缺失/读不出/缺 headSha/git 不可用一律放行。
 *      这条是本次改动**唯一**真正的危险方向：判错成"没活"会把一次真需要的部署挡掉。
 *   N3 `-Force` 逃生门必须存在（重启 App / 重拍截图需要在无变更时重跑）。
 *   N4 必须 dot-source `lib/hx-noop.ps1`。
 *   N5 预检必须在 `Wait-HxFree` **之前**（无活时连锁都不取；与 C7 的"派发 cli 在忙检测之后"不冲突，
 *      因为预检不派发任何 cli）。
 *   N6 预检块内**不得**出现任何 cli 派发（`Invoke-CliStep` / `Start-CliLaunchDetached`）——
 *      否则就直接违反 hxRunContract 的 C7。
 *   N7 预检判红必须是 `-Exit 'env'` + `exit 2`，**不得**走 `-Exit 'ok'`（那是假绿：声称部署成功却没部署）。
 *   N8 成功路径必须写标记（否则预检永远放行，本次改动等于没做）。
 *   N9 `[Console]::OutputEncoding = UTF8` 必须钉死（日志 mojibake 的唯一真源；
 *      实测本文件此前 `OutputEncoding` 出现 0 次，而 dev-finish / style-loop / mp-weixin-check 各 2 次）。
 *
 * 形态沿用本仓既有守护测试（见 utils/hxRunContract.test.js）：
 *   先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */

const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const LIB_REL = 'scripts/lib/hx-noop.ps1';
const SCRIPT_REL = 'scripts/hx-run.ps1';

function readSource(rel) {
  return readText(path.join(ROOT, rel));
}

/** 掩掉 `<# ... #>` 文档块（块内字符替换成空格，偏移不变）。头注释允许叙述性字面量。 */
function maskDocBlocks(text) {
  return text.replace(/<#[\s\S]*?#>/g, (m) => m.replace(/[^\n]/g, ' '));
}

/** 纯函数：源码文本 → 违规清单 */
function scanNoopContract(sources) {
  const violations = [];
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };

  const lib = maskDocBlocks(sources.lib || '');
  const script = maskDocBlocks(sources.script || '');

  // ---- N1 判定必须落在 lib 的两个函数里 ----
  must(/function\s+Test-HxNoopRerun/.test(lib), 'N1', 'lib 缺 Test-HxNoopRerun（纯判定）');
  must(/function\s+Write-HxDeployMarker/.test(lib), 'N1', 'lib 缺 Write-HxDeployMarker（成功路径写标记）');

  // ---- N2 fail-safe：默认 Noop=$false，且标记缺失分支先返回 ----
  must(/\$result\s*=\s*@\{\s*Noop\s*=\s*\$false/.test(lib), 'N2', '预检默认值不是 Noop=$false（fail-safe 起点被改）');
  const missingBranch = lib.indexOf('Test-Path -LiteralPath $MarkerPath') !== -1;
  must(missingBranch, 'N2', 'lib 缺「标记不存在 ⇒ 放行」分支');
  const noopTrueAssign = (lib.match(/Noop\s*=\s*\$true/g) || []).length;
  must(noopTrueAssign === 1, 'N2', `Noop=$true 应只在唯一收口处赋值（实得 ${noopTrueAssign} 处）`);

  // ---- N3 -Force 逃生门 ----
  must(/\[switch\]\$Force/.test(script), 'N3', '缺 -Force 开关（无变更时重跑没有出口）');
  must(/\$CompileOnly -and -not \$Force/.test(script), 'N3', '预检未被 CompileOnly/Force 正确豁免');

  // ---- N4 dot-source ----
  must(/lib[\\/]hx-noop\.ps1/.test(script), 'N4', 'hx-run.ps1 未 dot-source lib/hx-noop.ps1');

  // ---- N5 预检在 Wait-HxFree 之前 ----
  const preIdx = script.indexOf('Test-HxNoopRerun -Project');
  const busyIdx = script.indexOf('Wait-HxFree -CliExe');
  must(preIdx !== -1, 'N5', 'hx-run.ps1 未调用 Test-HxNoopRerun');
  must(busyIdx !== -1, 'N5', 'hx-run.ps1 未调用 Wait-HxFree -CliExe（C3 形态被破坏）');
  must(preIdx !== -1 && busyIdx !== -1 && preIdx < busyIdx, 'N5', '预检排在 Wait-HxFree 之后（无活时也先取锁了）');

  // ---- N6/N7 预检块内不得派发 cli；判红必须 exit 2 + exit=env ----
  const blockStart = script.indexOf('# ---------- 预检');
  must(blockStart !== -1, 'N6', '找不到预检块锚点注释（# ---------- 预检）');
  if (blockStart !== -1 && busyIdx !== -1 && blockStart < busyIdx) {
    const block = script.slice(blockStart, busyIdx);
    ['Invoke-CliStep', 'Start-CliLaunchDetached', 'Invoke-CliLaunch'].forEach((token) => {
      must(!block.includes(token), 'N6', `预检块内出现 cli 派发：${token}（违反 hxRunContract C7：派发必须在忙检测之后）`);
    });
    must(/Write-HxRunResult[^\n]*-Exit 'env'/.test(block), 'N7', "预检判红未走 -Exit 'env'（会变假绿：说成功却没部署）");
    must(/\n\s*exit 2/.test(block), 'N7', '预检判红未 exit 2（环境不可用语义）');
    must(!/-Exit 'ok'/.test(block), 'N7', "预检块内出现 -Exit 'ok'（假绿）");
  }

  // ---- N8 成功路径写标记 ----
  must(/Write-HxDeployMarker -Project \$Project/.test(script), 'N8', '成功路径未写部署标记（预检永远放行 ⇒ 改动等于没做）');
  // ⚠️ 必须用 lastIndexOf：`-Exit 'ok'` 在 `-CompileOnly` 分支里也有一处，且位于标记写入之前
  //    （首跑就是这么红的）。真正的「真运行成功」收口是文件里最后那一处。
  const okIdx = script.lastIndexOf("-Exit 'ok'");
  const markerIdx = script.indexOf('Write-HxDeployMarker -Project $Project');
  must(okIdx !== -1 && markerIdx !== -1 && markerIdx < okIdx, 'N8', '写标记发生在报成功之后（失败轮可能被误记）');

  // ---- N9 编码钉死（日志 mojibake 的唯一真源）----
  must(
    /\[Console\]::OutputEncoding\s*=\s*\[Text\.Encoding\]::UTF8/.test(script),
    'N9',
    '未钉 [Console]::OutputEncoding = UTF8 ⇒ 重定向捕获的 cli 中文输出会按 GBK 解码成 mojibake'
  );

  return violations;
}

function realSources() {
  return { lib: readSource(LIB_REL), script: readSource(SCRIPT_REL) };
}

describe('hx-run 无代码变更预检 + 日志编码（#2027）', () => {
  it('真实文件零违规', () => {
    expect(scanNoopContract(realSources())).toEqual([]);
  });

  it('N1：判定落在 lib，不内联进 hx-run.ps1', () => {
    const s = realSources();
    expect(s.lib).toContain('function Test-HxNoopRerun');
    expect(s.lib).toContain('function Write-HxDeployMarker');
  });

  it('N2：fail-safe —— 默认 Noop=$false，只有唯一收口处才置 $true', () => {
    // 必须掩掉文档块：lib 的头注释里叙述性地写了「变了 ⇒ 该跑（Noop = $false）」这类散文，
    // 不掩码就会把注释当成代码来数（本测试首跑就是这么红的 —— 记在这里，别退回去）。
    const lib = maskDocBlocks(realSources().lib);
    expect(lib).toMatch(/\$result\s*=\s*@\{\s*Noop\s*=\s*\$false/);
    expect((lib.match(/Noop\s*=\s*\$true/g) || []).length).toBe(1);
    expect(lib).toContain('标记缺 headSha');
    expect(lib).toContain('⇒ 照常跑');
  });

  it('N5：预检排在 Wait-HxFree 之前，且不取锁', () => {
    const s = maskDocBlocks(realSources().script);
    const pre = s.indexOf('Test-HxNoopRerun -Project');
    const busy = s.indexOf('Wait-HxFree -CliExe');
    expect(pre).toBeGreaterThan(-1);
    expect(busy).toBeGreaterThan(-1);
    expect(pre).toBeLessThan(busy);
  });

  it('N9：日志编码钉死（乱码根因）', () => {
    const s = realSources().script;
    expect(s).toContain('[Console]::OutputEncoding = [Text.Encoding]::UTF8');
  });

  // ---------------- 负控：注入违规的变形样本必须被检出 ----------------
  describe('负控（防空跑假绿）', () => {
    const base = realSources();

    it('N2 被破坏 ⇒ 检出', () => {
      // 锚到**代码**里那处赋值（`$result = @{ Noop = $false`），不锚文档块里的散文
      const broken = { ...base, lib: base.lib.replace('$result = @{ Noop = $false', '$result = @{ Noop = $true') };
      expect(scanNoopContract(broken).join('\n')).toContain('N2');
    });

    it('N3 去掉 -Force ⇒ 检出', () => {
      const broken = { ...base, script: base.script.replace('[switch]$Force,', '') };
      expect(scanNoopContract(broken).join('\n')).toContain('N3');
    });

    it('N5 把预检挪到 Wait-HxFree 之后 ⇒ 检出', () => {
      const s = base.script;
      const start = s.indexOf('# ---------- 预检');
      const busy = s.indexOf('$hx = Wait-HxFree -CliExe');
      expect(start).toBeGreaterThan(-1);
      expect(busy).toBeGreaterThan(-1);
      const moved = s.slice(0, start) + s.slice(busy) + s.slice(start, busy);
      expect(scanNoopContract({ ...base, script: moved }).join('\n')).toContain('N5');
    });

    it('N6 在预检块里派发 cli ⇒ 检出', () => {
      const broken = {
        ...base,
        script: base.script.replace(
          '        Write-HxRunResult -Mode $mode -Compile 0',
          "        Invoke-CliStep -Name 'open' -CliArgs @('open')\n        Write-HxRunResult -Mode $mode -Compile 0"
        ),
      };
      expect(scanNoopContract(broken).join('\n')).toContain('N6');
    });

    it('N8 去掉写标记 ⇒ 检出', () => {
      const broken = { ...base, script: base.script.replace('Write-HxDeployMarker -Project $Project', 'Write-Nothing') };
      expect(scanNoopContract(broken).join('\n')).toContain('N8');
    });

    it('N9 去掉编码钉死 ⇒ 检出', () => {
      const broken = {
        ...base,
        script: base.script.replace('try { [Console]::OutputEncoding = [Text.Encoding]::UTF8 } catch { }', ''),
      };
      expect(scanNoopContract(broken).join('\n')).toContain('N9');
    });
  });
});
