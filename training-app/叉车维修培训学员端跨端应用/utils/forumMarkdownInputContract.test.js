/**
 * 论坛**输入区形态**契约（#1240 P3；ADR-0025 ⑥-5/⑥-6、⑦，根 ADR-0052 决策①/②/⑧）
 *
 * ## 这是**接线守护**，不是 ③ 门证据（`docs/agents/guards.md`）
 *
 * 它守的是「形态接线没有被悄悄拆掉」：模板里那几条 `v-if` 闸门、两组控件没被换成同款、
 * 图片的两个入口还在、两个页面共用同一个组件。这些都是**源码结构**，node 里跑不出来
 * （`.uvue` 的模板/样式不参与 UTS 行为缝）。
 *
 * **行为面由谁承重**（缺了它本文件会假绿，故逐条点名）：
 *   · 「按钮承诺的语法必须真能渲染」⇒ `utils/markdownToolbarBehavior.test.js`（真执行 round-trip，
 *     含「加一枚未声明按钮」「代码块承诺落空」两种坏实现的必红取证）；
 *   · 「格式档位选哪一支」⇒ `utils/forumBodyBehavior.test.js`（真执行 `forumContentBlocks` 产出物）；
 *   · 「作者的声明真的进/不进载荷」⇒ `utils/forumContentFormatBehavior.test.js`（真执行 `api/forum.uts`，
 *     创建/回复**必须带**、编辑**必须不带**两个反方向）。
 *
 * ## 判据来源（逐条对应票面验收标准，不自己加戏）
 *
 * ADR-0025 ⑥-5：编写/预览可切；工具栏可横滑；长按出中文提示；图片有拍照与相册两个入口；
 * **纯文本档不出现 tab / 工具栏 / 能力提示行**；能力提示行只讲边界、不讲能力清单。
 * 根 ADR-0052 决策②：**数据档位（会永久保存）与视图档位（临时）形态必须不同** ——
 * 同卡片并排两组同款胶囊，用户分不清哪组会永久保存。
 */
const path = require('path');

/** 读源码一律经共享读者归一 EOL（ADR-0019）：与检出平台无关，Windows CRLF 也免疫。 */
const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));

const INPUT_REL = 'pages/forum/components/forum-markdown-input.uvue';
const CREATE_REL = 'pages/forum/forum-create.uvue';
const DETAIL_REL = 'pages/forum/forum-detail.uvue';
const COMPOSER_REL = 'composables/useReplyComposer.uts';
const MY_FORUM_REL = 'pages/forum/my-forum.uvue';

const input = read(INPUT_REL);
const createPage = read(CREATE_REL);
const detailPage = read(DETAIL_REL);
const composer = read(COMPOSER_REL);

// ===== ① 形态只有一份：发帖与回复共用同一个组件 =====

describe('形态只有一份：发帖表单与回复栏共用同一个输入区组件（根 ADR-0052 决策①）', () => {
  it('两个宿主都显式 import 同一个组件（各自 import 自己那份就是形态分家了）', () => {
    for (const [rel, src] of [[CREATE_REL, createPage], [DETAIL_REL, detailPage]]) {
      expect([rel, src.includes("from './components/forum-markdown-input.uvue'")]).toEqual([rel, true]);
    }
    // 形态的差异只允许由 `variant` prop 表达（两处不是两份实现）
    expect(createPage).toContain('variant="topic"');
    expect(detailPage).toContain('variant="reply"');
  });

  it('旧的重复形态已退场：页面里不再自持 textarea / 回复行输入（否则会与组件里的那份漂移）', () => {
    expect(createPage).not.toContain('class="form-textarea"');
    expect(detailPage).not.toContain('class="reply-input"');
    // 也删掉了配套的死 CSS（ui-conventions：删 hint / 迁形态时同步删 class）
    expect(createPage).not.toContain('.form-textarea {');
    expect(detailPage).not.toContain('.reply-bar-row {');
  });

  it('工具栏（横滑行 / 长按提示）只在这一个文件里实现', () => {
    expect(input).toContain('@longpress="onToolbarLongPress(idx)"');
    expect(input).toContain('@click="onToolbarTap(idx)"');
    for (const [rel, src] of [[CREATE_REL, createPage], [DETAIL_REL, detailPage]]) {
      expect([rel, src.includes('@longpress')]).toEqual([rel, false]);
    }
  });
});

