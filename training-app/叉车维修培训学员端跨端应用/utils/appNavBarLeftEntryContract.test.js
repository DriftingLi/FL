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

/** 抹注释（块 + 行，防注释文本里的字段名被误判成真接线）。先例 dashboardContract。 */
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
 * 页头标题恒居中（#2761 改判 #2480 / ADR-0067 决定 5 / ADR-0069 决定 1）
 *
 * 沿革：
 * - 2026-10-27：AI 助手左槽补 ☰ 后加 `centerTitle` prop 做三段式居中；
 * - #2402：`--center` 流内居中实为「剩余空间居中」，右槽胶囊让位把标题推左；
 * - #2442：标题块回流入内、`flex:1`，居中在「左槽右缘 → 右槽左缘」的可用空间里；
 * - #2479：一级页（无返回箭头）改**真屏幕中心**，二级页整枝不动；
 * - #2480：二级页收口为「返回箭头后左对齐」；
 * - **#2761（本契约）**：取消一级/二级二分 —— 用户裁定「形态不统一、不符合行业习惯」，
 *   全站页头统一为「返回箭头最左 + 标题屏幕居中 + 右侧胶囊」。
 *
 * 三条几何（缺一条就不是屏幕中心）：
 * 1. 标题块流内 `flex:1` + 两端**等宽**留白 ⇒ 文字中心恒 375rpx；
 * 2. 两侧槽 `position:absolute` 出流 ⇒ 侧槽内容与胶囊让位都不参与标题块可用宽度；
 * 3. 留白由 `capsuleLeft` 现算（右缘 = 胶囊左缘 − CAPSULE_CLEARANCE_RPX），不写死数值。
 * ══════════════════════════════════════════════════════════════════════════════ */

