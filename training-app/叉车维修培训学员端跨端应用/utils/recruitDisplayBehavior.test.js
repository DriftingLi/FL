/**
 * 招聘者工作区展示判定 · **行为级**测试（ADR-0021 ④ 线 4 / ADR-0022 ④ 步骤 P2）
 *
 * 为什么是行为而不是源码文本：本票最容易写错的三件事 ——
 *   ① RFC3339 → epoch 的纯算术解析（含 `+08:00` 偏移）；
 *   ② 「剩余天数」的向上取整边界；
 *   ③ 分段徽标**只计未过期的 pending**（服务端不按 `expires_at` 判状态，过期记录仍发 `pending`）
 * —— 都是**只有跑起来才看得见**的语义。源码契约测试只能证明「某个字面量出现过」，
 * 证明不了「过期的那条没被算进徽标」。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts` 把 `.uts` 去类型后当 JS 执行（先例
 * `utils/concurrent401RefreshBehavior.test.js` 跑 `stores/auth.uts` / `api/request.uts`）。
 * `utils/recruitDisplay.uts` 故意做成**零运行期依赖**（只有 `import type`），因此注入空绑定即可。
 *
 * 口径**与检出平台无关**：期望值由 `Date.UTC` 现算（独立于被测实现的正交真源），
 * 不写死时间戳、不读工作树真源。
 */
const path = require('path');

const { loadUts } = require('./utsHarness');

const DISPLAY_UTS = path.join(__dirname, 'recruitDisplay.uts');

/** 每次取一个**全新模块实例**（模块级常量幂等，互不串） */
const display = () => loadUts(DISPLAY_UTS, {});

/** 正交真源：平台自己的 UTC 解释 */
const utc = (y, mo, d, h = 0, mi = 0, s = 0) => Date.UTC(y, mo - 1, d, h, mi, s);

const DAY = 86400000;

describe('parseIsoMs：RFC3339 → epoch 毫秒（纯算术，不依赖平台日期解析）', () => {
  const { parseIsoMs, daysFromCivil } = display();

  it('daysFromCivil 与公历基准一致（1970-01-01 = 0，已知锚点两个）', () => {
    expect(daysFromCivil(1970, 1, 1)).toBe(0);
    expect(daysFromCivil(2000, 3, 1)).toBe(11017);
    expect(daysFromCivil(1969, 12, 31)).toBe(-1);
  });

  it('`Z` 结尾按 UTC 解析', () => {
    expect(parseIsoMs('2026-10-01T12:00:00Z')).toBe(utc(2026, 10, 1, 12));
    expect(parseIsoMs('1970-01-01T00:00:00Z')).toBe(0);
  });

  it('`+HH:MM` 偏移减回去（本地 = UTC + 偏移）', () => {
    expect(parseIsoMs('2026-10-01T20:00:00+08:00')).toBe(utc(2026, 10, 1, 12));
    expect(parseIsoMs('2026-10-01T02:30:00+14:30')).toBe(utc(2026, 9, 30, 12));
  });

  it('`-HH:MM` 偏移加回来', () => {
    expect(parseIsoMs('2026-10-01T07:30:00-05:30')).toBe(utc(2026, 10, 1, 13));
  });

  it('无后缀按 UTC 解析；日期与时间之间允许空格', () => {
    expect(parseIsoMs('2026-10-01T12:00:00')).toBe(utc(2026, 10, 1, 12));
    expect(parseIsoMs('2026-10-01 12:00:00')).toBe(utc(2026, 10, 1, 12));
  });

  it('小数秒**被忽略**（整秒精度；非 RFC3339 差异，是显式声明过的行为）', () => {
    expect(parseIsoMs('2026-10-01T12:00:00.999Z')).toBe(utc(2026, 10, 1, 12));
  });

  it('跨月/跨年正确（12-31 23:59:59 → 次年 01-01 00:00:00 差 1 秒）', () => {
    const a = parseIsoMs('2026-12-31T23:59:59Z');
    const b = parseIsoMs('2027-01-01T00:00:00Z');
    expect(b - a).toBe(1000);
  });

  it('无法解析一律返回 -1（**不是** 0，否则会被误判成「已过期」）', () => {
    const bad = [
      '',                       // 空
      '2026-10-01',             // 只有日期（长度不足）
      '2026/10/01T12:00:00Z',   // 分隔符不是 '-'
      '2026-10-01X12:00:00Z',   // 日期与时间之间不是 T/空格
      '2026-10-01T12-00:00Z',   // 时间分隔符不是 ':'
      '2026-13-01T12:00:00Z',   // 月份越界
      '2026-10-32T12:00:00Z',   // 日越界
      '2026-10-00T12:00:00Z',   // 日下界
    ];
    for (const s of bad) {
      expect([s, parseIsoMs(s)]).toEqual([s, -1]);
    }
  });
});

