/**
 * HBuilderX「忙检测 + 等待上限 + agent 互斥锁」契约守护（scripts/lib/hx-busy.ps1 + 四个门脚本）
 *
 * 背景（2026-09-12 维护者追加）：HBuilderX 是**单实例串行资源**——`cli.exe` 只驱动**同一个主程序**，
 * `publish` / `launch` 等重活都要排进主程序的编译队列。维护者用 GUI 编译/运行时，agent 的门脚本若并发
 * 发起，既拖慢维护者、也会因排队产生**假失败**（实测：publish 被排在另一个会话的
 * `launch app-android --compile true` 之后，输出停在「正在编译中...」不返回）。
 * 故四个需要 HBuilderX 的门脚本统一 dot-source `scripts/lib/hx-busy.ps1`：
 *   ④a scripts/compile-check.ps1 · ④c scripts/kotlin-all-check.ps1（publish 段）· ② scripts/mp-weixin-check.ps1
 *   （仿真机冒烟不接这条：纯 adb/emulator，不依赖 HBuilderX）
 *
 * 守护的不变量：
 *   H1 helper 存在且提供 Acquire/Release/Test/Wait 四个函数与锁路径
 *   H2 四个脚本都 dot-source 了 helper
 *   H3 任何脚本都不得出现 kill 主程序的调用（HBuilderX 是共享资源，**绝不抢占**）
 *   H4 忙/超时路径是 `exit 2`（环境不可用），不是 `exit 1`
 *   H5 锁在 `finally` 路径里释放
 *   H6 限制写实：helper 头注释必须写明「机械上无法可靠探测 GUI 是否正在编译」
 *   H7 锁的陈旧判定（30 分钟）与 `-NoWait` 逃生都实现且写进注释
 *   H8 HX_BUSY 结果行（便于日志/评论里核对实际等待时长）
 *   H9 ADR-0008 已记录「HBuilderX 单实例/串行」坑位 + 「跑完 HBuilderX 门后 git status 看 manifest 是否被改脏」
 *   H12 陈旧锁必须查「持有者 pid 是否还活着」（#974，2026-09-14 实测白等 ~25 分钟）：锁文件里本来就存着 pid，
 *       旧判据只比 mtime ⇒ 持有会话被强杀后仍要白等到 30 分钟。**优先级不可颠倒**：只有**确定**找不到该 pid
 *       才判死，拿不准（非数字 / 权限）一律回落 30 分钟时间线 —— 绝不放行活会话
 *
 * 设计沿用本仓既有守护测试形态（见 utils/kotlinAllGateContract.test.js）：先对注入违规断言检测有效，再对真实文件断言零命中。
 *
 * **未实测声明（照实记）**：本 helper 的「忙」判定、锁与陈旧抢占**在本会话中只做了不接 HBuilderX 的单元级验证**
 * （lock 占用 / 陈旧抢占 / -NoWait 立即 exit 2 三条路径已实测通过）；「维护者 GUI 正在编译时脚本是否真的等到/退出 2」
 * **需在维护者空出 HBuilderX 后再复测**（见 ADR-0008 与 PR 正文的「待跑」档）。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const HELPER_REL = 'scripts/lib/hx-busy.ps1';
const ADR_REL = 'docs/adr/0008-移动端验收门与证据.md';
// 四个需要 HBuilderX 的门脚本（② / ④a / ④c）
const HX_SCRIPTS = ['scripts/compile-check.ps1', 'scripts/kotlin-all-check.ps1', 'scripts/mp-weixin-check.ps1'];

function readSource(rel) {
  return readText(path.join(ROOT, rel));
}

/** 纯函数：helper + 各门脚本 + ADR → 违规清单 */
function scanContract(sources) {
  const violations = [];
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };
  const helper = sources.helper || '';
  const adr = sources.adr || '';
  const scripts = sources.scripts || {};

  // H1 helper 的接口
  must(helper.includes('function Acquire-HxLock'), 'H1', 'helper 缺 Acquire-HxLock');
  must(helper.includes('function Release-HxLock'), 'H1', 'helper 缺 Release-HxLock');
  must(helper.includes('function Test-HxResponsive'), 'H1', 'helper 缺 Test-HxResponsive');
  must(helper.includes('function Wait-HxFree'), 'H1', 'helper 缺 Wait-HxFree');
  must(helper.includes('hx-agent.lock'), 'H1', 'helper 未使用 $env:TEMP\hx-agent.lock 作为互斥锁');
  must(helper.includes('HxLockStaleMinutes = 30'), 'H1', 'helper 未把陈旧判定写死为 30 分钟');

  // H2 每个门脚本都 dot-source
  Object.entries(scripts).forEach(([rel, text]) => {
    must(/(lib\/hx-busy\.ps1|lib\\hx-busy\.ps1)/.test(text || ''), 'H2', rel + ' 未 dot-source scripts/lib/hx-busy.ps1');
    must(/Wait-HxFree/.test(text || ''), 'H2', rel + ' 未调用 Wait-HxFree');
  });

  // H3 绝不 kill 主程序（连注释里出现该字面量也不允许，免得被误读为手段）
  [['lib/hx-busy.ps1', helper], ...Object.entries(scripts)].forEach(([rel, text]) => {
    must(!/Stop-Process[^\r\n]*HBuilderX/.test(text || ''), 'H3', rel + ' 出现 kill 主程序的调用（绝不抢占共享主程序）');
  });

  // H4 忙/超时是 exit 2
  const helperExit2 = (helper.match(/exit 2/g) || []).length;
  must(helperExit2 >= 2, 'H4', 'helper 的忙/超时路径未判为 exit 2（应为环境不可用，>=2 处）');
  must(!/exit 1/.test(helper), 'H4', 'helper 里出现 exit 1（忙/超时必须是 exit 2，exit 1 语义是「门未过」）');

  // H5 锁在 finally 里释放
  Object.entries(scripts).forEach(([rel, text]) => {
    const t = text || '';
    const finallyAt = t.lastIndexOf('} finally {');
    must(finallyAt !== -1, 'H5', rel + ' 缺 finally 路径（锁必须在那里释放）');
    if (finallyAt !== -1) must(/Release-HxLock/.test(t.slice(finallyAt)), 'H5', rel + ' 的 finally 路径里没有 Release-HxLock');
  });

  // H6 限制写实
  must(helper.includes('无法可靠探测'), 'H6', 'helper 未写明「机械上无法可靠探测 GUI 是否正在编译」这一限制');
  must(helper.includes('单实例串行'), 'H6', 'helper 未写明 HBuilderX 是单实例串行资源');

  // H7 -NoWait 逃生
  must(helper.includes('[switch]$NoWait'), 'H7', 'helper 缺 -NoWait 逃生开关');

  // H8 HX_BUSY 结果行
  must(helper.includes('HX_BUSY wait='), 'H8', 'helper 未输出 HX_BUSY wait=<秒> result=<free|timeout> 结果行');

  // H9 ADR 落锁
  must(adr.includes('单实例'), 'H9', 'ADR-0008 未记录「HBuilderX 单实例/串行」坑位');
  must(adr.includes('hx-busy.ps1'), 'H9', 'ADR-0008 未记录 hx-busy.ps1 这个共享载体');
  must(adr.includes('git status'), 'H9', 'ADR-0008 未写「跑完 HBuilderX 门后先 git status 看 manifest 是否被改脏」');

  // H10 锁交接（2026-09-14 加）：dev-finish 要在**一个临界区**里连跑三个需要 HBuilderX 的步骤
  // （编译门 → 真运行部署 → 逐页截图），若它自己持锁再调子脚本，子进程会看到父进程的新鲜锁
  // 而一直等到超时 → **自死锁**。故引入 `$env:HX_LOCK_OWNER` 交接：父持锁后把 PID 写进 env，
  // 子脚本认到「同一个持有者」就直接复用，不再加锁。
  must(helper.includes('HX_LOCK_OWNER'), 'H10', 'helper 未实现 $env:HX_LOCK_OWNER 锁交接（dev-finish 持锁时会自死锁）');
  must(helper.includes('function Test-HxLockInherited'), 'H10', 'helper 缺 Test-HxLockInherited');
  must(helper.includes('result=inherited'), 'H10', 'helper 未输出 HX_BUSY ... result=inherited 交接行');

  // H11 交接必须 fail-safe：**只有 env 里的 PID 与锁文件里的 PID 一致**才允许跳过加锁。
  // 否则父进程崩溃后 env 残留（或被子进程继承到无关场景）会让后续脚本**绕过互斥锁**，
  // 直接回到「并发抢主程序 → 假失败」那个坑里。
  must(helper.includes('-eq "$($env:HX_LOCK_OWNER)"'), 'H11', 'helper 的交接未比对「env PID == 锁文件 PID」（照抄即放开互斥锁）');

  // H12 陈旧锁必须查「持有者是否还活着」（#974 现象一，2026-09-14 实测白等 ~25 分钟）：
  // 锁文件里本来就存着 pid（Acquire 写入 "$PID"），旧判据手里有更强的信号却只比 mtime。
  // **优先级不可颠倒**：先判「确定已死」，否则回落时间线；拿不准（非数字 / 权限）必须回落，绝不放行活会话。
  must(helper.includes('function Test-HxProcessAlive'), 'H12', 'helper 缺持有者存活探测 Test-HxProcessAlive（锁里存着 pid 却没用）');
  must(
    helper.includes('catch [System.ArgumentException]'),
    'H12',
    'helper 未把「确定找不到该 pid」与「其它查询错误」分开（混为一谈会把权限错误当成持有者已死）'
  );
  must(
    helper.includes('Test-HxProcessAlive -PidText $Info.Pid'),
    'H12',
    'Test-HxLockStale 未查持有者存活（陈旧判据仍只看时间）'
  );
  must(
    /\(Get-Date\) - \$Info\.MTime\)\.TotalMinutes\s*-gt\s*\$script:HxLockStaleMinutes/.test(helper),
    'H12',
    '陈旧判定丢了「超过 30 分钟」这条时间线回落（新判据会把拿不准的锁也放行）'
  );
  const aliveProbeAt = helper.indexOf('Test-HxProcessAlive -PidText $Info.Pid');
  const timeLineAt = helper.indexOf('-gt $script:HxLockStaleMinutes');
  must(
    aliveProbeAt !== -1 && timeLineAt !== -1 && aliveProbeAt < timeLineAt,
    'H12',
    '「持有者已死」判定没有排在时间线之前（优先级颠倒 ⇒ 新判据形同虚设）'
  );

  return violations;
}