/** 标题恒居中违规清单：真实文件与注入样本共用同一检测路径。 */
function titleCenterViolations(srcRaw) {
  const src = stripComments(srcRaw);
  const v = [];
  // 判据只剩「有标题」—— 左槽有无字形不再影响标题位置
  const trigger = (src.match(/const titleCentered\s*=\s*computed\([^\n]*\n/) || [''])[0];
  if (!trigger) {
    v.push('缺 titleCentered 判定（页头居中的唯一开关）');
  } else {
    if (!/props\.title\.length\s*>\s*0/.test(trigger)) {
      v.push('titleCentered 丢非空标题守卫（空标题也居中 ⇒ 标题块白占可用空间）');
    }
    if (/leftOverlap/.test(trigger)) {
      v.push('titleCentered 仍按 leftOverlap 二分（#2761 已取消一级/二级形态差异）');
    }
    if (/props\.centerTitle/.test(trigger)) {
      v.push('titleCentered 仍读 centerTitle（#2761 起恒居中，该 prop 已删）');
    }
  }
  // 死件防复活
  if (/centerTitle\s*\?:\s*boolean/.test(src)) {
    v.push('centerTitle prop 复活（#2761 起标题恒居中，prop 已删）');
  }
  if (/leftOverlap/.test(src)) {
    v.push('leftOverlap 复活（#2761 起标题位置与左槽内容无关）');
  }
  if (/nav-bar-title-block--floating-center/.test(src)) {
    v.push('floating-center 复活（脱流定宽 574rpx 会越过胶囊左边线，见 #2442）');
  }
  if (/nav-bar-side--front/.test(src)) {
    v.push('side--front 复活（标题块已回流入流，不再需要 z-index 抬层）');
  }
  // 标题块必须在流内、flex:1 —— 这是「居中于屏幕中心」的几何本体
  const block = (src.match(/\.nav-bar-title-block\s*\{[^}]*\}/) || [''])[0];
  if (!/flex\s*:\s*1/.test(block)) {
    v.push('.nav-bar-title-block 缺 flex:1（不吃整行 ⇒ 居中点退回剩余空间中心）');
  }
  if (/position\s*:\s*absolute/.test(block)) {
    v.push('.nav-bar-title-block 出现 position:absolute（脱流即失去整行约束，见 #2442）');
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
  // 标题带：右缘 = 胶囊左缘 − 间隙，由 capsuleLeft 现算，禁写死数值
  const band = (src.match(/const titleBlockStyle\s*=\s*computed[\s\S]*?\}\)\n/) || [''])[0];
  if (!band) {
    v.push('缺 titleBlockStyle（标题带留白没有计算面）');
  } else {
    if (!/safeArea\.value\.capsuleLeft/.test(band)) {
      v.push('titleBlockStyle 未读 capsuleLeft（写死留白会在胶囊更靠左的机型上让标题钻进胶囊）');
    }
    if (!/CAPSULE_CLEARANCE_RPX/.test(band)) {
      v.push('titleBlockStyle 未用 CAPSULE_CLEARANCE_RPX（右缘与胶囊间隙脱钩）');
    }
    if (!/margin-left/.test(band) || !/margin-right/.test(band)) {
      v.push('titleBlockStyle 缺两端留白（单边留白 ⇒ 文字中心偏离 375rpx）');
    }
  }
  if (/\b532\b/.test(src)) {
    v.push('出现写死 532（胶囊左边线是机型相关值，必须由 capsuleLeft 现算）');
  }
  // 空标题守卫：标题块必须 v-if，否则 flex:1 会长出透明点击区
  const blockTag = (src.match(/<view\s+v-if="title\.length > 0 \|\| subtitle\.length > 0"\s+class="nav-bar-title-block"/) || [''])[0];
  if (blockTag.length === 0) {
    v.push('标题块缺 v-if 非空守卫（空标题下 flex:1 会长出透明点击区）');
  }
  // 两条配套几何：侧槽脱流 + 包含块
  const body = (src.match(/\.nav-bar-body\s*\{[^}]*\}/) || [''])[0];
  if (!/position\s*:\s*relative/.test(body)) {
    v.push('.nav-bar-body 缺 position:relative（侧槽绝对定位的包含块）');
  }
  const floatSides = [
    ['nav-bar-side--floating-left', 'left'],
    ['nav-bar-side--floating-right', 'right'],
  ];
  for (const pair of floatSides) {
    const blk = (src.match(new RegExp('\\.' + pair[0] + '\\s*\\{[^}]*\\}')) || [''])[0];
    if (blk.length === 0) {
      v.push('缺 .' + pair[0] + '（侧槽未脱流 ⇒ 侧槽内容仍把标题中心挤偏）');
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
      v.push(pair[0] + ' 未以 titleCentered 为门');
    }
    if (!new RegExp(pair[1]).test(c)) {
      v.push(pair[0] + ' 未返回 ' + pair[1]);
    }
  }
  return v;
}

describe('页头标题恒居中（#2761）', () => {
  const src = h.read(SRC_REL);

  it('守约：有标题即居中 + 两端等宽留白由胶囊现算，零违规', () => {
    expect(titleCenterViolations(src)).toEqual([]);
  });

  it('标题块把留白绑到 titleBlockStyle（内联样式优先于类，类里不再写死留白）', () => {
    expect(src).toMatch(/class="nav-bar-title-block"[^>]*:style="titleCentered \? titleBlockStyle : ''"/);
  });

  it('默认返回改为 34rpx 切片图，不再依赖文字字号与基线补偿', () => {
    const back = (src.match(/\.nav-bar-back-img\s*\{[^}]*\}/) || [''])[0];
    expect(back).toMatch(/width\s*:\s*34rpx/);
    expect(back).toMatch(/height\s*:\s*34rpx/);
    expect(back).not.toMatch(/font-size|margin-bottom/);
    expect(src).not.toContain('nav-bar-icon--back');
  });
});

