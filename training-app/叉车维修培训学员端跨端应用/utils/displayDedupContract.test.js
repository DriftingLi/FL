/**
 * 展示层泛型格式化「零第二实现」契约（2026-09-11 去重）
 *
 * 背景：formatDateStr 曾在全仓有 8 处实现（utils/format 规范 1 处 + forumDisplay 1 处
 * + 6 个页面内联逐字复制），formatDateTimeStr 3 处、formatCountCompact 2 处。
 * 处置：唯一实现收敛 utils/format；域展示模块与页面不再持有第二实现；
 * 模板消费一律走「本地语义名薄包装」（守护规则 S：模板禁直调 import 函数，先例 #677
 * activity-topic-card 的 displayTime）。
 *
 * 本锁防回潮：任一处重新长出 `function formatDateStr` 即红。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

const GENERIC_FORMATTERS = ['formatDateStr', 'formatDateTimeStr', 'formatCountCompact'];

describe('展示层泛型格式化：唯一实现点 utils/format，零第二实现', () => {
  it('规范源 utils/format.uts 提供三个泛型格式化函数', () => {
    const format = read('utils/format.uts');
    for (const fn of GENERIC_FORMATTERS) {
      expect(format).toContain(`export function ${fn}`);
    }
  });

  it('域展示模块不再持有泛型格式化第二实现', () => {
    for (const rel of ['utils/forumDisplay.uts', 'utils/forumDetailDisplay.uts']) {
      const src = read(rel);
      for (const fn of GENERIC_FORMATTERS) {
        expect(src).not.toContain(`export function ${fn}`);
        expect(src).not.toContain(`function ${fn}`);
      }
    }
  });

  it('消费页面/组件不持有本地同名实现（模板消费走语义名薄包装）', () => {
    const consumers = [
      'pages/profile/practice-records.uvue',
      'pages/profile/favorites.uvue',
      'pages/profile/mock-exam-records.uvue',
      'pages/profile/components/wrong-question-card.uvue',
      'pages/featured/featured-list.uvue',
      'pages/featured/featured-detail.uvue',
      'pages/forum/my-forum.uvue',
    ];
    for (const rel of consumers) {
      const src = read(rel);
      for (const fn of GENERIC_FORMATTERS) {
        expect(src).not.toContain(`function ${fn}`);
      }
    }
  });

  it('原内联复制点已改为消费 utils/format（import 存在）', () => {
    for (const rel of [
      'pages/profile/practice-records.uvue',
      'pages/profile/favorites.uvue',
      'pages/profile/mock-exam-records.uvue',
      'pages/profile/components/wrong-question-card.uvue',
      'pages/featured/featured-list.uvue',
      'pages/featured/featured-detail.uvue',
      'pages/forum/my-forum.uvue',
      'pages/forum/components/forum-topic-card.uvue',
      'pages/forum/components/forum-checkin-card.uvue',
      'pages/forum/components/forum-resource-panel.uvue',
      'pages/forum/forum-detail.uvue',
    ]) {
      expect(read(rel)).toMatch(
        new RegExp(`import \\{[^}]*\\b(${GENERIC_FORMATTERS.join('|')})\\b[^}]*\\} from '.*utils/format'`)
      );
    }
  });

  it('模板不直调 import 函数（守护规则 S 的消费侧对照）', () => {
    for (const rel of [
      'pages/profile/practice-records.uvue',
      'pages/profile/favorites.uvue',
      'pages/profile/mock-exam-records.uvue',
      'pages/featured/featured-list.uvue',
      'pages/featured/featured-detail.uvue',
      'pages/forum/my-forum.uvue',
    ]) {
      const src = read(rel);
      const tpl = src.slice(src.indexOf('<template>'), src.lastIndexOf('</template>'));
      expect(tpl).not.toMatch(/\{\{\s*format(Date|DateTime)Str\(/);
    }
  });
});
