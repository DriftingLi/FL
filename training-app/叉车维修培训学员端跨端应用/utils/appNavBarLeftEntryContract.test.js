/**
 * 页头左入口代发契约（refs #1677）
 *
 * 现象（#1677 B）：dashboard 传 `:left-icon="''"` ⇒ 左槽内 text 无渲染 ⇒ 左槽 view
 * 零宽零高，`onLeftClick` 命中区不存在；`.nav-bar-title-block` 原本无 @click ⇒
 * 页头左入口动作（证件切换）在全命中面点不动。
 *
 * 修复（#1677）：标题块挂 `@click="onTitleBlockClick"`，handler 内**仅左槽空置时**
 * （leftIcon.length == 0）上移发同一条 leftClick；左槽有 icon（返回箭头）时标题是纯
 * 展示文字，点击不得触发左动作 —— 否则全仓子页点标题即误返回。标题块高度 88rpx +
 * 内容垂直居中，把命中区从「文字高」抬到整条 bar 高（视觉不变，触摸目标 44×44）。
 *
 * 分类：接线守护（读源码形状断言），不构成 ③ 门证据；判别力由注入违规样本的必红面自证，
 * 行为面（真点击开合浮层）由 ② 门 automator 取证覆盖。
 */
const h = require('./contractHarness');

const SRC_REL = 'components/app-nav-bar/app-nav-bar.uvue';

/** 抹注释（块 + 行，防注释文本里的字段名被误判成真接线）。先例 certSelectorOverlayContract。 */
function stripComments(s) {
  return s.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

/** 左入口代发违规清单：真实文件与注入样本共用同一检测路径。 */
function leftEntryViolations(srcRaw) {
  const src = stripComments(srcRaw);
  const v = [];
  if (!/@click="onTitleBlockClick"/.test(src)) {
    v.push('标题块缺 @click="onTitleBlockClick"（左槽空置时左入口无命中区）');
  }
  if (!/function onTitleBlockClick\(\)[\s\S]*?\{[\s\S]*?\}/.test(src)) {
    v.push('onTitleBlockClick handler 缺失');
    return v;
  }
  const fn = (src.match(/function onTitleBlockClick\(\)[\s\S]*?\{[\s\S]*?\}/) || [''])[0];
  if (!/props\.leftIcon\.length == 0/.test(fn)) {
    v.push('onTitleBlockClick 缺左槽空置守卫（props.leftIcon.length == 0）—— 无守卫时全仓子页点标题即触发返回');
  }
  if (!/emit\(\s*'leftClick'\s*\)/.test(fn)) {
    v.push('onTitleBlockClick 未发 leftClick（未上移进同一条左入口链）');
  }
  if (!/height\s*:\s*88rpx\s*;/.test((src.match(/\.nav-bar-title-block\s*\{[^}]*\}/) || [''])[0])) {
    v.push('.nav-bar-title-block 缺 height:88rpx（命中区未抬到整条 bar 高，触摸目标不足 44×44）');
  }
  if (!/justify-content\s*:\s*center\s*;/.test((src.match(/\.nav-bar-title-block\s*\{[^}]*\}/) || [''])[0])) {
    v.push('.nav-bar-title-block 缺 justify-content:center（占满 bar 高后内容须垂直居中，否则标题视觉下移）');
  }
  return v;
}

/* ══ 真实文件（必不红面） ══ */
describe('页头左入口代发契约（真实源码，#1677）', () => {
  const src = h.read(SRC_REL);

  it('守约：标题块接 @click + 空槽守卫代发 leftClick + 命中区 88rpx 垂直居中，零违规', () => {
    expect(leftEntryViolations(src)).toEqual([]);
  });

  it('左槽 @click 链仍在（左槽有 icon 时的原返回入口不被拆）', () => {
    // 判据容忍标签内插入其它属性（2026-10-27 插了抬层 :class，2026-11-10 起值是 titleFloating）。
    // 用 [^>]* 把范围钉在**同一个标签内** —— 放宽成 [\s\S]* 会跨标签，@click 落到别家也算命中（假绿）。
    expect(src).toMatch(/class="nav-bar-side nav-bar-left"[^>]*@click="onLeftClick"/);
  });
});

