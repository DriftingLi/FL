/**
 * ② 微信开发者工具门（scripts/mp-weixin-check.ps1 + scripts/mp-weixin-probe.mjs）契约守护
 *
 * 背景：#883 spike 证实 ② 可完全无人值守执行（HBuilderX `publish mp-weixin` 构建并自己拉起开发者工具
 * → `cli.bat auto` 开自动化端口 → `miniprogram-automator` 连上读 pageStack / console / exceptions 并截图）。
 * 2026-09-12 修订把 ② 从**人工门**降为**半自动门**（与 ④a 同款口径，#870 裁定 B）。它的价值全押在
 * 下面几条不变量上，本测试就是守这几条：
 *   C1 输入 = HBuilderX CLI 构建产物（unpackage\dist\build\mp-weixin），不另造构建；探针在同目录 scripts/
 *   C2 顺序写死 `cli.bat close` → `cli.bat auto`；**auto 必须等它跑完**（中途 kill ⇒ 端口永不监听，2026-09-12 实测）
 *   C3 「pageStack 非空」必须是**前置断言**；探针不得用元素级 API（page.$() 会挂起 15s，且会触发 `$(` 检测）
 *   C4 判成败只看输出：不得出现 $LASTEXITCODE；必须解析 MP_WEIXIN_PROBE / MP_WEIXIN_RESULT 与 publish 文案
 *   C5 端口监听面风险必须写明（实测绑 `::` 通配地址，CLI 无法收窄），且必须轮询端口延迟出现
 *   C6 结果评论格式 == `gate-evidence:②` + `commit: <sha>`；只在门通过（exit 0）分支贴
 *   C7 `cli.bat close` 在 finally 路径里执行 + 幂等保护（避免端口/会话残留）
 *   C8 非等价声明 + 挡不住的类别 + **源 manifest appid 前置断言**（HBuilderX 首次导入会回写 manifest 置 null）
 *   C9 截图入库纪律（P2）：WebP→JPEG 回退、宽 ≤720、单张 ≤150KB、合计 ≤1.5MB、-NoArchive、失败不阻塞门
 *   C10 注册与文档：package.json 脚本、ADR-0008、PR 模板、移动端 AGENTS.md 的人工门清单
 * 设计沿用本仓既有守护测试的形态（见 utils/kotlinAllGateContract.test.js）：
 * 先对「注入违规」的变形样本断言检测有效（防空跑假绿），再对真实文件断言零命中。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const SCRIPT_REL = 'scripts/mp-weixin-check.ps1';
const PROBE_REL = 'scripts/mp-weixin-probe.mjs';
const PKG_REL = 'package.json';
const ADR_REL = 'docs/adr/0008-移动端验收门与证据.md';
const AGENTS_REL = 'AGENTS.md';
const PR_TEMPLATE_REL = path.join('..', '..', '.github', 'PULL_REQUEST_TEMPLATE.md');

function readSource(rel) {
  return fs.readFileSync(path.join(ROOT, rel), 'utf8');
}

/** 纯函数：六份源码文本 → 违规清单 */
function scanContract(sources) {
  const violations = [];
  const s = sources.script || '';
  const probe = sources.probe || '';
  const pkg = sources.pkg || '';
  const adr = sources.adr || '';
  const agents = sources.agents || '';
  const template = sources.template || '';
  const must = (cond, rule, msg) => { if (!cond) violations.push(rule + ' ' + msg); };

  // C1 输入/产物路径
  must(s.includes("$DistRelative = 'unpackage\\dist\\build\\mp-weixin'"), 'C1', '未把输入口径写死为 unpackage\\dist\\build\\mp-weixin');
  must(s.includes("$ProbeRelative = 'scripts\\mp-weixin-probe.mjs'"), 'C1', '未引用同目录探针 scripts\\mp-weixin-probe.mjs');

  // C2 close → auto 的顺序；auto 必须等它跑完（不得被 kill）
  const closeAt = s.indexOf("'close', '--project'");
  const autoAt = s.indexOf("'auto', '--project'");
  must(closeAt !== -1, 'C2', '缺少 `cli.bat close`（清残留自动化会话）');
  must(autoAt !== -1, 'C2', '缺少 `cli.bat auto --auto-port`（开自动化端口）');
  if (closeAt !== -1 && autoAt !== -1) must(closeAt < autoAt, 'C2', '`close` 未出现在 `auto` 之前（顺序错了会撞 pageStack 空）');
  must(/Invoke-Process -FilePath \$devTools -Arguments @\('auto'/.test(s), 'C2', 'auto 未走 Invoke-Process（必须等它跑完；kill 掉会让端口永不监听）');
  must(s.includes('$auto.TimedOut'), 'C2', 'auto 缺超时兜底');

  // C3 pageStack 前置断言 + 禁止元素级 API
  must(probe.includes('pageStackNonEmpty'), 'C3', '探针缺 pageStack 非空前置断言');
  must(probe.includes('getPageMetaByWebviewId'), 'C3', '探针未记录 pageStack 空导致的误导弹错（#883 坑位 1）');
  probe.split(/\r?\n/).forEach((line, idx) => {
    if (/^\s*\/\//.test(line)) return; // 注释里提 API 名是允许的；本守护守的是「别用」
    if (/\$\(|getElementByXpath/.test(line) && !/pageStack|mp\.pageStack/.test(line)) {
      violations.push('C3 第 ' + (idx + 1) + ' 行用了元素级 API（page.$ 会挂起 15s，#883 坑位 3）');
    }
  });

  // C4 判成败只看输出
  must(!/\$LASTEXITCODE/.test(s), 'C4', '出现 $LASTEXITCODE：CLI 退出码恒为 0，判成败不得依赖退出码');
  must(s.includes('MP_WEIXIN_PROBE'), 'C4', '未解析探针的 MP_WEIXIN_PROBE 结果行（无输出判据）');
  must(s.includes('MP_WEIXIN_RESULT'), 'C4', '缺少 MP_WEIXIN_RESULT 结果行（无法机检成败）');
  must(s.includes('导出微信小程序成功'), 'C4', '未以构建产物文案（导出微信小程序成功）判定 publish 成败');

  // C5 端口监听面风险 + 端口延迟出现要轮询
  must(s.includes('通配'), 'C5', '未写明端口绑定通配地址的风险');
  must(s.includes('监听面'), 'C5', '未写明监听面无法从 CLI 侧收窄（不做加固的声明）');
  must(s.includes('$PortWaitSeconds'), 'C5', '缺端口轮询（auto 跑完后端口是延迟出现的）');

  // C6 评论格式 + 只在通过分支贴
  must(s.includes('gate-evidence:②'), 'C6', '未输出 gate-evidence:② 标记（pr-evidence 认不出这条评论证据）');
  must(s.includes('- commit: $sha'), 'C6', '评论缺 commit 行（无法与 PR head sha 绑定）');
  must(/\$PostToPr/.test(s), 'C6', '缺 -PostToPr 开关（门结果免手抄的入口）');
  const callAt = s.indexOf('Publish-GateComment -PrNumber');
  must(callAt !== -1, 'C6', '未调用 Publish-GateComment');
  if (callAt !== -1) must(s.indexOf('exit 1') < callAt, 'C6', 'exit 1 失败分支出现在贴评论调用之后（失败也会贴评论）');

  // C7 finally 路径 + 幂等
  const finallyAt = s.lastIndexOf('finally {');
  must(finallyAt !== -1, 'C7', '缺 finally 路径');
  if (finallyAt !== -1) must(s.slice(finallyAt).includes('Invoke-DevToolsClose'), 'C7', 'finally 路径里没有 `cli.bat close`（会残留端口/会话）');
  must(s.includes('$script:closed'), 'C7', 'close 未做幂等保护（$script:closed）');

  // C8 非等价声明 + 源 manifest appid 前置断言
  must(s.includes('非等价'), 'C8', '缺「非等价声明」（② ≠ ① 真机门 / ② ≠ ④b 云打包门）');
  must(s.includes('checkIsSupportSoterAuthentication'), 'C8', '未列出挡不住的类别（生物识别实测在开发者工具不可用）');
  must(s.includes('manifest-appid'), 'C8', '源 manifest appid 不匹配时未判门不过（fail-closed 缺一路径）');
  must(/mp-weixin"\\s\*:\\s\*\\{/.test(s) || s.includes('mp-weixin"'), 'C8', '缺源 manifest appid 前置断言（HBuilderX 首次导入会回写 manifest 置 null）');
  must(s.includes('不在 Page 上') || s.includes('Page 上'), 'C8', '未记录 screenshot 挂在 MiniProgram 而非 Page（#883 坑位 2）');

  // C9 截图入库纪律
  must(s.includes("$script:ArchiveMaxCount = 10"), 'C9', '入库张数上限未写死为 10');
  must(s.includes('$script:ArchiveMaxBytes = 150KB'), 'C9', '单张体积上限未写死为 150KB');
  must(s.includes('$script:ArchiveMaxTotal = 1.5MB'), 'C9', '合计体积上限未写死为 1.5MB');
  must(s.includes('$script:ArchiveMaxWidth = 720'), 'C9', '宽度上限未写死为 720');
  must(s.includes('docs/verification/'), 'C9', '入库落点不是 docs/verification/<模块>/<PR号>/');
  must(s.includes('-NoArchive'), 'C9', '缺 -NoArchive 逃生开关');
  must(/Get-WebpEncoder/.test(s) && /cwebp/.test(s) && /ffmpeg/.test(s) && /magick/.test(s), 'C9', '未探测 WebP 编码器（cwebp / ffmpeg / magick）');
  must(s.includes('System.Drawing') || s.includes('[System.Drawing.'), 'C9', '缺 JPEG 回退实现（System.Drawing）');
  must(/Save-Jpeg/.test(s) && /Quality/.test(s), 'C9', '缺 JPEG 质量控制（q75）');
  must(s.includes('不得让门因此失败') || s.includes('不影响门结论'), 'C9', '入库失败未声明「不得让门失败」');
  must(/^.*-after/s.test(s) || s.includes('-after'), 'C9', '产物命名缺 -after 后缀');
  must(s.includes('省了'), 'C9', '减图时未要求在评论里写明省了哪几页');
  must(s.includes('入库截图'), 'C9', '评论未用仓库内相对路径列出入库截图');

  // C11 HBuilderX 忙检测（单实例串行资源）：② 也必须接共享 helper，且不得 kill 主程序
  must(/(lib\/hx-busy\.ps1|lib\\hx-busy\.ps1)/.test(s), 'C11', '未 dot-source scripts/lib/hx-busy.ps1（HBuilderX 忙检测）');
  must(/Wait-HxFree/.test(s), 'C11', '未调用 Wait-HxFree（忙检测 + 等待上限）');
  must(/Release-HxLock/.test(s), 'C11', '未在 finally 里释放 agent 互斥锁');
  must(!/Stop-Process[^\r\n]*HBuilderX/.test(s), 'C11', '出现 kill 主程序的调用（HBuilderX 是共享单实例资源，绝不抢占）');
  must(s.includes('$HxWaitSeconds') && s.includes('$HxNoWait'), 'C11', '缺 -HxWaitSeconds / -HxNoWait 参数');

  // C10 注册与文档
  must(/"build:mp-weixin-check"\s*:\s*"[^"]*scripts\/mp-weixin-check\.ps1"/.test(pkg), 'C10', 'package.json 未注册 build:mp-weixin-check');
  must(adr.includes('半自动'), 'C10', 'ADR-0008 未把 ② 记为半自动门');
  must(adr.includes('#883'), 'C10', 'ADR-0008 未引用 #883');
  must(adr.includes('mp-weixin-check.ps1'), 'C10', 'ADR-0008 未记录 ② 的脚本载体');
  must(template.includes('gate-evidence:②'), 'C10', 'PR 模板未说明 ② 可用 -PostToPr 评论承载');
  must(agents.includes('人工门只剩') || agents.includes('② 微信开发者工具门 = 半自动门'), 'C10', '移动端 AGENTS.md 未把 ② 移出人工门清单并写明运行前提');

  return violations;
}

describe('② 微信开发者工具门契约（#883 / 2026-09-12 半自动）', () => {
  const real = {
    script: readSource(SCRIPT_REL),
    probe: readSource(PROBE_REL),
    pkg: readSource(PKG_REL),
    adr: readSource(ADR_REL),
    agents: readSource(AGENTS_REL),
    template: readSource(PR_TEMPLATE_REL)
  };

  it('自检：注入违规必须被检出（防空跑假绿）', () => {
    const cases = [
      ['C1', { ...real, script: real.script.replace("$DistRelative = 'unpackage\\dist\\build\\mp-weixin'", "$DistRelative = 'x'") }],
      ['C1', { ...real, script: real.script.replace(/scripts\\mp-weixin-probe\.mjs/g, 'scripts\\other.mjs') }],
      ['C2', { ...real, script: real.script.replace("'close', '--project'", "'closeX', '--project'") }],
      ['C2', { ...real, script: real.script.replace("'auto', '--project'", "'autoX', '--project'") }],
      ['C3', { ...real, probe: real.probe.replace(/pageStackNonEmpty/g, 'stackOk') }],
      ['C3', { ...real, probe: real.probe + '\nconst el = await page.$("view");\n' }],
      ['C4', { ...real, script: real.script + '\nif ($LASTEXITCODE -ne 0) { exit 1 }\n' }],
      ['C4', { ...real, script: real.script.replace(/MP_WEIXIN_PROBE/g, 'PROBE_X') }],
      ['C4', { ...real, script: real.script.replace(/MP_WEIXIN_RESULT/g, 'RESULT_X') }],
      ['C5', { ...real, script: real.script.replace(/通配/g, '本机') }],
      ['C5', { ...real, script: real.script.replace(/监听面/g, 'x') }],
      ['C6', { ...real, script: real.script.replace(/gate-evidence:②/g, 'gate-evidence:x') }],
      ['C6', { ...real, script: real.script.replace(/- commit: \$sha/g, '- sha: $sha') }],
      ['C7', { ...real, script: real.script.replace(/\$script:closed/g, '$closedX') }],
      ['C8', { ...real, script: real.script.replace(/非等价/g, '等价性待定') }],
      ['C8', { ...real, script: real.script.replace(/manifest-appid/g, 'manifest-x') }],
      ['C9', { ...real, script: real.script.replace('$script:ArchiveMaxBytes = 150KB', '$script:ArchiveMaxBytes = 900KB') }],
      ['C9', { ...real, script: real.script.replace(/-NoArchive/g, '-ArchiveAll') }],
      ['C10', { ...real, pkg: real.pkg.replace('build:mp-weixin-check', 'build:mp-x') }],
      ['C10', { ...real, adr: real.adr.replace(/半自动/g, '全自动') }],
      ['C10', { ...real, template: real.template.replace(/gate-evidence:②/g, 'gate-evidence:x') }],
      ['C10', { ...real, agents: '人工门清单（无 ② 条目）' }],
      ['C11', { ...real, script: real.script.replace(/hx-busy\.ps1/g, 'other.ps1') }],
      ['C11', { ...real, script: real.script.replace(/Release-HxLock/g, 'ReleaseNothing') }],
      ['C11', { ...real, script: real.script.replace('Release-HxLock', 'Stop-Process -Name HBuilderX -Force') }]
    ];
    cases.forEach(([rule, sources]) => {
      const found = scanContract(sources);
      expect(found.some((v) => v.startsWith(rule))).toBe(true);
    });
  });

  it('真实文件：零违规', () => {
    expect(scanContract(real)).toEqual([]);
  });

  it('C2：`close` 严格早于 `auto`，且 auto 走 Invoke-Process（等它跑完）', () => {
    expect(real.script.indexOf("'close', '--project'")).toBeLessThan(real.script.indexOf("'auto', '--project'"));
    expect(real.script).toContain("Invoke-Process -FilePath $devTools -Arguments @('auto'");
  });

  it('C4：门结论只来自探针 JSON 输出，不来自任何退出码', () => {
    expect(real.script).toContain('MP_WEIXIN_PROBE');
    expect(real.script).not.toMatch(/\$LASTEXITCODE/);
    expect(real.probe).toContain('probeOk');
  });

  it('C6：仅门通过（exit 0）分支贴 sha 绑定评论', () => {
    const callAt = real.script.indexOf('Publish-GateComment -PrNumber');
    expect(callAt).toBeGreaterThan(-1);
    expect(real.script.indexOf('exit 1')).toBeLessThan(callAt);
    expect(real.script.indexOf('exit $exitCode')).toBeGreaterThan(callAt);
  });

  it('C9：入库上限写死在脚本里（10 张 / 150KB / 1.5MB / 720px）', () => {
    expect(real.script).toContain('$script:ArchiveMaxCount = 10');
    expect(real.script).toContain('$script:ArchiveMaxBytes = 150KB');
    expect(real.script).toContain('$script:ArchiveMaxTotal = 1.5MB');
    expect(real.script).toContain('$script:ArchiveMaxWidth = 720');
  });
});