describe('标题恒居中检测器判别力（注入违规样本必红）', () => {
  const BASE = [
    '<view v-if="title.length > 0 || subtitle.length > 0" class="nav-bar-title-block" :class="titleBlockClass" :style="titleCentered ? titleBlockStyle : \'\'"></view>',
    'const titleCentered = computed((): boolean => props.title.length > 0)',
    'const titleBlockStyle = computed((): string => {',
    '    const capsuleLeft = safeArea.value.capsuleLeft',
    '    const bandMargin = (750 - NAV_BODY_PADDING_X) - (capsuleLeft - CAPSULE_CLEARANCE_RPX)',
    "    return 'margin-left: ' + bandMargin + 'rpx; margin-right: ' + bandMargin + 'rpx;'",
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
    '.nav-bar-title-block--centered { align-items: center; }',
    '.nav-bar-title--centered { width: 100%; text-align: center; margin-left: 0; }',
    '</style>',
  ].join('\n');

  it('基准样本零违规（检测器自身不空跑）', () => {
    expect(titleCenterViolations(BASE)).toEqual([]);
  });

  it('去掉非空标题守卫（空标题也居中）→ 报「非空标题守卫」', () => {
    const v = titleCenterViolations(BASE.replace('props.title.length > 0', 'true'));
    expect(v.some((x) => x.includes('非空标题守卫'))).toBe(true);
  });

  it('titleCentered 加回 leftOverlap 二分 → 报「仍按 leftOverlap 二分」', () => {
    const v = titleCenterViolations(
      BASE.replace('props.title.length > 0', 'props.title.length > 0 && !leftOverlap.value')
    );
    expect(v.some((x) => x.includes('leftOverlap 二分'))).toBe(true);
  });

  it('centerTitle prop 复活 → 报「centerTitle prop 复活」', () => {
    const v = titleCenterViolations(BASE.replace('const titleCentered', 'centerTitle ?: boolean\nconst titleCentered'));
    expect(v.some((x) => x.includes('centerTitle prop 复活'))).toBe(true);
  });

  it('标题带不读 capsuleLeft（写死留白）→ 报「未读 capsuleLeft」', () => {
    const v = titleCenterViolations(BASE.replace('const capsuleLeft = safeArea.value.capsuleLeft', 'const capsuleLeft = 556'));
    expect(v.some((x) => x.includes('未读 capsuleLeft'))).toBe(true);
  });

  it('标题带不用 CAPSULE_CLEARANCE_RPX → 报「未用 CAPSULE_CLEARANCE_RPX」', () => {
    const v = titleCenterViolations(BASE.replace('(capsuleLeft - CAPSULE_CLEARANCE_RPX)', '(capsuleLeft - 24)'));
    expect(v.some((x) => x.includes('未用 CAPSULE_CLEARANCE_RPX'))).toBe(true);
  });

  it('写死 532 → 报「写死 532」', () => {
    const v = titleCenterViolations(BASE.replace('const capsuleLeft = safeArea.value.capsuleLeft', 'const capsuleLeft = 532'));
    expect(v.some((x) => x.includes('写死 532'))).toBe(true);
  });

  it('居中块去掉 align-items:center → 报「缺 align-items:center」', () => {
    const v = titleCenterViolations(BASE.replace('.nav-bar-title-block--centered { align-items: center; }', '.nav-bar-title-block--centered { }'));
    expect(v.some((x) => x.includes('缺 align-items:center'))).toBe(true);
  });

  it('居中文字去掉 text-align:center → 报「缺 text-align:center」', () => {
    const v = titleCenterViolations(
      BASE.replace('.nav-bar-title--centered { width: 100%; text-align: center; margin-left: 0; }', '.nav-bar-title--centered { width: 100%; margin-left: 0; }')
    );
    expect(v.some((x) => x.includes('缺 text-align:center'))).toBe(true);
  });

  it('标题块去掉 flex:1 → 报「缺 flex:1」', () => {
    const v = titleCenterViolations(BASE.replace('.nav-bar-title-block { flex: 1; min-width: 0; }', '.nav-bar-title-block { min-width: 0; }'));
    expect(v.some((x) => x.includes('缺 flex:1'))).toBe(true);
  });

  it('标题块改回 position:absolute → 报「position:absolute」', () => {
    const v = titleCenterViolations(
      BASE.replace('.nav-bar-title-block { flex: 1; min-width: 0; }', '.nav-bar-title-block { flex: 1; position: absolute; }')
    );
    expect(v.some((x) => x.includes('position:absolute'))).toBe(true);
  });

  it('标题块去掉 v-if 非空守卫 → 报「缺 v-if 非空守卫」', () => {
    const v = titleCenterViolations(
      BASE.replace('<view v-if="title.length > 0 || subtitle.length > 0" class="nav-bar-title-block"', '<view class="nav-bar-title-block"')
    );
    expect(v.some((x) => x.includes('缺 v-if 非空守卫'))).toBe(true);
  });

  it('侧槽不脱流（摘掉 --floating-left）→ 报「缺 .nav-bar-side--floating-left」', () => {
    const v = titleCenterViolations(BASE.replace('.nav-bar-side--floating-left { position: absolute; left: 24rpx; }', ''));
    expect(v.some((x) => x.includes('缺 .nav-bar-side--floating-left'))).toBe(true);
  });

  it('侧槽脱流类丢 titleCentered 门 → 报「未以 titleCentered 为门」', () => {
    const v = titleCenterViolations(
      BASE.replace("    return titleCentered.value ? 'nav-bar-side--floating-right' : ''", "    return 'nav-bar-side--floating-right'")
    );
    expect(v.some((x) => x.includes('未以 titleCentered 为门'))).toBe(true);
  });

  it('.nav-bar-body 丢 position:relative（侧槽失去包含块）→ 报「缺 position:relative」', () => {
    const v = titleCenterViolations(BASE.replace('.nav-bar-body { position: relative; }', '.nav-bar-body { }'));
    expect(v.some((x) => x.includes('缺 position:relative'))).toBe(true);
  });
});