// ===== ② 纯文本档不出现 tab / 工具栏 / 能力提示行（⑥-5）=====

describe('纯文本档不出现 tab / 工具栏 / 边界提示行（ADR-0025 ⑥-5，照根 ADR-0052 决策⑧）', () => {
  it('视图档位（下划线 tab）由 `isMarkdown` 闸门把守', () => {
    expect(input).toContain('<view v-if="isMarkdown" class="view-tabs">');
    expect(input).toContain('<text class="view-tab-text"');
    // 「编写 | 预览」两档都在
    expect(input).toContain("onSelectView('edit')");
    expect(input).toContain("onSelectView('preview')");
  });

  it('工具栏由 `isMarkdown` 闸门把守，且是**可横滑**的一行', () => {
    expect(input).toContain('<scroll-view v-if="isMarkdown" class="toolbar-scroll" scroll-x="true">');
    // 溢出靠 flex + 子项 flex-shrink: 0 撑出（不写 white-space —— 它只在 <text> / <button> 上合法，
    // 全仓口径由 `utils/uvueWhiteSpaceContract.test.js` 执法，本文件不重复一遍）
    expect(input).toContain('flex-shrink: 0');
  });

  it('能力提示行由 `isMarkdown` 闸门把守，且**只讲边界**（不提能力清单）', () => {
    expect(input).toContain('<text v-if="isMarkdown" class="boundary-notice">{{ boundaryNotice }}</text>');
    // 判据是「闸门表达式里没有取反」：纯文本档不许出现这三样
    const guarded = input.match(/v-if="isMarkdown[^"]*"/g) || [];
    expect(guarded.length).toBeGreaterThanOrEqual(3);
    expect(input).not.toMatch(/v-if="!isMarkdown/);
    // 文案单点在 utils/forumDisplay，组件不另抄一份中文
    expect(input).toContain('FORUM_MARKDOWN_BOUNDARY_NOTICE');
    expect(input).not.toContain('表格与图表显示原文');
  });
});

// ===== ③ 两组档位形态必须不同（根 ADR-0052 决策②）=====

describe('数据档位与视图档位**形态不同**：前者胶囊（永久保存）、后者下划线（临时）', () => {
  it('数据档位 = 胶囊分段控件（圆角胶囊）', () => {
    expect(input).toContain('class="format-row"');
    expect(input).toContain('class="format-chip"');
    expect(input).toMatch(/\.format-chip \{[\s\S]*?border-radius: 28rpx/);
    // 两个取值都在（纯文本 / Markdown 是作者的声明位）
    expect(input).toContain('{ label: \'纯文本\', value: FORMAT_TEXT }');
    expect(input).toContain("{ label: 'Markdown', value: FORMAT_MARKDOWN }");
  });

  it('视图档位 = 下划线 tab（靠 border-bottom 表达选中，不是第二个胶囊）', () => {
    expect(input).toMatch(/\.view-tab\.active \{[\s\S]*?border-bottom: 4rpx solid #2979ff/);
    // 两组不共用样式类 —— 共用就是「用户分不清哪组会永久保存」
    const formatClasses = ['format-row', 'format-chip', 'format-chip-text'];
    for (const cls of formatClasses) {
      expect([cls, input.includes('view-tab ' + cls)]).toEqual([cls, false]);
    }
  });
});

// ===== ④ 图片：拍照 / 相册双入口（ADR-0025 ⑦）=====

describe('图片走拍照 / 相册**两个直达入口**（ADR-0025 ⑦ 对 Web 点击/粘贴/拖拽的替换）', () => {
  it('组件渲染两个入口，各自把来源直接交给宿主的处理器', () => {
    expect(input).toContain("@click=\"onAddImage('camera')\"");
    expect(input).toContain("@click=\"onAddImage('album')\"");
    expect(input).toContain("emit('add-image', source)");
    // 两个入口的文案不同（否则用户分不清哪个是哪个）
    expect(input).toContain("'拍照'");
    expect(input).toContain("'相册'");
  });

  it('宿主按来源**直达**：`sourceType` 只含该来源（旧的「两个来源共用系统选择器」已退场）', () => {
    expect(createPage).toContain('sourceType: [source]');
    expect(composer).toContain('sourceType: [source]');
    for (const [rel, src] of [[CREATE_REL, createPage], [COMPOSER_REL, composer]]) {
      expect([rel, src.includes("sourceType: ['album', 'camera']")]).toEqual([rel, false]);
    }
  });

  it('图片限额仍按面分档：发帖 ≤9、回复 ≤3', () => {
    expect(createPage).toContain(':max-images="9"');
    expect(detailPage).toContain(':max-images="3"');
  });
});

// ===== ⑤ 编辑态：不出现档位控件，按帖子原格式呈现（Q5 裁定）=====

describe('编辑态不给档位控件，只按帖子**原格式**决定形态（编辑不变式的形态侧）', () => {
  it('控件闸门 = 非编辑 且 非资源态（两者都存不下格式声明）', () => {
    expect(createPage).toContain(':show-format-switch="showFormatSwitch"');
    // 闸门表达式本身就是判据：编辑态与资源态各由一个合取项挡掉
    expect(createPage).toContain('=> !isEdit.value && !isResourceMode.value)');
  });

  it('编辑态回填读详情 DTO 的 `content_format`（选择形态的依据），且**不写回载荷**', () => {
    expect(createPage).toContain('editFormat.value = data.content_format');
    // 生效档位：资源态恒纯文本；编辑态用帖子原格式；新建态用作者记忆位
    expect(createPage).toMatch(/if \(isResourceMode\.value\) return FORMAT_TEXT/);
    expect(createPage).toMatch(/return isEdit\.value \? editFormat\.value : authorFormat\.value/);
    // 「不写回载荷」的行为面由 forumContentFormatBehavior 真执行承重（PUT 键集合不含 content_format）
    expect(createPage).not.toMatch(/updateForumTopicApi\([^)]*content_format/);
  });

  it('新建态用作者记忆位（根 ADR-0044「记住上次选择」），切换即写回记忆', () => {
    expect(createPage).toContain('const authorFormat = ref<string>(readAuthorFormat())');
    expect(createPage).toContain('writeAuthorFormat(value)');
    expect(composer).toContain('const replyFormat = ref<string>(readAuthorFormat())');
    expect(composer).toContain('writeAuthorFormat(value)');
  });
});

// ===== ⑥ 界面层不直引档位常量 / 解析器（P2 立的口径延伸到新组件）=====

describe('界面层只经格式轴：不直引档位常量、不直连解析器', () => {
  it('输入区组件不 import `utils/markdown`，也不出现档位常量或解析器名', () => {
    expect(input).not.toContain("from '../../../utils/markdown'");
    expect(input).not.toContain('SUBSET_');
    expect(input).not.toContain('parseMarkdown');
  });

  it('它接的是三条单点：按钮集 / 格式轴 / 块组件（预览与发布同源）', () => {
    expect(input).toContain('forumToolbarMembers()');
    expect(input).toContain('forumContentBlocks(props.content, props.format)');
    expect(input).toContain('isMarkdownFormat(props.format)');
    expect(input).toContain("from './forum-markdown-blocks.uvue'");
  });
});

// ===== ⑦ P2 遗留的真实缺口收口：「我的动态」的回复原文 =====

describe('「我的动态」的回复原文接上格式轴（P2 复核回写修正①的真实缺口，本批收口）', () => {
  const myForum = read(MY_FORUM_REL);

  it('完整 `content` 的承载点不再无条件直出源串', () => {
    expect(myForum).toContain('function isMarkdownReply(item : ForumReply) : boolean');
    expect(myForum).toContain('function replyBlocks(item : ForumReply) : MarkdownBlock[]');
    expect(myForum).toContain('v-if="!isMarkdownReply(item)" class="reply-content"');
    expect(myForum).toContain('<ForumMarkdownBlocks :blocks="replyBlocks(item)" />');
  });

  it('它消费的是 DTO 上已有的 `content_format`（P1 的 `buildReply` 已带上，本批不动 DTO）', () => {
    expect(myForum).toContain('isMarkdownFormat(item.content_format)');
    expect(myForum).toContain('forumContentBlocks(item.content, item.content_format)');
  });
});
