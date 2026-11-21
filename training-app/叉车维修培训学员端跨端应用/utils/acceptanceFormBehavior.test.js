/**
 * 验收表单口径行为测试（#2743）
 *
 * 守的是用户能感知的两件事：
 * 1) 提交前校验真的会拦住缺结论 / 缺备注 / 缺凭证（票面要求 5）
 * 2) 整体结论真的按「最严重的一项」汇总（页面与 mock 共用这一处，不能各写一份）
 *
 * 用 loadUts 跑真实 `.uts` 源码，不复制实现。
 */
const h = require('./utsHarness');

const form = h.loadUts('utils/acceptanceForm.uts', {});

const proof = (url) => ({ url, name: '凭证', kind: 'image' });

const item = (id, result, remark, proofs) => ({
  id,
  name: '项' + id,
  requirement: '要求' + id,
  result,
  remark: remark || '',
  proofs: proofs || [],
});

describe('验收结论与状态文案', () => {
  it('三种结论与未确认都有中文（未知值兜底不抛错）', () => {
    expect(form.acceptanceResultText('pass')).toBe('通过');
    expect(form.acceptanceResultText('fail')).toBe('不通过');
    expect(form.acceptanceResultText('rectify')).toBe('待整改');
    expect(form.acceptanceResultText('')).toBe('未确认');
    expect(form.acceptanceResultText('weird')).toBe('未确认');
  });

  it('四种单据状态各自有文案', () => {
    expect(form.acceptanceStatusText('pending')).toBe('待验收');
    expect(form.acceptanceStatusText('submitted')).toBe('待教练验收');
    expect(form.acceptanceStatusText('accepted')).toBe('已通过');
    expect(form.acceptanceStatusText('rejected')).toBe('验收未通过');
    expect(form.acceptanceStatusText('x')).toBe('未知状态');
  });
});

describe('整体结论按最严重项汇总', () => {
  it('任一不通过 ⇒ fail（哪怕同时有待整改）', () => {
    const items = [item(1, 'pass'), item(2, 'rectify'), item(3, 'fail')];
    expect(form.acceptanceRollupResult(items)).toBe('fail');
  });

  it('没有不通过、但有待整改 ⇒ rectify', () => {
    expect(form.acceptanceRollupResult([item(1, 'pass'), item(2, 'rectify')])).toBe('rectify');
  });

  it('全部通过 ⇒ pass', () => {
    expect(form.acceptanceRollupResult([item(1, 'pass'), item(2, 'pass')])).toBe('pass');
  });
});

describe('统计与进度文案', () => {
  it('已确认数 / 异常数 / 凭证总数按项累加', () => {
    const items = [
      item(1, 'pass', '', [proof('a')]),
      item(2, 'rectify', '需复检', [proof('b'), proof('c')]),
      item(3, '', ''),
    ];
    expect(form.acceptanceDecidedCount(items)).toBe(2);
    expect(form.acceptanceFailedCount(items)).toBe(1);
    expect(form.acceptanceProofTotal(items)).toBe(3);
    expect(form.acceptanceProgressText(items)).toBe('已确认 2/3 项');
  });
});

describe('提交前校验（票面要求 5：必填项完整）', () => {
  it('验收项为空直接拒绝', () => {
    const v = form.acceptanceValidate([]);
    expect(v.ok).toBe(false);
    expect(v.message).toContain('验收项为空');
  });

  it('有项没选结论时，提示里点名第几项与项名，并回传该项 id', () => {
    const v = form.acceptanceValidate([item(1, 'pass'), item(2, '', '', [proof('a')])]);
    expect(v.ok).toBe(false);
    expect(v.message).toContain('第 2 项');
    expect(v.message).toContain('项2');
    expect(v.missingItemId).toBe(2);
  });

  it('「不通过」缺备注 ⇒ 拒绝', () => {
    const v = form.acceptanceValidate([item(1, 'fail', '', [proof('a')])]);
    expect(v.ok).toBe(false);
    expect(v.message).toContain('请填写备注');
  });

  it('「待整改」只填空格 ⇒ 仍算没填', () => {
    const v = form.acceptanceValidate([item(1, 'rectify', '   ', [proof('a')])]);
    expect(v.ok).toBe(false);
    expect(v.message).toContain('请填写备注');
  });

  it('备注超过 200 字 ⇒ 拒绝', () => {
    const long = '备'.repeat(201);
    const v = form.acceptanceValidate([item(1, 'fail', long, [proof('a')])]);
    expect(v.ok).toBe(false);
    expect(v.message).toContain('200');
  });

  it('备注恰好 200 字 ⇒ 通过（边界值不误杀）', () => {
    const exact = '备'.repeat(200);
    const v = form.acceptanceValidate([item(1, 'fail', exact, [proof('a')])]);
    expect(v.ok).toBe(true);
  });

  it('整单没有任何凭证 ⇒ 拒绝（票面要求 2：支持凭证，且提交要有依据）', () => {
    const v = form.acceptanceValidate([item(1, 'pass', '')]);
    expect(v.ok).toBe(false);
    expect(v.message).toContain('凭证');
  });

  it('逐项结论齐 + 异常项有备注 + 至少一份凭证 ⇒ 放行', () => {
    const v = form.acceptanceValidate([
      item(1, 'pass', '', [proof('a')]),
      item(2, 'rectify', '轮胎老化，建议更换'),
    ]);
    expect(v.ok).toBe(true);
    expect(v.message).toBe('');
  });
});
