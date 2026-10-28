/**
 * 职位「需持证件」展示与筛选判定 · **行为级**测试（#1945 / spec #1935 草稿 19）
 *
 * 为什么是行为而不是源码文本：本票最容易写错的三件事 ——
 *   ① 「多证折叠」的计数（显示几个名字、折叠里写的是**真实总数**还是可见数）；
 *   ② 「无证件要求」必须是**不渲染**，不是渲染一条空标签；
 *   ③ 「全部证件」与非法 id 必须折成「不过滤」（传错一个 0 就会被后端判 400，
 *      而票面的「非法证件 id 400」正是后端行为）。
 * —— 三者都是只有跑起来才看得见的语义。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts` 把 `.uts` 去类型后当 JS 执行
 * （先例 `utils/recruitDisplayBehavior.test.js` 跑同一形态的纯判定模块）。
 */
const fs = require('fs');
const os = require('os');
const path = require('path');

// readText 是读取层归一（CRLF → LF，ADR-0019）：裸 readFileSync 在 Windows 检出下会让
// 多行锚点匹配 0 次、变异没生效而断言照跑 ⇒ 守护静默失效（仓内守卫 scripts/check-contract-read.mjs 判红）
const { loadUts, readText } = require('./utsHarness');

const DISPLAY_UTS = path.join(__dirname, 'jobCredentialDisplay.uts');

/** 每次取一个全新模块实例（模块级常量幂等，互不串） */
const display = () => loadUts(DISPLAY_UTS, {});

/** 造一条职位所需证件（后端 required_credentials 的最小集合：id + 名称） */
const cred = (credential_id, name) => ({ credential_id, name });

describe('jobCredentialNotice：职位卡「需持 X 证」一行文案', () => {
  const { jobCredentialNotice } = display();

  it('无证件要求 ⇒ 空串（页面据此不渲染该行，而不是渲染空标签）', () => {
    expect(jobCredentialNotice([])).toBe('');
  });

  it('单证 ⇒ 需持 + 字典名称', () => {
    expect(jobCredentialNotice([cred(3, '叉车司机N1证')])).toBe('需持 叉车司机N1证');
  });

  it('两证 ⇒ 顿号连接（默认上限就是 2 个具名）', () => {
    expect(jobCredentialNotice([cred(3, '叉车司机N1证'), cred(7, '电工证')]))
      .toBe('需持 叉车司机N1证、电工证');
  });

  it('超过上限 ⇒ 前 2 个具名 + 等 N 证（N 是真实总数，不是可见数）', () => {
    const three = [cred(1, '甲证'), cred(2, '乙证'), cred(3, '丙证')];
    expect(jobCredentialNotice(three)).toBe('需持 甲证、乙证 等 3 证');
  });

  it('上限可由调用方收窄（上限 1 时也报真实总数）', () => {
    const three = [cred(1, '甲证'), cred(2, '乙证'), cred(3, '丙证')];
    expect(jobCredentialNotice(three, 1)).toBe('需持 甲证 等 3 证');
  });

  it('上限 >= 总数 ⇒ 全部具名，不带「等 N 证」', () => {
    const three = [cred(1, '甲证'), cred(2, '乙证'), cred(3, '丙证')];
    expect(jobCredentialNotice(three, 5)).toBe('需持 甲证、乙证、丙证');
  });

  it('名称为空的条目被跳过；全为空 ⇒ 空串（不拼出「需持 」这种半截文案）', () => {
    expect(jobCredentialNotice([cred(1, ''), cred(2, '  '), cred(3, '甲证')])).toBe('需持 甲证');
    expect(jobCredentialNotice([cred(1, ''), cred(2, '   ')])).toBe('');
  });

  it('名称两端空白被裁掉（字典脏数据不把空格带进卡面）', () => {
    expect(jobCredentialNotice([cred(1, '  甲证  ')])).toBe('需持 甲证');
  });

  it('折叠计数按**去掉空名之后**的真实数算', () => {
    const withBlank = [cred(1, ''), cred(2, '甲证'), cred(3, '乙证'), cred(4, '丙证')];
    expect(jobCredentialNotice(withBlank)).toBe('需持 甲证、乙证 等 3 证');
  });
});