/* ══ 判别力（必红面）：注入违规样本，检测器必须红 ══ */
describe('检测器判别力（注入违规样本必红）', () => {
  const BASE = [
    '<view class="nav-bar-title-block" @click="onTitleBlockClick"></view>',
    'function onTitleBlockClick() : void {',
    '    if (props.leftIcon.length == 0) {',
    "        emit('leftClick')",
    '    }',
    '}',
    '<style lang="scss">',
    '.nav-bar-title-block {',
    '    height: 88rpx;',
    '    justify-content: center;',
    '}',
    '</style>',
  ].join('\n');
  it('标题块去掉 @click → 报「缺 @click」', () => {
    const v = leftEntryViolations(BASE.replace(' @click="onTitleBlockClick"', ''));
    expect(v.some((x) => x.includes('缺 @click'))).toBe(true);
  });

  it('handler 去掉左槽空置守卫（无条件代发）→ 报「缺守卫」', () => {
    const v = leftEntryViolations(BASE.replace('    if (props.leftIcon.length == 0) {\n', '').replace('    }\n', ''));
    expect(v.some((x) => x.includes('缺左槽空置守卫'))).toBe(true);
  });

  it('handler 去掉 emit → 报「未发 leftClick」', () => {
    const v = leftEntryViolations(BASE.replace("        emit('leftClick')", ''));
    expect(v.some((x) => x.includes('未发 leftClick'))).toBe(true);
  });

  it('标题块高度去掉（命中区塌回文字高）→ 报「缺 height:88rpx」', () => {
    const v = leftEntryViolations(BASE.replace('    height: 88rpx;\n', ''));
    expect(v.some((x) => x.includes('缺 height:88rpx'))).toBe(true);
  });

  it('占满 bar 高后去掉垂直居中 → 报「缺 justify-content:center」', () => {
    const v = leftEntryViolations(BASE.replace('    justify-content: center;\n', ''));
    expect(v.some((x) => x.includes('缺 justify-content:center'))).toBe(true);
  });

  it('修复前 master 形状（标题块无 @click 无高度）→ 至少两处红', () => {
    const PRE = [
      '<view class="nav-bar-title-block" :class="{ \'nav-bar-title-block--center\': !leftOverlap }">',
      '<style lang="scss">',
      '.nav-bar-title-block {',
      '    flex-direction: column;',
      '    align-items: flex-start;',
      '    margin-left: 16rpx;',
      '}',
      '</style>',
    ].join('\n');
    const v = leftEntryViolations(PRE);
    expect(v.length).toBeGreaterThanOrEqual(2);
  });
});

/* ══════════════════════════════════════════════════════════════════════════════
 * 标题居中形态（`titleFloating`，2026-10-27 起；2026-11-10 #2402 收口）
 *
 * 起因一（2026-10-27）：AI 助手页左槽补上 ☰ 之后，§5.1 的「左槽有字 ⇒ 标题左对齐」把它变成
 * 「☰ AI 助手 ⋯」挤在左上角；聊天页要的是三段式居中 ⇒ 加了 `centerTitle` prop。
 *
 * 起因二（#2402）：`--center`（流内居中）根本不是居中 —— 右槽为让开小程序胶囊带
 * width + 大段 margin-right，流内标题只能在「左槽 → 右槽」的剩余宽度里居中，视线上必然偏左
 * （真机复现页：我的）。于是判定收口成一处：**左槽空置且有标题就脱流居中**，
 * `centerTitle` 降级为「左槽有字也要居中」的覆盖开关，`--center` 形态类整体删除。
 *
 * 这个形态**必须脱流**才是真居中，这是全仓唯一一种居中实现。
 *
 * 脱流带来一个**静默失效**风险，正是本组要锁的东西：全宽标题块会盖住左槽 ☰ 与右槽 ⋯，
 * 两层都点不动 —— 不报错、不崩、只是点了没反应。本仓无 `pointer-events` 先例（全仓 0 处），
 * 所以靠 z-index 分层：两侧槽 `position:relative + z-index` 抬到标题块之上。
 * 同理 `.nav-bar-body` 必须有 `position:relative`，否则脱流的标题块找不到定位包含块、
 * 会跑到整个页面的原点去。
 * ══════════════════════════════════════════════════════════════════════════════ */

