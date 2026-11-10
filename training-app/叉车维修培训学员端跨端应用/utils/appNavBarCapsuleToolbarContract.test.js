/**
 * 页头胶囊让位 + 标题栏下工具栏契约（2026-11-10）
 *
 * 现象：微信小程序右上角胶囊与业务按钮争宽。右槽可用宽度 ≈ capsuleLeft(rpx) − 内边距，
 * 典型约 194rpx —— 够一个图标或一段 MM:SS，两个以上动作只能向左溢出压标题
 * （先例：forum #1846 把发帖 pill 手工下沉到搜索栏）。
 *
 * 本文件守的是**几何与开关**，不是像素好看：
 * A. 胶囊位置只有一真源 —— utils/system.uts 的 useSafeArea；页头件只消费
 *    capsuleLeft / rightSlotWidth，不得自己量、不得写死右向预留
 *    （`padding-right: <数字>rpx` 正是 #1848 死件 platform-nav-bar 的历史形态；
 *    槽容器的简写 `padding: 0 <数字>rpx` 是同一件事的另一种写法，一并禁）。
 * B. 右槽右缘 = 胶囊左边线 − RIGHT_SLOT_CLEARANCE：内容盒偏移由 margin-right 补偿。
 * C. 工具栏（showToolbar + #toolbar）是标题栏**之下的第二行**：根层兄弟节点、
 *    默认关闭 ⇒ 现有调用方零 diff；它不做胶囊让宽，否则「第二行」只剩和右槽一样的宽度。
 *
 * 分类：接线守护（读源码形状断言），不构成 ③ 门证据；判别力由注入违规样本的必红面自证。
 * 行为面（第二行真渲染、动作真在标题栏之下、真不与胶囊相交）无自动化守护 —— 与
 * appNavBarLeftEntryContract 同口径：由发版冒烟的真机页级检查兜底（ADR-0043）。
 */
const h = require('./contractHarness');

const SRC_REL = 'components/app-nav-bar/app-nav-bar.uvue';

