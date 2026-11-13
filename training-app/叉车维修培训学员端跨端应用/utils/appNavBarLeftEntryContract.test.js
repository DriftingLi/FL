/**
 * 页头左入口接线契约（refs #1677 / #2479）
 *
 * 现象（#1677 B）：dashboard 传 `:left-icon="''"` ⇒ 左槽内 text 无渲染 ⇒ 左槽 view
 * 零宽零高，`onLeftClick` 命中区不存在；`.nav-bar-title-block` 原本无 @click ⇒
 * 页头左入口动作（证件切换）在全命中面点不动。
 *
 * 沿革（#1677）：标题块挂 `@click="onTitleBlockClick"`，handler 内**仅左槽空置时**
 * （leftIcon.length == 0）上移发同一条 leftClick。标题块高度 88rpx + 内容垂直居中，
 * 把命中区从「文字高」抬到整条 bar 高（视觉不变，触摸目标 44×44）—— 这两条本契约保留。
 *
 * 改判（#2479 D3，2026-11-12）：`#1677` 的「左槽空置时标题代发 leftClick」是隐式规则 ——
 * 页面把左入口动作绑在**标题**上，读代码看不出接线。现改为显式 `titleClick`：标题块点击
 * 一律 `emit('titleClick')`，页面用 `@title-click` 接（dashboard 证件切换）。左槽空置守卫
 * 与代发 leftClick 路径一并删除，本契约钉「必须是 titleClick，且不得再代发 leftClick /
 * 不得再带左槽空置守卫」。
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
    v.push('标题块缺 @click="onTitleBlockClick"（标题命中区无接线）');
  }
  if (!/function onTitleBlockClick\(\)[\s\S]*?\{[\s\S]*?\}/.test(src)) {
    v.push('onTitleBlockClick handler 缺失');
    return v;
  }
  const fn = (src.match(/function onTitleBlockClick\(\)[\s\S]*?\{[\s\S]*?\}/) || [''])[0];
  if (!/emit\(\s*'titleClick'\s*\)/.test(fn)) {
    v.push("onTitleBlockClick 未发 titleClick（#2479 D3：标题点击必须是显式 titleClick 事件）");
  }
  if (/emit\(\s*'leftClick'\s*\)/.test(fn)) {
    v.push('onTitleBlockClick 仍在代发 leftClick（#2479 D3 已删「左槽空置时标题代发 leftClick」隐式规则）');
  }
  if (/props\.leftIcon\.length\s*==\s*0/.test(fn)) {
    v.push('onTitleBlockClick 仍带左槽空置守卫（守卫随代发规则一并删除，点击一律发 titleClick）');
  }
  const emitsDecl = (src.match(/defineEmits\(\s*\[[^\]]*\]\s*\)/) || [''])[0];
  if (!/['"]titleClick['"]/.test(emitsDecl)) {
    v.push("defineEmits 未声明 titleClick（父级 @title-click 收不到事件）");
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
describe('页头左入口接线契约（真实源码，#1677 / #2479）', () => {
  const src = h.read(SRC_REL);

  it('守约：标题块接 @click + 一律 emit titleClick + 命中区 88rpx 垂直居中，零违规', () => {
    expect(leftEntryViolations(src)).toEqual([]);
  });

  it('左槽 @click 链仍在（左槽有 icon 时的原返回入口不被拆）', () => {
    // 判据容忍标签内插入其它属性（2026-10-27 插了抬层 :class，2026-11-10 起值是 titleCentered）。
    // 用 [^>]* 把范围钉在**同一个标签内** —— 放宽成 [\s\S]* 会跨标签，@click 落到别家也算命中（假绿）。
    expect(src).toMatch(/class="nav-bar-side nav-bar-left"[^>]*@click="onLeftClick"/);
  });
});

/* ══ 判别力（必红面）：注入违规样本，检测器必须红 ══ */
describe('检测器判别力（注入违规样本必红）', () => {
  const BASE = [
    "defineEmits(['leftClick', 'rightClick', 'titleClick'])",
    '<view class="nav-bar-title-block" @click="onTitleBlockClick"></view>',
    'function onTitleBlockClick() : void {',
    "    emit('titleClick')",
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

  it('handler 改回代发 leftClick（#2479 已删的隐式规则）→ 报「仍在代发 leftClick」', () => {
    const v = leftEntryViolations(BASE.replace("    emit('titleClick')", "    emit('leftClick')"));
    expect(v.some((x) => x.includes('仍在代发 leftClick'))).toBe(true);
  });

  it('handler 加回左槽空置守卫 → 报「仍带左槽空置守卫」', () => {
    const v = leftEntryViolations(
      BASE.replace("    emit('titleClick')", "    if (props.leftIcon.length == 0) {\n        emit('titleClick')\n    }")
    );
    expect(v.some((x) => x.includes('仍带左槽空置守卫'))).toBe(true);
  });

  it('handler 去掉 emit → 报「未发 titleClick」', () => {
    const v = leftEntryViolations(BASE.replace("    emit('titleClick')\n", ''));
    expect(v.some((x) => x.includes('未发 titleClick'))).toBe(true);
  });

  it('defineEmits 漏声明 titleClick → 报「未声明 titleClick」', () => {
    const v = leftEntryViolations(
      BASE.replace("defineEmits(['leftClick', 'rightClick', 'titleClick'])", "defineEmits(['leftClick', 'rightClick'])")
    );
    expect(v.some((x) => x.includes('未声明 titleClick'))).toBe(true);
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
 * 标题居中形态（`titleCentered`，2026-10-27 起；#2402 收口；#2442 改几何）
 *
 * 起因一（2026-10-27）：AI 助手页左槽补上 ☰ 之后，§5.1 的「左槽有字 ⇒ 标题左对齐」把它变成
 * 「☰ AI 助手 ⋯」挤在左上角；聊天页要的是三段式居中 ⇒ 加了 `centerTitle` prop。
 *
 * 起因二（#2402）：`--center`（流内居中）不是居中 —— 右槽为让开小程序胶囊带 width + 大段
 * margin-right，流内标题只能在「左槽 → 右槽」的剩余宽度里居中，视线上必然偏左。
 * 当时的解法是脱流定宽（left 88 / width 574）。
 *
 * 起因三（#2442 用户补充规则）：**一级导航即使存在右侧操作，也允许标题居中**，但标题必须以
 * 导航栏整体可用空间为约束，不得与右侧操作、微信胶囊重叠。脱流定宽 574rpx 违反这条 ——
 * 它的右缘 662rpx 越过胶囊左边线（典型 556rpx），长标题直接钻到胶囊下面。
 *
 * 现形态：标题块留在流内、`flex: 1` 吃掉「左槽右缘 → 右槽左缘」这段可用空间，居中在**这段
 * 空间**里。右槽被 margin-right 钉在胶囊左边线 − 间隙 ⇒ 标题块右缘天然止于右槽左缘，
 * 几何上不可能进胶囊、也不可能盖住左槽。因此不再需要 position:absolute / 定宽 / z-index 兜底。
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
    v.push('titleCentered 缺 centerTitle 直通分支（prop 形同虚设）');
  }
  if (!/nav-bar-title-block--centered/.test(src)) {
    v.push('缺 centered 形态类');
  }
  // 居中判定只允许一处 —— 左槽空置且有标题 ⇒ 自动居中，不再逐页加 prop
  const trigger = (src.match(/const titleCentered\s*=\s*computed[\s\S]*?\}\)\n/) || [''])[0];
  if (!trigger) {
    v.push('缺 titleCentered 判定（页头居中的唯一开关，见 #2402 / #2442）');
  } else {
    if (!/props\.centerTitle/.test(trigger)) {
      v.push('titleCentered 丢 centerTitle 直通（AI 助手「☰ 标题 ⋯」三段式会翻车）');
    }
    if (!/!leftOverlap\.value/.test(trigger)) {
      v.push('titleCentered 丢「左槽空置」分支（!leftOverlap）—— tabBar 页会退回逐页偏左');
    }
    if (!/props\.title\.length\s*>\s*0/.test(trigger)) {
      v.push('titleCentered 丢非空标题守卫（空标题也居中 ⇒ 标题块白占可用空间）');
    }
  }
  // 死件防复活：脱流定宽居中（--floating-center）与流内旧类（--center）都已随 #2442 删除
  if (/nav-bar-title-block--floating-center/.test(src)) {
    v.push('floating-center 复活（脱流定宽 574rpx 会越过胶囊左边线，见 #2442）');
  }
  if (/nav-bar-title-block--center\b/.test(src)) {
    v.push('nav-bar-title-block--center 复活（居中只允许 centered 一种实现，见 #2442）');
  }
  if (/nav-bar-side--front/.test(src)) {
    v.push('side--front 复活（标题块已回流入流，不再需要 z-index 抬层）');
  }
  // 标题块必须在流内、flex:1 —— 这是「以可用空间为约束」的几何本体
  const block = (src.match(/\.nav-bar-title-block\s*\{[^}]*\}/) || [''])[0];
  if (!/flex\s*:\s*1/.test(block)) {
    v.push('.nav-bar-title-block 缺 flex:1（不吃可用空间 ⇒ 居中点退回屏幕中心、会压胶囊）');
  }
  if (/position\s*:\s*absolute/.test(block)) {
    v.push('.nav-bar-title-block 出现 position:absolute（脱流即失去可用空间约束，见 #2442）');
  }
  const centered = (src.match(/\.nav-bar-title-block--centered\s*\{[^}]*\}/) || [''])[0];
  if (!/align-items\s*:\s*center/.test(centered)) {
    v.push('centered 缺 align-items:center');
  }
  const centeredText = (src.match(/\.nav-bar-title--centered\s*\{[^}]*\}/) || [''])[0];
  if (!/text-align\s*:\s*center/.test(centeredText)) {
    v.push('nav-bar-title--centered 缺 text-align:center');
  }
  if (!/width\s*:\s*100%/.test(centeredText)) {
    v.push('nav-bar-title--centered 缺 width:100%（不铺满则 ellipsis 不生效）');
  }
  // 空标题守卫：标题块必须 v-if，否则 flex:1 会凭空长出一块「点了会发 leftClick」的透明区
  // 注意：不能用 `<view[^>]*class="nav-bar-title-block"` —— v-if 值里含 `>`（`title.length > 0`），
  // `[^>]*` 会在那里截断，永远匹配不到 class。改用字面前缀匹配。
  const blockTag = (src.match(/<view\s+v-if="title\.length > 0 \|\| subtitle\.length > 0"\s+class="nav-bar-title-block"/) || [''])[0];
  if (blockTag.length === 0) {
    v.push('标题块缺 v-if 非空守卫（空标题下 flex:1 会长出透明点击区）');
  }
  // 居中形态的两条配套几何（#2479）：侧槽脱流 + 两端 48rpx 对称留白，缺一条中心就不是 375rpx
  const body = (src.match(/\.nav-bar-body\s*\{[^}]*\}/) || [''])[0];
  if (!/position\s*:\s*relative/.test(body)) {
    v.push('.nav-bar-body 缺 position:relative（居中形态侧槽绝对定位的包含块）');
  }
  const floatSides = [
    ['nav-bar-side--floating-left', 'left'],
    ['nav-bar-side--floating-right', 'right'],
  ];
  for (const pair of floatSides) {
    const blk = (src.match(new RegExp('\\.' + pair[0] + '\\s*\\{[^}]*\\}')) || [''])[0];
    if (blk.length === 0) {
      v.push('缺 .' + pair[0] + '（居中形态侧槽未脱流 ⇒ 空右槽的胶囊让位仍把标题中心挤左）');
      continue;
    }
    if (!/position\s*:\s*absolute/.test(blk)) {
      v.push('.' + pair[0] + ' 未 position:absolute（未出流则仍参与 space-between）');
    }
    if (!new RegExp(pair[1] + '\\s*:\\s*24rpx').test(blk)) {
      v.push('.' + pair[0] + ' 缺 ' + pair[1] + ':24rpx（与流内位置不再一致）');
    }
  }
  const floatClasses = [
    ['leftSlotFloatClass', 'nav-bar-side--floating-left'],
    ['rightSlotFloatClass', 'nav-bar-side--floating-right'],
  ];
  for (const pair of floatClasses) {
    const c = (src.match(new RegExp('const ' + pair[0] + '\\s*=\\s*computed[\\s\\S]*?\\}\\)')) || [''])[0];
    if (c.length === 0) {
      v.push('缺 ' + pair[0] + ' 计算属性（侧槽脱流没有触发面）');
      continue;
    }
    if (!/titleCentered\.value/.test(c)) {
      v.push(pair[0] + ' 未以 titleCentered 为门（非居中形态的二级页也会被脱流）');
    }
    if (!new RegExp(pair[1]).test(c)) {
      v.push(pair[0] + ' 未返回 ' + pair[1]);
    }
  }
  if (!/margin-left\s*:\s*48rpx/.test(centered)) {
    v.push('居中形态缺左 48rpx 对称留白（标题文字带左界不再是 72rpx）');
  }
  if (!/margin-right\s*:\s*48rpx/.test(centered)) {
    v.push('居中形态缺右 48rpx 对称留白（两端不对称 ⇒ 文字中心偏离 375rpx）');
  }
  return v;
}