describe('contactExpired：过期是纯事实，与 status 无关', () => {
  const { contactExpired, parseIsoMs } = display();
  const at = '2026-10-01T12:00:00Z';
  const ms = parseIsoMs(at);

  it('到期时刻**本身**算已过期（<=，不是 <）', () => {
    expect(contactExpired(at, ms)).toBe(true);
    expect(contactExpired(at, ms - 1)).toBe(false);
  });

  it('解析不了（脏数据）返回 false：宁可留在「未过期」被看见，也不凭空标成已过期', () => {
    expect(contactExpired('', ms)).toBe(false);
    expect(contactExpired('not-a-date', ms)).toBe(false);
  });
});

describe('remainingDays：向上取整的边界', () => {
  const { remainingDays, parseIsoMs } = display();
  const at = '2026-10-01T12:00:00Z';
  const ms = parseIsoMs(at);

  it('整 3 天 / 差 1 毫秒到整 3 天 → 都是 3', () => {
    expect(remainingDays(at, ms - 3 * DAY)).toBe(3);
    expect(remainingDays(at, ms - 3 * DAY + 1)).toBe(3);
  });

  it('差 1 毫秒 → 1（不折成 0）', () => {
    expect(remainingDays(at, ms - 1)).toBe(1);
  });

  it('到期时刻与已过期 → 0', () => {
    expect(remainingDays(at, ms)).toBe(0);
    expect(remainingDays(at, ms + DAY)).toBe(0);
  });

  it('解析不了 → 0', () => {
    expect(remainingDays('', ms)).toBe(0);
  });
});

describe('徽标与紧迫行：只计**未过期**的 pending（票面判据的行为面）', () => {
  const at = (daysFromNow) => new Date(utc(2026, 10, 1, 12) + daysFromNow * DAY).toISOString().replace('.000Z', 'Z');
  const NOW = utc(2026, 10, 1, 12);

  const items = [
    { id: 1, status: 'pending', expires_at: at(3) },    // 未过期 pending → 计入
    { id: 2, status: 'pending', expires_at: at(11) },   // 未过期 pending → 计入
    { id: 3, status: 'pending', expires_at: at(-1) },   // **过期** pending → 不计入
    { id: 4, status: 'pending', expires_at: at(0) },    // 恰好到期 → 不计入
    { id: 5, status: 'approved', expires_at: at(9) },   // 非 pending → 不计入
    { id: 6, status: 'rejected', expires_at: at(9) },   // 非 pending → 不计入
    { id: 7, status: 'expired', expires_at: at(-9) },   // 服务端已标过期 → 不计入
    { id: 8, status: 'pending', expires_at: 'bad' },    // 脏数据 → 留在未过期（fail-open 但可见）
  ];

  it('计数：只数未过期的 pending（本例 3 条：id 1/2/8）', () => {
    const { unexpiredPendingCount } = display();
    expect(unexpiredPendingCount(items, NOW)).toBe(3);
  });

  it('计数忽略顺序与长度（空表 → 0）', () => {
    const { unexpiredPendingCount } = display();
    expect(unexpiredPendingCount([], NOW)).toBe(0);
    expect(unexpiredPendingCount([items[3]], NOW)).toBe(0);
  });

  it('紧迫行取的是**最近**到期的那条未过期 pending', () => {
    const { soonestPendingIndex } = display();
    expect(soonestPendingIndex(items, NOW)).toBe(0); // id 1：3 天后
    const shuffled = [items[1], items[0], items[4]];
    expect(shuffled[soonestPendingIndex(shuffled, NOW)].id).toBe(1);
  });

  it('解析不出时间的 pending **不参与**排序（否则它的 -1 会恒为最小、把紧迫行整条压掉）', () => {
    const { soonestPendingIndex } = display();
    const dirtyOnly = [items[7]]; // id 8：expires_at = 'bad'
    expect(soonestPendingIndex(dirtyOnly, NOW)).toBe(-1);
    // 有正常行时，脏行不得抢占（下标的对象必须是 id 1）
    const mixed = [items[7], items[0]];
    expect(mixed[soonestPendingIndex(mixed, NOW)].id).toBe(1);
  });

  it('没有未过期 pending 时返回 -1 哨兵', () => {
    const { soonestPendingIndex } = display();
    expect(soonestPendingIndex([], NOW)).toBe(-1);
    expect(soonestPendingIndex(items.slice(2), NOW)).toBe(-1);
  });

  it('文案：数量为 0 或天数不足时不产出空话', () => {
    const { contactUrgencyText } = display();
    expect(contactUrgencyText(1, 3)).toBe('交换申请有 1 条等待回应，最近 3 天后过期');
    expect(contactUrgencyText(2, 1)).toBe('交换申请有 2 条等待回应，最近 1 天后过期');
    expect(contactUrgencyText(0, 3)).toBe('');
    expect(contactUrgencyText(1, 0)).toBe('');
  });

  it('剩余天数文案', () => {
    const { contactRemainingText } = display();
    expect(contactRemainingText(5)).toBe('剩 5 天');
    expect(contactRemainingText(1)).toBe('剩 1 天');
    expect(contactRemainingText(0)).toBe('');
  });
});

