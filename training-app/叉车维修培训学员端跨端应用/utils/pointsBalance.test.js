/**
 * 积分余额字段映射契约
 *
 * 后端 GET /api/points/balance 返回 { balance, total_earned, total_spent }（见
 * backend/internal/service/points_service.go PointsBalanceResult）。#709 前 PointsBalance
 * 沿用早期字段名 total_points / today_earned 并靠映射兜底，「我的」页可用积分卡片消费的
 * 就是这条回退——若映射丢掉 balance，页面会稳定显示 0，故在此钉住契约。
 *
 * #709 起字段与后端逐字对齐（total_points / today_earned / level 等幻影字段退役），
 * 本测试从「盯回退链」改为「盯直读 + 消费方读 balance」：
 *   1) api 层读 obj['balance']，不再有任何历史别名兜底；
 *   2) 页面消费 data.balance（读旧字名的写法一律视为回归）。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const apiSrc = fs.readFileSync(path.join(ROOT, 'api', 'points.uts'), 'utf8');
const profileSrc = fs.readFileSync(path.join(ROOT, 'pages', 'profile', 'profile.uvue'), 'utf8');
const typesSrc = fs.readFileSync(path.join(ROOT, 'types', 'index.uts'), 'utf8');

function buildFnBody() {
  const start = apiSrc.indexOf('function buildPointsBalance');
  if (start === -1) throw new Error('未找到 buildPointsBalance');
  return apiSrc.slice(start, apiSrc.indexOf('\n}', start));
}

describe('buildPointsBalance 字段映射', () => {
  const body = buildFnBody();

  it('直读后端三字段 balance / total_earned / total_spent', () => {
    expect(body).toContain("obj['balance']");
    expect(body).toContain("obj['total_earned']");
    expect(body).toContain("obj['total_spent']");
  });

  it('早期别名字段（total_points / today_earned / level）不再出现', () => {
    expect(body).not.toContain('total_points');
    expect(body).not.toContain('today_earned');
    expect(body).not.toContain("obj['level']");
  });
});

describe('消费方读后端真实字段名', () => {
  it('profile.uvue 可用积分取 data.balance（读旧名是 #709 回归）', () => {
    expect(profileSrc).toContain('data.balance');
    expect(profileSrc).not.toMatch(/\.total_points/);
  });

  it('PointsBalance 类型不含幻影字段', () => {
    const seg = typesSrc.slice(typesSrc.indexOf('export type PointsBalance = {'));
    const block = seg.slice(0, seg.indexOf('}') + 1);
    expect(block).not.toMatch(/total_points|today_earned|level_name|next_level_points/);
  });
});

it('仍然请求 /points/balance 端点（经 getMapped 出口，T03 收紧 #641 更新调用形态）', () => {
  expect(apiSrc).toContain("getMapped<PointsBalance>('/points/balance'");
});