/**
 * 二级页标题形态违规清单（#2480）：返回箭头后左对齐 + 定宽省略，不压右槽动作。
 *
 * 二级页（有返回箭头）标题**不居中**（ADR-0067 决定 5）：标题块 align-items:flex-start，
 * 标题文字无 text-align:center。标题必须铺满标题块内容盒（width:100%），否则 lines:1 +
 * text-overflow:ellipsis 不生效，长标题按 max-content 撑出块、压到右槽动作。
 */
function secondaryTitleViolations(srcRaw) {
  const src = stripComments(srcRaw);
  const v = [];
  const block = (src.match(/\.nav-bar-title-block\s*\{[^}]*\}/) || [''])[0];
  if (block === '') {
    v.push('.nav-bar-title-block 规则缺失');
  } else if (!/align-items\s*:\s*flex-start/.test(block)) {
    v.push('非居中标题块缺 align-items:flex-start（二级页标题须在返回箭头后左对齐）');
  }
  const title = (src.match(/\.nav-bar-title\s*\{[^}]*\}/) || [''])[0];
  if (title === '') {
    v.push('.nav-bar-title 规则缺失');
  } else {
    if (/text-align\s*:\s*center/.test(title)) {
      v.push('.nav-bar-title 出现 text-align:center（二级页标题不得居中）');
    }
    if (!/width\s*:\s*100%/.test(title)) {
      v.push('.nav-bar-title 缺 width:100%（不铺满则 lines:1 + ellipsis 不生效，长标题压右槽动作）');
    }
  }
  return v;
}