/**
 * 页头形态（#2442 建立；#2761 收窄）
 *
 * 一级/二级的**标题形态**差异已取消（见上）。仍保留的两条：
 * - 页级动作只能放左槽（`#left`），右槽必须空 —— 侧槽脱流后右槽内容会盖在标题上；
 * - 二级页（默认返回箭头）必须接 `@left-click`，不留死按钮。
 */
describe('页头形态（#2442 / #2761）', () => {
  const navBar = h.read(SRC_REL);

  it('页头件开放 #left 插槽，且左槽点击按 leftIcon 收口（插槽内容自带 @click 时不重复上抛）', () => {
    expect(navBar).toMatch(/<slot name="left">/);
    expect(navBar).toMatch(/function onLeftClick\(\)[\s\S]{0,200}?props\.leftIcon\.length > 0/);
  });

  it('一级页面（left-icon=""）的单个自定义操作放左槽，右槽不再承载', () => {
    const forum = h.read('pages/forum/forum.uvue');
    expect(forum).toMatch(/<template #left>[\s\S]*?msg-icon-wrap/);
    expect(forum).not.toMatch(/<template #right>/);
    // dashboard 左槽只承载品牌展示；证件、搜索、商城仍统一下移到独立业务栏。
    const dash = h.read('pages/dashboard/dashboard.uvue');
    expect(dash).toMatch(/<template #left>[\s\S]*?dashboard-brand[\s\S]*?<\/template>/);
    expect(dash).not.toMatch(/nav-bell/);
    expect(dash).not.toMatch(/:right-icons=/);
    expect(dash).toMatch(/class="credential-bar"/);
    // #2907：证件栏改整页入口（onCertTap 随浮层开关一起退役），仍须可点
    expect(dash).toMatch(/class="credential-picker"[^>]*@click="openCertPage"/);
  });

  it('全页右槽必须空（#2761：侧槽脱流后右槽内容会盖在居中标题上）', () => {
    const bad = [];
    for (const rel of h.sourceFilesIn('pages')) {
      const src = h.read(rel);
      for (const m of src.matchAll(/<(?:app-nav-bar|AppNavBar)\b[^>]*>/g)) {
        const tag = m[0];
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