/** 标题居中形态违规清单：真实文件与注入样本共用同一检测路径。 */
function centerTitleViolations(srcRaw) {
  const src = stripComments(srcRaw);
  const v = [];
  if (!/centerTitle\s*\?:\s*boolean/.test(src)) {
    v.push('缺 centerTitle prop 声明');
  }
  if (!/centerTitle\s*:\s*false/.test(src)) {
    v.push('centerTitle 默认值不是 false（现有调用方会被动变形，违反零 diff）');
  }
  if (!/if\s*\(\s*props\.centerTitle\s*\)/.test(src)) {
    v.push('titleFloating 缺 centerTitle 直通分支（prop 形同虚设）');
  }
  if (!/nav-bar-title-block--floating-center/.test(src)) {
    v.push('缺 floating-center 形态类');
  }
  // 两侧槽**都要**挂 --front：漏一个，那一个就点不动。判据看 titleFloating 而不是 centerTitle ——
  // #2402 起自动居中（左槽空置）同样脱流，两侧槽若不跟着抬层，☰/⋯ 会一起被盖住。
  const frontCount = (src.match(/nav-bar-side--front':\s*titleFloating/g) || []).length;
  if (frontCount < 2) {
    v.push(`两侧槽只挂 ${frontCount} 处 --front（应为 2：左 ☰ 与右 ⋯ 都会被脱流标题块盖住）`);
  }
  // #2402 收口：居中判定只允许一处 —— 左槽空置且有标题 ⇒ 自动脱流，不再逐页加 prop
  const trigger = (src.match(/const titleFloating\s*=\s*computed[\s\S]*?\}\)\n/) || [''])[0];
  if (!trigger) {
    v.push('缺 titleFloating 判定（页头居中的唯一开关，见 #2402）');
  } else {
    if (!/props\.centerTitle/.test(trigger)) {
      v.push('titleFloating 丢 centerTitle 直通（AI 助手「☰ 标题 ⋯」三段式会翻车）');
    }
    if (!/!leftOverlap\.value/.test(trigger)) {
      v.push('titleFloating 丢「左槽空置」分支（!leftOverlap）—— tabBar 页会退回逐页偏左');
    }
    if (!/props\.title\.length\s*>\s*0/.test(trigger)) {
      v.push('titleFloating 丢非空标题守卫（空标题脱流 ⇒ 长出 574rpx×88rpx 的透明点击区）');
    }
  }
  // 死件防复活：流内居中（--center）已随 #2402 删除，居中只剩脱流这一种实现
  if (/nav-bar-title-block--center/.test(src)) {
    v.push('nav-bar-title-block--center 复活（流内居中不成立，居中只允许脱流，见 #2402）');
  }
  const floating = (src.match(/\.nav-bar-title-block--floating-center\s*\{[^}]*\}/) || [''])[0];
  const floatingRules = [
    ['position:\\s*absolute', 'position:absolute（不脱流就只能在剩余宽度里居中）'],
    ['left:\\s*88rpx', 'left:88rpx（两侧对称内缩，避让左槽 [24,68]rpx 的几何兜底）'],
    ['width:\\s*574rpx', 'width:574rpx（= 750 − 88×2，内缩而非铺满全宽）'],
    ['align-items:\\s*center', 'align-items:center'],
  ];
  for (const [re, label] of floatingRules) {
    if (!new RegExp(re).test(floating)) v.push(`floating-center 缺 ${label}`);
  }
  // 铺满全宽 = 回到「透明块盖住左槽」的形态：z-index 一旦不生效，☰ 就点不动
  if (/width\s*:\s*100%/.test(floating)) {
    v.push('floating-center 宽度铺满全宽（会盖住左槽 ☰ ⇒ 只靠 z-index 单点兜底）');
  }
  const front = (src.match(/\.nav-bar-side--front\s*\{[^}]*\}/) || [''])[0];
  if (!/z-index\s*:\s*\d+/.test(front)) {
    v.push('side--front 缺 z-index（脱流标题块会盖住两侧槽 ⇒ ☰ 与 ⋯ 点了没反应）');
  }
  if (!/position\s*:\s*relative/.test(front)) {
    v.push('side--front 缺 position:relative（z-index 对 static 元素不生效）');
  }
  const body = (src.match(/\.nav-bar-body\s*\{[^}]*\}/) || [''])[0];
  if (!/position\s*:\s*relative/.test(body)) {
    v.push('.nav-bar-body 缺 position:relative（脱流标题块无定位包含块，会跑出页头）');
  }
  return v;
}