describe('标题居中形态（titleCentered，2026-10-27 / #2402 / #2442）', () => {
  const src = h.read(SRC_REL);

  it('守约：prop 默认关 + 流内 flex:1 居中 + 非空守卫，零违规', () => {
    expect(centerTitleViolations(src)).toEqual([]);
  });

  it('默认值必须是 false（现有调用方零 diff —— 本仓多处页头靠 leftOverlap 走左对齐）', () => {
    const defaults = (src.match(/divider:\s*false[^\n]*\n?\s*centerTitle:\s*false/) || []).length;
    expect(defaults).toBe(1);
  });
});

describe('居中形态检测器判别力（注入违规样本必红）', () => {
  const BASE = [
    '<view v-if="title.length > 0 || subtitle.length > 0" class="nav-bar-title-block" :class="titleBlockClass"></view>',
    'centerTitle ?: boolean',
    'centerTitle: false',
    'const titleCentered = computed((): boolean => {',
    '    if (props.centerTitle) {',
    '        return true',
    '    }',
    '    return props.title.length > 0 && !leftOverlap.value',
    '})',
    'const titleBlockClass = computed((): string => {',
    "    return titleCentered.value ? 'nav-bar-title-block--centered' : ''",
    '})',
    'const leftSlotFloatClass = computed((): string => {',
    "    return titleCentered.value ? 'nav-bar-side--floating-left' : ''",
    '})',
    'const rightSlotFloatClass = computed((): string => {',
    "    return titleCentered.value ? 'nav-bar-side--floating-right' : ''",
    '})',
    '<style lang="scss">',
    '.nav-bar-body { position: relative; }',
    '.nav-bar-side--floating-left { position: absolute; left: 24rpx; }',
    '.nav-bar-side--floating-right { position: absolute; right: 24rpx; }',
    '.nav-bar-title-block { flex: 1; min-width: 0; }',
    '.nav-bar-title-block--centered { align-items: center; margin-left: 48rpx; margin-right: 48rpx; }',
    '.nav-bar-title--centered { width: 100%; text-align: center; margin-left: 0; }',
    '</style>',
  ].join('\n');

  it('基准样本零违规（检测器自身不空跑）', () => {
    expect(centerTitleViolations(BASE)).toEqual([]);
  });

  it('默认值改成 true → 报「默认值不是 false」', () => {
    const v = centerTitleViolations(BASE.replace('centerTitle: false', 'centerTitle: true'));
    expect(v.some((x) => x.includes('默认值不是 false'))).toBe(true);
  });

  it('自动居中分支被摘掉（退回逐页加 prop）→ 报「左槽空置」', () => {
    const v = centerTitleViolations(
      BASE.replace('return props.title.length > 0 && !leftOverlap.value', 'return false')
    );
    expect(v.some((x) => x.includes('左槽空置'))).toBe(true);
  });

  it('去掉非空标题守卫（空标题也居中）→ 报「非空标题守卫」', () => {
    const v = centerTitleViolations(
      BASE.replace('return props.title.length > 0 && !leftOverlap.value', 'return !leftOverlap.value')
    );
    expect(v.some((x) => x.includes('非空标题守卫'))).toBe(true);
  });

  it('复活脱流定宽 --floating-center → 报「floating-center 复活」', () => {
    const v = centerTitleViolations(
      BASE.replace('</style>', '.nav-bar-title-block--floating-center { position: absolute; }</style>')
    );
    expect(v.some((x) => x.includes('floating-center 复活'))).toBe(true);
  });

  it('复活两侧槽抬层 --front → 报「side--front 复活」', () => {
    const v = centerTitleViolations(
      BASE.replace('</style>', '.nav-bar-side--front { position: relative; z-index: 2; }</style>')
    );
    expect(v.some((x) => x.includes('side--front 复活'))).toBe(true);
  });

  it('标题块去掉 flex:1（居中点退回屏幕中心）→ 报「缺 flex:1」', () => {
    const v = centerTitleViolations(BASE.replace('.nav-bar-title-block { flex: 1; min-width: 0; }', '.nav-bar-title-block { min-width: 0; }'));
    expect(v.some((x) => x.includes('缺 flex:1'))).toBe(true);
  });

  it('标题块改回 position:absolute → 报「position:absolute」', () => {
    const v = centerTitleViolations(
      BASE.replace('.nav-bar-title-block { flex: 1; min-width: 0; }', '.nav-bar-title-block { flex: 1; position: absolute; }')
    );
    expect(v.some((x) => x.includes('position:absolute'))).toBe(true);
  });

  it('居中文字去掉 text-align:center → 报「缺 text-align:center」', () => {
    const v = centerTitleViolations(
      BASE.replace('.nav-bar-title--centered { width: 100%; text-align: center; margin-left: 0; }', '.nav-bar-title--centered { width: 100%; margin-left: 0; }')
    );
    expect(v.some((x) => x.includes('缺 text-align:center'))).toBe(true);
  });

  it('标题块去掉 v-if 非空守卫 → 报「缺 v-if 非空守卫」', () => {
    const v = centerTitleViolations(
      BASE.replace('<view v-if="title.length > 0 || subtitle.length > 0" class="nav-bar-title-block"', '<view class="nav-bar-title-block"')
    );
    expect(v.some((x) => x.includes('缺 v-if 非空守卫'))).toBe(true);
  });

  it('居中形态侧槽不脱流（摘掉 --floating-left）→ 报「缺 .nav-bar-side--floating-left」', () => {
    const v = centerTitleViolations(BASE.replace('.nav-bar-side--floating-left { position: absolute; left: 24rpx; }', ''));
    expect(v.some((x) => x.includes('缺 .nav-bar-side--floating-left'))).toBe(true);
  });

  it('侧槽脱流类丢 titleCentered 门（二级页也被脱流）→ 报「未以 titleCentered 为门」', () => {
    const v = centerTitleViolations(
      BASE.replace("    return titleCentered.value ? 'nav-bar-side--floating-right' : ''", "    return 'nav-bar-side--floating-right'")
    );
    expect(v.some((x) => x.includes('未以 titleCentered 为门'))).toBe(true);
  });

  it('对称留白只留一边（中心偏离 375rpx）→ 报「缺右 48rpx」', () => {
    const v = centerTitleViolations(
      BASE.replace('align-items: center; margin-left: 48rpx; margin-right: 48rpx;', 'align-items: center; margin-left: 48rpx;')
    );
    expect(v.some((x) => x.includes('缺右 48rpx'))).toBe(true);
  });

  it('.nav-bar-body 丢 position:relative（侧槽失去包含块）→ 报「缺 position:relative」', () => {
    const v = centerTitleViolations(BASE.replace('.nav-bar-body { position: relative; }', '.nav-bar-body { }'));
    expect(v.some((x) => x.includes('缺 position:relative'))).toBe(true);
  });
});