/** 抹注释（块 + 行）：注释里出现的字段名不是接线。先例 appNavBarLeftEntryContract。 */
function stripComments(s) {
  return s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

/** 取一条 `.class { ... }` 规则块；未命中返回 ''。 */
function ruleBlock(src, selector) {
  const re = new RegExp(selector.replace(/[.*+?^${}()|[\]\\]/g, '\\$&') + '\\s*\\{[^}]*\\}');
  const m = src.match(re);
  return m ? m[0] : '';
}

/** 胶囊让位 + 工具栏违规清单：真实文件与注入样本共用同一检测路径。 */
function capsuleToolbarViolations(srcRaw) {
  const src = stripComments(srcRaw);
  const v = [];

  // ── A. 胶囊位置动态取，不写死右侧预留 ──
  if (!/useSafeArea\s*\(\s*\)/.test(src)) {
    v.push('页头件未消费 useSafeArea（胶囊位置须来自 utils/system.uts 单一真源）');
  }
  if (/getMenuButtonBoundingClientRect/.test(src)) {
    v.push('页头件自己调 getMenuButtonBoundingClientRect（量测应只在 utils/system.uts）');
  }
  if (/padding-right\s*:\s*\d+rpx/.test(src)) {
    v.push('页头件出现写死的右向 padding 魔数（#1848 死件形态；胶囊宽度必须由测量结果算）');
  }
  // 槽容器的简写写法是同一件事：`padding: 0 <数字>rpx` 把右向预留又写死了。
  for (const sel of ['.nav-bar-body', '.nav-bar-right', '.nav-bar-toolbar']) {
    const blk = ruleBlock(src, sel);
    if (blk !== '' && /padding[^;{}]*\d+rpx/.test(blk)) {
      v.push(`${sel} 出现 rpx 数值内边距（右向预留须由胶囊测量驱动，不写死）`);
    }
  }
  if (!/rightSlotWidth/.test(src)) {
    v.push('右槽未用 rightSlotWidth（让宽须由 getMenuButtonBoundingClientRect 的结果驱动）');
  }

  // ── B. 右槽右缘几何 ──
  if (!/margin-right/.test(src)) {
    v.push('右槽缺 margin-right 补偿（内容盒偏移未抵消 ⇒ 图标落进胶囊矩形）');
  }
  if (!/\(750\s*-\s*NAV_BODY_PADDING_X\)\s*-\s*\(capsuleLeft\s*-\s*RIGHT_SLOT_CLEARANCE\)/.test(src)) {
    v.push('右槽右缘几何不是 (750 − paddingX) − (capsuleLeft − clearance)');
  }

  // ── C. 工具栏：显式开关 + 默认关 + 根层兄弟 + 单行 + 不做胶囊让宽 ──
  if (!/showToolbar\s*\?:\s*boolean/.test(src)) {
    v.push('缺 showToolbar prop（工具栏须显式开关，不靠 $slots 探测）');
  }
  if (!/showToolbar\s*:\s*false/.test(src)) {
    v.push('showToolbar 默认值不是 false（现有调用方会凭空多一行）');
  }
  if (!/<slot\s+name="toolbar">/.test(src)) {
    v.push('缺 #toolbar 插槽');
  }

  const toolbarTag = (src.match(/<view[^>]*class="nav-bar-toolbar"[^>]*>/) || [''])[0];
  if (toolbarTag === '') {
    v.push('工具栏节点缺失（class="nav-bar-toolbar"）');
  } else {
    if (!/v-if="showToolbar"/.test(toolbarTag)) {
      v.push('工具栏未用 v-if="showToolbar" 开合（默认开会让所有页面多一行）');
    }
    // 让宽只能靠布局位置（第二行在胶囊之下），不能挂 rightSlotStyle 再挤一次。
    if (/rightSlotStyle|:style/.test(toolbarTag)) {
      v.push('工具栏节点挂了胶囊让宽（第二行在标题栏之下，让宽等于把挤压原样搬下来）');
    }
  }

  const bodyAt = src.indexOf('class="nav-bar-body"');
  const toolbarAt = src.indexOf('class="nav-bar-toolbar"');
  if (bodyAt >= 0 && toolbarAt >= 0 && toolbarAt < bodyAt) {
    v.push('工具栏出现在标题栏之前（必须在 .nav-bar-body 之下作第二行）');
  }
  // 结构位：模板按 4 空格缩进 ⇒ 根层子节点是 8 空格。缩进变深就是被塞进 .nav-bar-body
  // 成了第三列 —— 正则看「紧跟某个 </view>」分辨不出这种嵌套（那两种写法都命中）。
  if (toolbarAt >= 0 && !/^ {8}<view[^>]*v-if="showToolbar"[^>]*class="nav-bar-toolbar"/m.test(src)) {
    v.push('工具栏不在根层（8 空格缩进）⇒ 疑似被塞进 .nav-bar-body 与左右槽挤同一行');
  }

  const toolbar = ruleBlock(src, '.nav-bar-toolbar');
  if (toolbar === '') {
    v.push('.nav-bar-toolbar 规则缺失');
  } else {
    if (!/flex-direction\s*:\s*row/.test(toolbar)) {
      v.push('.nav-bar-toolbar 缺 flex-direction: row（uvue view 默认 column ⇒ 动作竖排）');
    }
    if (!/height\s*:\s*88rpx/.test(toolbar)) {
      v.push('.nav-bar-toolbar 缺 height: 88rpx（与标题栏同高，触摸目标 44×44）');
    }
    if (!/padding\s*:\s*0\s+\$spacing-md/.test(toolbar)) {
      v.push('.nav-bar-toolbar 缺 padding: 0 $spacing-md（与标题栏同左右内边距）');
    }
    if (/capsuleLeft|rightSlotWidth|rightSlotStyle/.test(toolbar)) {
      v.push('工具栏做了胶囊让宽（第二行在标题栏之下，让宽等于把挤压原样搬下来）');
    }
  }

  return v;
}

/* ══ 真实文件（必不红面） ══ */
describe('页头胶囊让位 + 标题栏下工具栏契约（真实源码）', () => {
  const src = h.read(SRC_REL);

  it('守约：胶囊让位几何 + 工具栏默认关闭，零违规', () => {
    expect(capsuleToolbarViolations(src)).toEqual([]);
  });

  it('工具栏在标题栏之下：根层（8 空格）兄弟节点，插槽是 #toolbar', () => {
    expect(src.indexOf('class="nav-bar-toolbar"')).toBeGreaterThan(src.indexOf('class="nav-bar-body"'));
    expect(src).toMatch(/^ {8}<view v-if="showToolbar" class="nav-bar-toolbar">\s*\n\s*<slot name="toolbar">/m);
  });
});

/* ══ 判别力（必红面）：对真实源码注入违规，检测器必须红 ══ */
describe('检测器判别力（注入违规样本必红）', () => {
  const REAL = h.read(SRC_REL);

  /** 变异必须真的落地 —— 否则「必红」是假自证（源码漂移会让 replace 静默空转）。 */
  function mutated(re, repl) {
    const out = REAL.replace(re, repl);
    expect(out).not.toBe(REAL);
    return out;
  }

  function expectRed(re, repl) {
    // 断言写成 not.toEqual([]) 而不是布尔：红的时候直接打出检测器看到的违规清单。
    expect(capsuleToolbarViolations(mutated(re, repl))).not.toEqual([]);
  }

  it('A: 写死右向 padding（历史死件形态）', () => {
    expectRed(
      /(\.nav-bar-toolbar\s*\{[^}]*?)padding:\s*0\s*\$spacing-md;/,
      '$1padding-right: 80rpx;',
    );
  });

  it('A: 页头件自己量胶囊（绕过 system.uts 单一真源）', () => {
    expectRed(
      /const safeArea = useSafeArea\(\)/,
      'const safeArea = useSafeArea(); const rect = uni.getMenuButtonBoundingClientRect()',
    );
  });

  it('B: 右槽几何丢掉内容盒补偿', () => {
    expectRed(
      /const marginRight = Math\.max\(0, \(750 - NAV_BODY_PADDING_X\) - \(capsuleLeft - RIGHT_SLOT_CLEARANCE\)\)/,
      'const marginRight = 0',
    );
  });

  it('A: 槽容器用简写把右向预留写死（padding: 0 <n>rpx）', () => {
    expectRed(
      /(\.nav-bar-toolbar\s*\{[^}]*?)padding:\s*0\s*\$spacing-md;/,
      '$1padding: 0 40rpx;',
    );
  });

  it('C: 工具栏未开合（默认开 ⇒ 全仓多一行）', () => {
    expectRed(/<view v-if="showToolbar" class="nav-bar-toolbar">/, '<view class="nav-bar-toolbar">');
  });

  it('C: 工具栏被塞进 .nav-bar-body（缩进 12 空格 ⇒ 与左右槽挤同一行）', () => {
    expectRed(
      /^ {8}<view v-if="showToolbar" class="nav-bar-toolbar">/m,
      '            <view v-if="showToolbar" class="nav-bar-toolbar">',
    );
  });

  it('C: 工具栏节点挂回 rightSlotStyle（把让宽搬下来）', () => {
    expectRed(
      /<view v-if="showToolbar" class="nav-bar-toolbar">/,
      '<view v-if="showToolbar" class="nav-bar-toolbar" :style="rightSlotStyle">',
    );
  });

  it('C: 工具栏默认值改成 true', () => {
    expectRed(/showToolbar:\s*false/, 'showToolbar: true');
  });

  it('C: 工具栏缺 row（动作竖排）', () => {
    expectRed(
      /(\.nav-bar-toolbar\s*\{[^}]*?)flex-direction:\s*row;/,
      '$1',
    );
  });

  it('C: 工具栏做了胶囊让宽（第二行被重新挤窄）', () => {
    expectRed(
      /(\.nav-bar-toolbar\s*\{[^}]*?)height:\s*88rpx;/,
      '$1margin-right: (750 - capsuleLeft)rpx; height: 88rpx;',
    );
  });

  it('C: 插槽名被改回 right（页面传 #toolbar 不生效）', () => {
    expectRed(/<slot name="toolbar"><\/slot>/, '<slot name="right"></slot>');
  });
});
