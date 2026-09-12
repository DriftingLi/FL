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
 *   C11 HBuilderX 忙检测：必须 dot-source scripts/lib/hx-busy.ps1、等它给结论、**绝不 kill 主程序**
 *   C12 **② 的时序三步写死**（2026-09-12 复测）：`close → open --project <dist> → auto`；
 *       `open` 必须是**显式步骤 + 写进日志**，`-SkipBuild` 下同样能定位构建产物，且不得被 build 分支条件包住
 *   C13 **导航不可用时诚实降级**（2026-09-12 复测）：探针把「逐页导航 + 每页截图」记 SKIP
 *       （reason=navigation-api-unsupported）、SKIP **不得写成 PASS**、必须写进结果行与门评论；
 *       门只对可验证子集作结论（failures 只由该子集产生）
 *   C14 贴评论调用**不得断链**：语句续行符缺失会让 `-ArchivedRel …` 变成另一条命令（曾真实存在）
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

  // C12 ② 的时序三步：close → open --project <dist> → auto（缺 open 会撞 pageStack 恒空）
  const closeIdx = s.indexOf("'close', '--project'");
  const openIdx = s.indexOf("'open', '--project'");
  const autoIdx = s.indexOf("'auto', '--project'");
  must(openIdx !== -1, 'C12', '缺少 `cli.bat open --project <dist>`（close 之后 auto 之前必须重开项目窗口，否则 pageStack 恒空）');
  if (closeIdx !== -1 && openIdx !== -1 && autoIdx !== -1) {
    must(closeIdx < openIdx, 'C12', '`open` 未出现在 `close` 之后（close 会把项目窗口一起关掉，必须先关再开）');
    must(openIdx < autoIdx, 'C12', '`open` 未出现在 `auto` 之前（auto 不会替你重开项目窗口）');
  }
  must(/Invoke-Process -FilePath \$devTools -Arguments @\('open', '--project', \$dist\)/.test(s), 'C12', 'open 未作为显式步骤经 Invoke-Process 调用 $dist（须用构建产物目录，-SkipBuild 下也要能定位）');
  must(s.includes('devtools-open'), 'C12', 'open 步骤没有独立的 Tag/日志（`devtools-open`）');
  must(/Write-Log "`n>>> devtools-open/.test(s), 'C12', 'open 步骤的输出没有写进日志（须与 close/auto 一样可追溯）');
  must(s.includes('$open.TimedOut'), 'C12', 'open 缺超时兜底（窗口没重开就没法继续，超时判 env 不可用）');
  if (openIdx !== -1) {
    const lastDistGuard = s.lastIndexOf('no-dist');
    must(lastDistGuard !== -1 && openIdx > lastDistGuard, 'C12', 'open 步骤落在构建分支条件内（-SkipBuild 路径到不了它）');
  }

  // C13 导航不可用时诚实降级：SKIP，不得写成 PASS；且必须写进结果行与门评论
  must(probe.includes('navigation-api-unsupported'), 'C13', '探针未记录降级原因 `navigation-api-unsupported`');
  must(/NAV_SKIP_ASSERTIONS/.test(probe), 'C13', '探针未声明「降级涉及哪几条断言」（allRoutesVisited / screenshotsProduced）');
  must(/'skip'/.test(probe), 'C13', '探针未把降级断言记为 SKIP（字符串 skip），有假绿风险');
  must(/if \(v === false\)/.test(probe), 'C13', '探针的 failures 不是「只由明确 false（可验证子集不过）」产生（SKIP 会被算成失败或 PASS 混入）');
  must(/isNavigationUnsupported/.test(probe), 'C13', '探针未区分「导航 API 不支持」与真失败（TIMEOUT 之类不得被降级吞掉）');
  must(probe.includes('currentPageScreenshot') || probe.includes('current.png'), 'C13', '探针降级后没有取「当前页」截图（可验证子集要求 ≥1 张）');
  must(s.includes('navigation=$navField'), 'C13', '结果行缺 `navigation=` 字段（降级必须机检可见）');
  must(s.includes('navigation=SKIP reason='), 'C13', '日志缺 `navigation=SKIP reason=…` 行（降级必须写进日志）');
  must(!/navigation=PASS/.test(s), 'C13', '把导航降级写成了 PASS（降级不等于通过）');
  must(s.includes('降级（SKIP，非 PASS）'), 'C13', 'PR 门评论未显式写明降级（SKIP，非 PASS）');
  must(/navigation-api-unsupported/.test(s), 'C13', '脚本侧未写明降级原因（reason=navigation-api-unsupported）');
  must(/未证实/.test(s), 'C13', '脚本头未写明「未证实」（版本组合问题留给后续排查，不得宣称已定位根因）');
  must(/首跑/.test(s), 'C13', '脚本头未写明首跑校准要求');

  // C14 贴评论调用不得断链（缺续行反引号会把参数变成另一条命令）
  const lines = s.split(/\r?\n/);
  lines.forEach((line, idx) => {
    if (!/^\s+-[A-Za-z][\w]*\b/.test(line)) return;
    if (idx === 0 || /`\s*$/.test(lines[idx - 1])) return;
    violations.push('C14 第 ' + (idx + 1) + ' 行以 `' + line.trim().split(/\s+/)[0] + '` 起行，但上一行没有续行反引号（语句断链，参数不会生效）');
  });
  if (callAt !== -1) {
    const callBlock = s.slice(callAt, s.indexOf('exit $exitCode'));
    must(callBlock.includes('-ArchivedRel') && callBlock.includes('-ArchiveNotes'), 'C14', 'Publish-GateComment 调用未传 -ArchivedRel / -ArchiveNotes（入库截图清单进不了评论）');
  }

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
      ['C11', { ...real, script: real.script.replace('Release-HxLock', 'Stop-Process -Name HBuilderX -Force') }],
      // C12：删掉/挪走「重开项目窗口」这一步（pageStack 恒空的老问题）
      ['C12', { ...real, script: real.script.replace("'open', '--project'", "'openX', '--project'") }],
      ['C12', { ...real, script: real.script.replace("Invoke-Process -FilePath $devTools -Arguments @('open', '--project', $dist)", "Invoke-Process -FilePath $devTools -Arguments @('auto', '--project', $dist)") }],
      ['C12', { ...real, script: real.script.replace(/devtools-open/g, 'devtools-x') }],
      ['C12', { ...real, script: real.script.replace(/\$open\.TimedOut/g, '$openX.TimedOut') }],
      // C13：把降级写成 PASS / 让 SKIP 静默变成通过
      ['C13', { ...real, script: real.script.replace('navigation=SKIP reason=', 'navigation=PASS reason=') }],
      ['C13', { ...real, script: real.script.replace('navigation=$navField', 'navigation=ok') }],
      ['C13', { ...real, script: real.script.replace('降级（SKIP，非 PASS）', '结果说明') }],
      ['C13', { ...real, probe: real.probe.replace(/navigation-api-unsupported/g, 'x') }],
      ['C13', { ...real, probe: real.probe.replace(/'skip'/g, 'true') }],
      ['C13', { ...real, probe: real.probe.replace('if (v === false)', 'if (v === true)') }],
      // C14：续行反引号被删 ⇒ `-ArchivedRel …` 变成另一条命令（曾真实发生过）
      ['C14', { ...real, script: real.script.replace(/-ReproCommand 'npm run build:mp-weixin-check' `/g, "-ReproCommand 'npm run build:mp-weixin-check'") }]
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

  it('C12：close → open --project <dist> → auto 三步顺序写死，且 open 是显式步骤并写进日志', () => {
    const s = real.script;
    const closeIdx = s.indexOf("'close', '--project'");
    const openIdx = s.indexOf("'open', '--project'");
    const autoIdx = s.indexOf("'auto', '--project'");
    expect(closeIdx).toBeGreaterThan(-1);
    expect(openIdx).toBeGreaterThan(closeIdx);
    expect(autoIdx).toBeGreaterThan(openIdx);
    // 显式步骤 + 走 $dist（-SkipBuild 下同样定位构建产物）
    expect(s).toContain("Invoke-Process -FilePath $devTools -Arguments @('open', '--project', $dist)");
    expect(s).toContain('devtools-open');
    expect(s).toMatch(/Write-Log "`n>>> devtools-open/);
    expect(s).toContain('$open.TimedOut');
    // 不得被 build 分支包住：open 必须晚于 -SkipBuild 分支的 no-dist 兜底
    expect(openIdx).toBeGreaterThan(s.lastIndexOf('no-dist'));
  });

  it('C13：导航不可用时降级为 SKIP（不是 PASS），且写进结果行 / 日志 / 门评论', () => {
    expect(real.probe).toContain('navigation-api-unsupported');
    expect(real.probe).toContain("NAV_SKIP_ASSERTIONS");
    expect(real.probe).toContain("'skip'");
    expect(real.probe).toContain('if (v === false)');
    expect(real.script).toContain('navigation=$navField');
    expect(real.script).toContain('navigation=SKIP reason=');
    expect(real.script).toContain('navigation-api-unsupported');
    expect(real.script).toContain('降级（SKIP，非 PASS）');
    expect(real.script).not.toMatch(/navigation=PASS/);
    // 可验证子集写在脚本头里（门只对它作结论）
    expect(real.script).toContain('可验证子集');
    expect(real.script).toContain('currentPageScreenshot');
    expect(real.probe).toContain('currentPageScreenshot');
  });

  it('C13：SKIP 不产生 failures，failures 只由可验证子集（明确 false）产生', () => {
    // 静态守护：探针只把 `v === false` 收进 failures，且降级断言的值是字符串 'skip'
    const probe = real.probe;
    const finishBody = probe.slice(probe.indexOf('function finish('), probe.indexOf('const loaded = loadAutomator()'));
    expect(finishBody).toMatch(/if \(v === false\) result\.failures\.push/);
    expect(finishBody).toMatch(/:\s*'skip'/); // 降级时断言的值是字符串 'skip'（既不是 true 也不是 false）
    expect(finishBody).not.toMatch(/v\)\s*\{\s*result\.failures/); // 旧的 `if (!v)` 形态（会把 SKIP 算成失败）
  });

  it('C14：贴评论调用是一条语句（续行反引号不缺，-ArchivedRel/-ArchiveNotes 在语句内）', () => {
    const lines = real.script.split(/\r?\n/);
    lines.forEach((line, idx) => {
      if (!/^\s+-[A-Za-z][\w]*\b/.test(line)) return;
      if (idx === 0) return;
      expect(lines[idx - 1]).toMatch(/`\s*$/); // 上一行必须以续行反引号结尾
    });
    const callAt = real.script.indexOf('Publish-GateComment -PrNumber');
    const callBlock = real.script.slice(callAt, real.script.indexOf('exit $exitCode'));
    expect(callBlock).toContain('-ArchivedRel');
    expect(callBlock).toContain('-ArchiveNotes');
  });
});