describe('状态词表投影：过期态覆盖 pending，未知取值诚实兜底', () => {
  it('交换申请：pending 未过期 = 待同意；pending 已过期 = **已过期**（服务端不会自己改）', () => {
    const { describeRecruitContactStatus } = display();
    expect(describeRecruitContactStatus('pending', false).label).toBe('待同意');
    expect(describeRecruitContactStatus('pending', true).label).toBe('已过期');
    expect(describeRecruitContactStatus('pending', true).status).toBe('expired');
  });

  it('交换申请：其余四态与词表一致；认不出的取值折到「状态未知」而不是假装成 pending', () => {
    const { describeRecruitContactStatus } = display();
    expect(describeRecruitContactStatus('approved', false).label).toBe('已同意');
    expect(describeRecruitContactStatus('rejected', false).label).toBe('已拒绝');
    expect(describeRecruitContactStatus('revoked', false).label).toBe('已撤回');
    expect(describeRecruitContactStatus('expired', false).label).toBe('已过期');
    const unknown = describeRecruitContactStatus('weird_new_state', false);
    expect(unknown.label).toBe('状态未知');
    expect(unknown.status).toBe('unknown');
  });

  it('投递：三态 + 未知兜底', () => {
    const { describeRecruitApplicationStatus } = display();
    expect(describeRecruitApplicationStatus('applied').label).toBe('投递中');
    expect(describeRecruitApplicationStatus('rejected').label).toBe('不合适');
    expect(describeRecruitApplicationStatus('withdrawn').label).toBe('已撤回');
    expect(describeRecruitApplicationStatus('weird_new_state').status).toBe('unknown');
  });

  it('职位：强制下架**优先于** status（open+forced 不得显示成「招聘中」）', () => {
    const { describeRecruitJobStatus } = display();
    expect(describeRecruitJobStatus('open', true).label).toBe('平台强制下架');
    expect(describeRecruitJobStatus('open', false).label).toBe('招聘中');
    expect(describeRecruitJobStatus('closed', false).label).toBe('已下架');
    expect(describeRecruitJobStatus('weird_new_state', false).status).toBe('unknown');
  });

  it('词表 class 后缀只用 `[a-z_]` 会踩 uvue 吗：派生键不含下划线（`.tag-forced`）', () => {
    const { describeRecruitJobStatus } = display();
    // 强制下架这一行的 status 同时是 class 后缀（`.tag-<status>`）——
    // uvue 的 class 选择器用下划线虽合法，但消费面刻意避开，此断言把这条约定钉住
    expect(describeRecruitJobStatus('open', true).status).toBe('forced');
  });
});

describe('模块契约：零运行期依赖（loadUts 注入空绑定即可跑）', () => {
  it('同一个文件能被反复载入且互不污染（模块级常量不是可变状态）', () => {
    const a = display();
    const b = display();
    expect(a.RECRUIT_CONTACT_STATUS_DESCRIPTORS.length).toBe(b.RECRUIT_CONTACT_STATUS_DESCRIPTORS.length);
    expect(a.RECRUIT_CONTACT_STATUS_DESCRIPTORS).not.toBe(b.RECRUIT_CONTACT_STATUS_DESCRIPTORS);
  });
});
