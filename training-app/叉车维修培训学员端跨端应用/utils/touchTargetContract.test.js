/**
 * 触摸目标契约（#2445）
 *
 * 背景（#2436 取证 → #2438 裁定 Q5）：全仓只有两处可点元素命中区小于 44×44px——
 * `pages/forum/forum.uvue` 的 `.create-pill`（120×56rpx，高 28px）、
 * `pages/jobs/job-list.uvue` 的 `.tab-avatar-btn`（72×88rpx，宽 36px）。
 * 裁定是「修这两处」，不是「全仓扫一遍」——所以本文件只钉这两处 + 规范落点，
 * 不做全仓静态扫描（那会把存量页一次性判红，属治理票的事）。
 *
 * 修法口径（docs/ui-spec.md §5.11）：视觉件可以更小，用外层 hit 盒撑到 88rpx；
 * 别把视觉件本身拉大来凑。
 */
const h = require('./contractHarness');

const MIN_RPX = 88;

/** 取 `.cls { ... }` 的声明体。选择器按字面 + `{` 锚定，避免 `.create-pill` 命中 `.create-pill-hit`。 */
function classBody(src, cls) {
  const m = src.match(new RegExp('\\.' + cls + '\\s*\\{([^}]*)\\}'));
  return m ? m[1] : '';
}

/** 从声明体里取某个属性的 rpx 数值；取不到返回 NaN（= 判红，别让「没写」被当成「够大」）。 */
function rpxOf(body, prop) {
  const m = body.match(new RegExp('(?:^|[;\\s])' + prop + '\\s*:\\s*(\\d+)rpx'));
  return m ? Number(m[1]) : NaN;
}

/**
 * 论坛发帖 pill 的违规清单。
 * 判据：命中区（`.create-pill-hit`）高度 ≥ 88rpx，且 `@click` 挂在命中区上而不是视觉胶囊上。
 */
function forumPillViolations(src) {
  const out = [];
  const hit = classBody(src, 'create-pill-hit');
  if (hit.length === 0) {
    out.push('缺 .create-pill-hit 命中盒（视觉胶囊 56rpx 不能直接当命中区）');
  } else if (!(rpxOf(hit, 'height') >= MIN_RPX)) {
    out.push(`.create-pill-hit 高度 ${rpxOf(hit, 'height')}rpx < ${MIN_RPX}rpx`);
  }
  if (!/<view class="create-pill-hit"[^>]*@click=/.test(src)) {
    out.push('@click 没挂在 .create-pill-hit 上');
  }
  if (/<view class="create-pill"[^>]*@click=/.test(src)) {
    out.push('@click 仍挂在 .create-pill 视觉胶囊上（命中区没生效）');
  }
  return out;
}

/** 职位列表头像按钮的违规清单：宽高都要 ≥ 88rpx。 */
function avatarBtnViolations(src) {
  const body = classBody(src, 'tab-avatar-btn');
  if (body.length === 0) {
    return ['缺 .tab-avatar-btn 样式块'];
  }
  const out = [];
  for (const prop of ['width', 'height']) {
    const v = rpxOf(body, prop);
    if (!(v >= MIN_RPX)) {
      out.push(`.tab-avatar-btn ${prop} ${v}rpx < ${MIN_RPX}rpx`);
    }
  }
  return out;
}

describe('触摸目标（#2445）', () => {
  it('A: 论坛发帖 pill 的命中区撑到 88rpx，@click 挂在命中区上', () => {
    expect(forumPillViolations(h.read('pages/forum/forum.uvue'))).toEqual([]);
  });

  it('B: 职位列表头像按钮宽高都 ≥ 88rpx', () => {
    expect(avatarBtnViolations(h.read('pages/jobs/job-list.uvue'))).toEqual([]);
  });

  it('C: 规范落在 docs/ui-spec.md §5.11（触摸目标 + 长文本截断）', () => {
    const spec = h.read('docs/ui-spec.md');
    expect(spec).toContain('### 5.11 触摸目标与长文本截断');
    expect(spec).toContain('88rpx × 88rpx');
    expect(spec).toContain('lines: 1');
  });

  it('D: 判别力 —— 修之前的旧值必须被判红', () => {
    const oldForum = [
      '<view class="create-pill" @click="onCreate">',
      '<style lang="scss">',
      '.create-pill { width: 120rpx; height: 56rpx; }',
      '</style>',
    ].join('\n');
    expect(forumPillViolations(oldForum).length).toBeGreaterThan(0);

    const oldAvatar = '<style lang="scss">\n.tab-avatar-btn { width: 72rpx; height: 88rpx; }\n</style>';
    expect(avatarBtnViolations(oldAvatar)).toEqual(['.tab-avatar-btn width 72rpx < 88rpx']);

    const goodAvatar = '<style lang="scss">\n.tab-avatar-btn { width: 88rpx; height: 88rpx; }\n</style>';
    expect(avatarBtnViolations(goodAvatar)).toEqual([]);
  });
});
