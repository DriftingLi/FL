/**
 * 积分余额字段映射契约
 *
 * 后端 GET /api/points/balance 返回 { balance, total_earned }（见
 * backend/internal/service/points_service.go），而 PointsBalance 沿用
 * total_points / today_earned 字段名。「我的」页面的可用积分卡片直接消费该字段，
 * 若映射丢掉 balance 回退，页面会稳定显示 0，故在此钉住契约。
 */
const fs = require('fs');
const path = require('path');

const apiSrc = fs.readFileSync(path.join(__dirname, '..', 'api', 'points.uts'), 'utf8');

function buildFnBody() {
  const start = apiSrc.indexOf('function buildPointsBalance');
  if (start === -1) throw new Error('未找到 buildPointsBalance');
  return apiSrc.slice(start, apiSrc.indexOf('\n}', start));
}

describe('buildPointsBalance 字段映射', () => {
  const body = buildFnBody();

  it('读取后端 balance / total_earned 作为回退字段', () => {
    expect(body).toContain("obj['balance']");
    expect(body).toContain("obj['total_earned']");
  });

  it('total_points 优先，缺失时才回退 balance', () => {
    expect(body).toContain("total_points: totalRaw != null ? toNumber(totalRaw) : toNumber(balanceRaw)");
    expect(body).toContain("today_earned: todayRaw != null ? toNumber(todayRaw) : toNumber(earnedRaw)");
    expect(body).toContain("const totalRaw = obj['total_points']");
    expect(body).toContain("const balanceRaw = obj['balance']");
  });

  it('仍然请求 /points/balance 端点', () => {
    expect(apiSrc).toContain("get('/points/balance')");
  });
});