/**
 * 一级/二级页面页头形态（#2442 裁定，见 docs/adr/0067；#2479 收窄为「一级页真居中」）
 *
 * 一级页面（无返回按钮）：标题居中于**屏幕中心**（#2479，几何见上），页级动作只能放左槽，
 * 右槽必须空 —— 右槽留内容时标题长文本会钻到它下面（侧槽已脱流）。
 * 二级页面（有返回按钮）：返回 + 标题左对齐，必要操作放右侧；返回必须真能返回。
 */
describe('一级/二级页面页头形态（#2442）', () => {
  const navBar = h.read(SRC_REL);

  it('页头件开放 #left 插槽，且左槽点击按 leftIcon 收口（插槽内容自带 @click 时不重复上抛）', () => {
    expect(navBar).toMatch(/<slot name="left">/);
    expect(navBar).toMatch(/function onLeftClick\(\)[\s\S]{0,200}?props\.leftIcon\.length > 0/);
  });

  it('一级页面（left-icon=""）的单个自定义操作放左槽，右槽不再承载', () => {
    const forum = h.read('pages/forum/forum.uvue');
    expect(forum).toMatch(/<template #left>[\s\S]*?msg-icon-wrap/);
    expect(forum).not.toMatch(/<template #right>/);
    // #2479 D5：dashboard 的 🔔 下沉（通知入口另有 forum / profile / resume 三处），
    // 证件切换改挂显式 @title-click —— 左右槽都清空，标题才居中在屏幕中心。
    const dash = h.read('pages/dashboard/dashboard.uvue');
    expect(dash).not.toMatch(/<template #left>/);
    expect(dash).not.toMatch(/nav-bell/);
    expect(dash).toMatch(/@title-click="onCertTap"/);
    expect(dash).not.toMatch(/:right-icons=/);
  });

  it('一级页面（left-icon=""）右槽必须空（#2479 D5/D6：侧槽脱流后标题块吃满整行）', () => {
    const bad = [];
    for (const rel of h.sourceFilesIn('pages')) {
      const src = h.read(rel);
      for (const m of src.matchAll(/<(?:app-nav-bar|AppNavBar)\b[^>]*>/g)) {
        const tag = m[0];
        if (!/left-icon="''"|left-icon=""/.test(tag)) continue;
        const icons = tag.match(/:right-icons="([^"]*)"/);
        if (icons && !/^\s*\[\s*\]\s*$/.test(icons[1])) {
          bad.push(rel + ' :: :right-icons=' + icons[1]);
        }
        const rest = src.slice((m.index || 0) + tag.length);
        const end = rest.indexOf('</app-nav-bar>');
        if (end >= 0 && /<template #right>/.test(rest.slice(0, end))) {
          bad.push(rel + ' :: <template #right>');
        }
      }
    }
    expect(bad).toEqual([]);
  });

  it('二级页面（默认返回箭头）必须接 @left-click，不留死按钮', () => {
    const dead = [];
    for (const rel of h.sourceFilesIn('pages')) {
      const src = h.read(rel);
      for (const m of src.matchAll(/<(?:app-nav-bar|AppNavBar)\b[^>]*>/g)) {
        const tag = m[0];
        if (/left-icon=""/.test(tag) || /left-icon="''"/.test(tag)) continue;
        if (!/@left-click=/.test(tag)) dead.push(rel + ' :: ' + tag.replace(/\s+/g, ' '));
      }
    }
    expect(dead).toEqual([]);
  });
});

/**
 * 二级页标题形态（#2480，ADR-0067 决定 5）
 *
 * 二级页标题在返回箭头后左对齐，不居中；标题铺满标题块内容盒，长标题省略而不是撑出块
 * 压到右槽动作。右槽与胶囊让位只影响标题可用宽度，不改变标题起始位置。
 */
describe('二级页标题形态（#2480）', () => {
  const navBar = h.read(SRC_REL);

  it('守约：非居中标题块左对齐 + 标题定宽省略，零违规', () => {
    expect(secondaryTitleViolations(navBar)).toEqual([]);
  });

  const BASE = [
    '.nav-bar-title-block { flex: 1; min-width: 0; flex-direction: column; align-items: flex-start; margin-left: 32rpx; height: 88rpx; justify-content: center; }',
    '.nav-bar-title { width: 100%; margin-left: 0; lines: 1; text-overflow: ellipsis; }',
  ].join('\n');

  it('基准样本零违规（检测器自身不空跑）', () => {
    expect(secondaryTitleViolations(BASE)).toEqual([]);
  });

  it('标题块改回居中（align-items:center）→ 报「缺 align-items:flex-start」', () => {
    const v = secondaryTitleViolations(BASE.replace('align-items: flex-start', 'align-items: center'));
    expect(v.some((x) => x.includes('缺 align-items:flex-start'))).toBe(true);
  });

  it('标题文字加 text-align:center → 报「不得居中」', () => {
    const v = secondaryTitleViolations(BASE.replace('width: 100%;', 'width: 100%; text-align: center;'));
    expect(v.some((x) => x.includes('不得居中'))).toBe(true);
  });

  it('标题去掉 width:100%（ellipsis 失效）→ 报「缺 width:100%」', () => {
    const v = secondaryTitleViolations(BASE.replace('width: 100%; margin-left: 0;', 'margin-left: 0;'));
    expect(v.some((x) => x.includes('缺 width:100%'))).toBe(true);
  });
});