describe('HBuilderX 忙检测契约（单实例串行资源，2026-09-12）', () => {
  const real = {
    helper: readSource(HELPER_REL),
    adr: readSource(ADR_REL),
    scripts: Object.fromEntries(HX_SCRIPTS.map((r) => [r, readSource(r)]))
  };

  it('自检：注入违规必须被检出（防空跑假绿）', () => {
    const first = HX_SCRIPTS[0];
    const second = HX_SCRIPTS[1];
    const cases = [
      ['H1', { ...real, helper: real.helper.replace(/function Wait-HxFree/g, 'function Wait-HxX') }],
      ['H1', { ...real, helper: real.helper.replace(/HxLockStaleMinutes = 30/g, 'HxLockStaleMinutes = 90') }],
      ['H2', { ...real, scripts: { ...real.scripts, [first]: real.scripts[first].replace(/hx-busy\.ps1/g, 'other.ps1') } }],
      ['H2', { ...real, scripts: { ...real.scripts, [second]: real.scripts[second].replace(/Wait-HxFree/g, 'WaitNothing') } }],
      ['H3', { ...real, scripts: { ...real.scripts, [first]: real.scripts[first] + '\nStop-Process -Name HBuilderX -Force\n' } }],
      ['H4', { ...real, helper: real.helper.replace(/exit 2/g, 'exit 1') }],
      ['H5', { ...real, scripts: { ...real.scripts, [first]: real.scripts[first].replace(/Release-HxLock/g, 'ReleaseNothing') } }],
      ['H6', { ...real, helper: real.helper.replace(/无法可靠探测/g, '可以精确探测') }],
      ['H7', { ...real, helper: real.helper.replace(/\[switch\]\$NoWait/g, '[switch]$WaitForever') }],
      ['H8', { ...real, helper: real.helper.replace(/HX_BUSY wait=/g, 'BUSY wait=') }],
      ['H9', { ...real, adr: real.adr.replace(/单实例/g, '多实例') }],
      ['H9', { ...real, adr: real.adr.replace(/git status/g, 'git diff') }],
      // H10/H11：交接机制与其 fail-safe 分开注入，确保两条规则各自有效
      ['H10', { ...real, helper: real.helper.replace(/HX_LOCK_OWNER/g, 'HX_LOCK_OTHER') }],
      ['H11', { ...real, helper: real.helper.replace(/-eq "\$\(\$env:HX_LOCK_OWNER\)"/g, '-ne ""') }],
      // H12：存活判据（#974）—— 每条各自注入，避免某一条失效而其余掩盖它
      ['H12', { ...real, helper: real.helper.replace(/function Test-HxProcessAlive/g, 'function X') }],
      ['H12', { ...real, helper: real.helper.replace(/catch \[System\.ArgumentException\]/g, 'catch {') }],
      ['H12', { ...real, helper: real.helper.replace('Test-HxProcessAlive -PidText $Info.Pid', '$false') }],
      ['H12', { ...real, helper: real.helper.replace('-gt $script:HxLockStaleMinutes', '-gt 0') }],
      ['H12', {
        ...real,
        helper: real.helper.replace(
          'function Test-HxLockStale {',
          'function Test-HxLockStale {\n    try { if (((Get-Date) - $Info.MTime).TotalMinutes -gt $script:HxLockStaleMinutes) { return $true } } catch { }'
        )
      }]
    ];
    // 注入一律用**全局**替换（/…/g）：判据多用 includes 判「存在」，若目标文本在文件里有第二处，
    // 只替换第一处会让注入静默失效 ⇒ 自检假绿。2026-09-13 实测踩中：ADR-0008 新增一句
    // 「单实例串行资源」使「单实例」出现两处，H9 的注入随之失效。
    cases.forEach(([rule, sources]) => {
      const found = scanContract(sources);
      expect(found.some((v) => v.startsWith(rule))).toBe(true);
    });
  });

  it('真实文件：零违规', () => {
    expect(scanContract(real)).toEqual([]);
  });

  it('H3：四个脚本都不得 kill 主程序（HBuilderX 是共享资源）', () => {
    Object.values(real.scripts).forEach((text) => {
      expect(text).not.toMatch(/Stop-Process[^\r\n]*HBuilderX/);
    });
    expect(real.helper).not.toMatch(/Stop-Process[^\r\n]*HBuilderX/);
  });

  it('H5：② / ④a / ④c 的 finally 路径都释放锁', () => {
    Object.entries(real.scripts).forEach(([rel, text]) => {
      const finallyAt = text.lastIndexOf('} finally {');
      expect(finallyAt).toBeGreaterThan(-1);
      expect(text.slice(finallyAt)).toContain('Release-HxLock');
    });
  });

  it('H12：陈旧锁先查持有者存活，再回落 30 分钟时间线（#974）', () => {
    expect(real.helper).toContain('function Test-HxProcessAlive');
    expect(real.helper).toContain('catch [System.ArgumentException]');
    const aliveAt = real.helper.indexOf('Test-HxProcessAlive -PidText $Info.Pid');
    const timeAt = real.helper.indexOf('-gt $script:HxLockStaleMinutes');
    expect(aliveAt).toBeGreaterThan(-1);
    expect(timeAt).toBeGreaterThan(-1);
    expect(aliveAt).toBeLessThan(timeAt); // 优先级：先判死，再回落时间线
    // 时间线必须**还在**（#974 是追加一条判据，不是替代它）
    expect(real.helper).toMatch(/\(Get-Date\) - \$Info\.MTime\)\.TotalMinutes\s*-gt\s*\$script:HxLockStaleMinutes/);
    expect(real.helper).toContain('HxLockStaleMinutes = 30');
  });
});