describe('标题居中形态（titleFloating，2026-10-27 / #2402 收口）', () => {
  const src = h.read(SRC_REL);

  it('守约：prop 默认关 + 脱流居中 + 两侧槽抬层 + 包含块齐备，零违规', () => {
    expect(centerTitleViolations(src)).toEqual([]);
  });

  it('默认值必须是 false（现有调用方零 diff —— 本仓多处页头靠 leftOverlap 走左对齐）', () => {
    const defaults = (src.match(/divider:\s*false[^\n]*\n?\s*centerTitle:\s*false/) || []).length;
    expect(defaults).toBe(1);
  });
});

describe('居中形态检测器判别力（注入违规样本必红）', () => {
  const BASE = [
    '<view class="nav-bar-side nav-bar-left" :class="{ \'nav-bar-side--front\': titleFloating }"></view>',
    '<view v-else class="nav-bar-side nav-bar-right" :class="{ \'nav-bar-side--front\': titleFloating }"></view>',
    'centerTitle ?: boolean',
    'centerTitle: false',
    'const titleFloating = computed((): boolean => {',
    '    if (props.centerTitle) {',
    '        return true',
    '    }',
    '    return props.title.length > 0 && !leftOverlap.value',
    '})',
    '<style lang="scss">',
    '.nav-bar-body { position: relative; }',
    '.nav-bar-side--front { position: relative; z-index: 2; }',
    '.nav-bar-title-block--floating-center {',
    '    position: absolute;',
    '    left: 88rpx;',
    '    top: 0;',
    '    width: 574rpx;',
    '    align-items: center;',
    '    margin-left: 0;',
    '    z-index: 1;',
    '}',
    '</style>',
  ].join('\n');

  it('基准样本零违规（检测器自身不空跑）', () => {
    expect(centerTitleViolations(BASE)).toEqual([]);
  });

  it('默认值改成 true → 报「默认值不是 false」', () => {
    const v = centerTitleViolations(BASE.replace('centerTitle: false', 'centerTitle: true'));
    expect(v.some((x) => x.includes('默认值不是 false'))).toBe(true);
  });

  it('漏挂一处 --front（右槽点不动）→ 报「只挂 1 处」', () => {
    const v = centerTitleViolations(
      BASE.replace(' :class="{ \'nav-bar-side--front\': titleFloating }"></view>', '></view>')
    );
    expect(v.some((x) => x.includes('只挂 1 处'))).toBe(true);
  });

  it('自动居中分支被摘掉（退回逐页加 prop）→ 报「左槽空置」', () => {
    const v = centerTitleViolations(
      BASE.replace('return props.title.length > 0 && !leftOverlap.value', 'return false')
    );
    expect(v.some((x) => x.includes('左槽空置'))).toBe(true);
  });

  it('去掉非空标题守卫（空标题也脱流）→ 报「透明点击区」', () => {
    const v = centerTitleViolations(
      BASE.replace('return props.title.length > 0 && !leftOverlap.value', 'return !leftOverlap.value')
    );
    expect(v.some((x) => x.includes('透明点击区'))).toBe(true);
  });

  it('复活流内居中 --center → 报「复活」（居中只允许脱流一种实现）', () => {
    const v = centerTitleViolations(
      BASE.replace('</style>', '.nav-bar-title-block--center { align-items: center; }</style>')
    );
    expect(v.some((x) => x.includes('复活'))).toBe(true);
  });

  it('两侧槽去掉 z-index → 报「缺 z-index」（这是「点了没反应」的唯一防线）', () => {
    const v = centerTitleViolations(
      BASE.replace('.nav-bar-side--front { position: relative; z-index: 2; }', '.nav-bar-side--front { position: relative; }')
    );
    expect(v.some((x) => x.includes('缺 z-index'))).toBe(true);
  });

  it('title-block 去掉 position:absolute → 报「不脱流」', () => {
    const v = centerTitleViolations(BASE.replace('    position: absolute;\n', ''));
    expect(v.some((x) => x.includes('不脱流'))).toBe(true);
  });

  it('宽度改成铺满全宽（透明块盖住左槽 ☰）→ 报「铺满全宽」', () => {
    const v = centerTitleViolations(BASE.replace('    width: 574rpx;', '    width: 100%;'));
    expect(v.some((x) => x.includes('铺满全宽'))).toBe(true);
  });

  it('.nav-bar-body 去掉 position:relative → 报「无定位包含块」', () => {
    const v = centerTitleViolations(BASE.replace('.nav-bar-body { position: relative; }', '.nav-bar-body { }'));
    expect(v.some((x) => x.includes('无定位包含块'))).toBe(true);
  });
});