describe('buildCredentialFilterOptions：筛选栏「证件」组选项', () => {
  const { buildCredentialFilterOptions, JOB_CREDENTIAL_ALL_ID } = display();

  it('首项恒为「全部证件」且 id 为 0（后端证件 id 恒正，不会撞车）', () => {
    const options = buildCredentialFilterOptions([cred(3, '叉车司机N1证')]);
    expect(options[0]).toEqual({ credential_id: 0, name: '全部证件' });
    expect(JOB_CREDENTIAL_ALL_ID).toBe(0);
  });

  it('保持后端字典顺序，不去重排、不重命名', () => {
    const options = buildCredentialFilterOptions([cred(9, '乙证'), cred(3, '甲证')]);
    expect(options.map(o => o.name)).toEqual(['全部证件', '乙证', '甲证']);
  });

  it('按 id 去重（字典接口分组返回时同一证件可能重复出现）', () => {
    const options = buildCredentialFilterOptions([cred(3, '甲证'), cred(3, '甲证'), cred(9, '乙证')]);
    expect(options.map(o => o.credential_id)).toEqual([0, 3, 9]);
  });

  it('非法 id（<= 0）被剔除，不制造出第二个「全部证件」', () => {
    const options = buildCredentialFilterOptions([cred(0, '零号'), cred(-5, '负数'), cred(3, '甲证')]);
    expect(options.map(o => o.credential_id)).toEqual([0, 3]);
  });

  it('空字典 ⇒ 只剩「全部证件」（筛选组不空、可清除）', () => {
    expect(buildCredentialFilterOptions([])).toEqual([{ credential_id: 0, name: '全部证件' }]);
  });
});

describe('成对取证（必红）：上面那些判据在真实坏实现上确实会红', () => {
  /** 读真源 → 注入变异 → 落临时目录（锚点必须命中：变异没进去就等于没测） */
  const mutatedCopy = (replacements) => {
    let src = readText(DISPLAY_UTS);
    for (const [from, to] of replacements) {
      expect([from, src.includes(from)]).toEqual([from, true]);
      src = src.split(from).join(to);
    }
    const dir = fs.mkdtempSync(path.join(os.tmpdir(), 'job-credential-'));
    const file = path.join(dir, 'jobCredentialDisplay.uts');
    fs.writeFileSync(file, src);
    return loadUts(file, {});
  };

  const three = [cred(1, '甲证'), cred(2, '乙证'), cred(3, '丙证')];

  it('坏实现①：折叠里写可见数（不是真实总数）⇒ 卡面谎报证件数，判据必红', () => {
    const broken = mutatedCopy([[
      "out = out + ' 等 ' + names.length.toString() + ' 证'",
      "out = out + ' 等 ' + visibleCount.toString() + ' 证'",
    ]]);
    expect(broken.jobCredentialNotice(three)).toBe('需持 甲证、乙证 等 2 证');
    expect(broken.jobCredentialNotice(three)).not.toBe(display().jobCredentialNotice(three));
  });

  it('坏实现②：无证件要求时返回半截文案（而非空串）⇒ 卡面渲染空标签，判据必红', () => {
    const broken = mutatedCopy([[
      "if (names.length == 0) return ''",
      "if (names.length == 0) return '需持 '",
    ]]);
    expect(broken.jobCredentialNotice([])).toBe('需持 ');
    expect(broken.jobCredentialNotice([])).not.toBe(display().jobCredentialNotice([]));
  });

  it('坏实现③：「全部证件」原样透传 ⇒ 后端吃 0 判 400，判据必红', () => {
    const broken = mutatedCopy([[
      'return selectedId > JOB_CREDENTIAL_ALL_ID ? selectedId : null',
      'return selectedId',
    ]]);
    expect(broken.credentialFilterParam(0)).toBe(0);
    expect(broken.credentialFilterParam(0)).not.toBe(display().credentialFilterParam(0));
  });
});

describe('credentialFilterParam：选中项 → 列表查询参数', () => {
  const { credentialFilterParam } = display();

  it('选中具体证件 ⇒ 原值透传（后端 credential_id 参数）', () => {
    expect(credentialFilterParam(7)).toBe(7);
  });

  it('「全部证件」⇒ null（不传参数 = 不限证件，含无证件要求的职位）', () => {
    expect(credentialFilterParam(0)).toBeNull();
  });

  it('非法值 ⇒ null（宁可不过滤，也不把 0/负数发给后端换一个 400）', () => {
    expect(credentialFilterParam(-3)).toBeNull();
  });
});
