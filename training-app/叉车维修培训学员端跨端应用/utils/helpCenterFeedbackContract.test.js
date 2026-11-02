/**
 * 帮助中心反馈入口（#1950）· 契约测试
 *
 * 票面（#1950 / 原型对照第一波 · 草稿 24）：帮助中心新增「意见反馈」（文字 + 图 + 联系方式）与
 * 「我的反馈」历史；「在线客服」跳现有 AI 学助手（教育类目送审要求客服可达）；「我的」页死占位摘掉。
 *
 * 本文件钉住的契约：
 * 1) 三条服务入口都在，「在线客服」走 `switchTab` 到 **tabBar 已注册**的 AI 学助手（navigateTo 到 tabBar 页会失败）
 * 2) 表单件存在且消费 `api/feedback`：图片上限 / 正文字数 / 状态词都从 api 常量取，页面不写第二份字面量
 * 3) 联系方式**不做格式校验**（ADR-0089：手机 / 微信 / QQ / 一句时间都收）
 * 4) 历史三态齐备（加载 / 失败带重试 / 空），且不出现「已回复」这类本域不下发的文案
 * 5) 两处新增件的 uvue CSS 约束（class 选择器、无 gap/calc/var/transition、模板 class ↔ 样式一一对应）
 * 6) 「我的」页三处死占位（学练计划 / 我的订单 / 地址管理）已摘除
 */
const fs = require('fs');
const path = require('path');

const { readText } = require('./utsHarness');
const ROOT = path.join(__dirname, '..');
const read = (rel) => readText(path.join(ROOT, rel));
const exists = (rel) => fs.existsSync(path.join(ROOT, rel));

const PAGE_REL = 'pages/profile/help-center.uvue';
const DIALOG_REL = 'pages/profile/components/feedback-dialog.uvue';
const API_REL = 'api/feedback.uts';
const TYPES_REL = 'types/feedback.uts';
const PROFILE_REL = 'pages/profile/profile.uvue';

const page = read(PAGE_REL);
const dialog = read(DIALOG_REL);
const api = read(API_REL);
const types = read(TYPES_REL);
const profile = read(PROFILE_REL);

const tplOf = (src) => src.slice(src.indexOf('<template>'), src.lastIndexOf('</template>'));
const scriptOf = (src) => src.slice(src.indexOf('<script'), src.lastIndexOf('</script>'));
const styleOf = (src) => src.slice(src.indexOf('<style'), src.lastIndexOf('</style>'));

describe('三条服务入口', () => {
  it('意见反馈 / 我的反馈 / 在线客服 都在帮助中心页里', () => {
    const tpl = tplOf(page);
    for (const label of ['意见反馈', '我的反馈', '在线客服']) {
      expect(tpl).toContain(label);
    }
    for (const handler of ['onOpenFeedback', 'onToggleHistory', 'onOpenCustomerService']) {
      expect(tpl).toContain('@click="' + handler + '"');
      expect(scriptOf(page)).toContain('function ' + handler);
    }
  });

  it('在线客服跳到 AI 学助手，且走 switchTab（它是 tabBar 页，navigateTo 会失败）', () => {
    const script = scriptOf(page);
    expect(script).toMatch(/uni\.switchTab\(\{ url: AI_ASSISTANT_ROUTE \}\)/);
    expect(script).not.toMatch(/navigateTo\(\{ url: AI_ASSISTANT_ROUTE/);
    const conf = JSON.parse(read('pages.json'));
    const tabs = (conf.tabBar && conf.tabBar.list ? conf.tabBar.list : []).map((t) => t.pagePath);
    expect(tabs).toContain('pages/ai-assistant/ai-assistant');
    expect(script).toContain("const AI_ASSISTANT_ROUTE = '/pages/ai-assistant/ai-assistant'");
  });

  it('帮助中心仍是普通页（不是 tabBar 页），入口从「我的」宫格 navigateTo 进得来', () => {
    const conf = JSON.parse(read('pages.json'));
    const tabs = (conf.tabBar && conf.tabBar.list ? conf.tabBar.list : []).map((t) => t.pagePath);
    expect(tabs).not.toContain('pages/profile/help-center');
    expect(profile).toContain("path: '/pages/profile/help-center'");
  });
});

describe('反馈表单件', () => {
  it('组件存在、被页面引用，且已登记进 profile 模块声明面', () => {
    expect(exists(DIALOG_REL)).toBe(true);
    expect(tplOf(page)).toContain('<feedback-dialog');
    expect(read('utils/modules.js')).toContain("'pages/profile/components/feedback-dialog.uvue'");
  });

  it('表单消费 api/feedback，上限与状态词都取常量（不写第二份字面量）', () => {
    const script = scriptOf(dialog);
    expect(script).toMatch(/from '\.\.\/\.\.\/\.\.\/api\/feedback'/);
    expect(script).toContain('submitFeedbackApi');
    expect(script).toContain('const contentMax = FEEDBACK_CONTENT_MAX');
    expect(script).toContain('const imageMax = FEEDBACK_IMAGES_MAX');
    expect(script).not.toMatch(/maxlength="1000"/);
    expect(script).not.toMatch(/:max="3"/);
  });

  it('图片区复用笔记那件组件（同一套选图/上传/上限提示，不另建第二套流水线）', () => {
    expect(scriptOf(dialog)).toContain("from './note-image-strip.uvue'");
    expect(tplOf(dialog)).toContain('<note-image-strip');
    expect(dialog).not.toContain('uni.chooseImage');
    expect(dialog).not.toContain('uploadForumImageApi');
  });

  it('联系方式不做格式校验（后端也不校验，端上加了会把真实可用的联系方式拦掉）', () => {
    const script = scriptOf(dialog);
    expect(script).not.toMatch(/function\s+isValid(Phone|Mobile|Email|Contact)\s*\(/);
    expect(script).not.toMatch(/\^1\[3-9\]/);
    expect(script).not.toMatch(/test\(\s*contact/);
    expect(tplOf(dialog)).toContain('placeholder="联系方式（选填，手机 / 微信 / QQ 都行）"');
  });

  it('空正文与上传中都被端上先拦下（不吃 400、不提交半张图）', () => {
    const script = scriptOf(dialog);
    expect(script).toMatch(/text\.length == 0/);
    expect(script).toContain('uni.showToast');
    expect(script).toContain('uploadingImage.value');
    expect(script).toContain('submitting');
  });

  it('提交成功后按全量数组提交（图片缺省也传空数组，与笔记同口径）', () => {
    expect(scriptOf(dialog)).toMatch(/submitFeedbackApi\(text, images\.value\.slice\(\), contact\.value\.trim\(\)\)/);
    expect(api).toContain('export function submitFeedbackApi(content : string, images : string[] = [], contact : string = \'\')');
  });
});

describe('我的反馈历史', () => {
  it('列表走 api/feedback 的 listFeedbackApi，页面层零直发请求', () => {
    const script = scriptOf(page);
    expect(script).toContain('listFeedbackApi');
    expect(page).not.toMatch(/from '[^']*api\/request/);
    expect(page).not.toMatch(/uni\.request\s*\(/);
  });

  it('三态齐备：加载 / 失败（可重试）/ 空', () => {
    const tpl = tplOf(page);
    const script = scriptOf(page);
    expect(script).toContain('historyLoading.value = true');
    expect(script).toContain('historyFailed.value = true');
    expect(tpl).toContain('加载中...');
    expect(tpl).toContain('反馈列表加载失败，点此重试');
    expect(tpl).toContain('还没有提交过反馈');
  });

  it('重试不重入、展开才拉，提交成功后自动展开并重拉', () => {
    const script = scriptOf(page);
    expect(script).toMatch(/if \(historyLoading\.value\) return/);
    expect(script).toMatch(/function onToggleHistory\(\)[\s\S]*loadHistory\(\)/);
    expect(script).toMatch(/function onFeedbackSubmitted\(\)[\s\S]*historyVisible\.value = true[\s\S]*loadHistory\(\)/);
  });

  it('状态只有「待处理 / 已处理」两态，且不出现本域不下发的「已回复」文案', () => {
    const script = scriptOf(page);
    expect(script).toContain('return status == handledStatus');
    expect(script).toContain("'已处理'");
    expect(script).toContain("'待处理'");
    // 只判模板面：源文件注释里「不写已回复」的说明不是实现（否则守护自己判红）
    expect(tplOf(page)).not.toContain('已回复');
    expect(tplOf(dialog)).not.toContain('已回复');
  });

  it('历史条目回显正文 + 图片（只读）+ 联系方式', () => {
    const tpl = tplOf(page);
    expect(tpl).toContain('{{ item.content }}');
    expect(tpl).toContain(':urls="item.images"');
    expect(tpl).toContain('{{ item.contact }}');
  });
});

describe('数据面契约（ADR-0003 手动映射）', () => {
  it('接口路径与字面量：POST /feedback、GET /feedback，无自造客服接口', () => {
    expect(api).toContain("postMapped<FeedbackItem>('/feedback'");
    expect(api).toContain("getPaged<FeedbackItem>('/feedback'");
    expect(api).not.toMatch(/customer-service|customerService|\/cs\b/);
  });

  it('字段逐项手动映射，handled_at 为 null 时保持 null', () => {
    for (const field of ['id', 'content', 'contact', 'status', 'handled_at', 'created_at']) {
      expect(api).toContain(`obj['${field}']`);
    }
    expect(api).toContain('handled_at: handledRaw != null ? toStr(handledRaw) : null');
    expect(api).toContain('images: extractImages(obj)');
  });

  it('类型定义与契约同形并从 barrel 再导出', () => {
    for (const f of ['id : number', 'content : string', 'images : string[]', 'contact : string', 'status : string', 'handled_at : string | null', 'created_at : string']) {
      expect(types).toContain(f);
    }
    expect(read('types/index.uts')).toContain("export { type FeedbackItem, type FeedbackListResult } from './feedback'");
  });
});

describe('「我的」页死占位清零（Q20/Q21）', () => {
  it('学练计划 / 我的订单 / 地址管理 三处点不开的入口已摘除', () => {
    for (const gone of ['学练计划', '我的订单', '地址管理']) {
      expect(profile).not.toContain(gone);
    }
    for (const key of ["'study-plan'", "'orders'", "'address'"]) {
      expect(profile).not.toContain(key);
    }
  });

  it('摘除后宫格只剩已接线的入口（无 available: false 残留）', () => {
    expect(profile).not.toMatch(/available: false/);
  });
});

describe('uvue CSS 约束（两处新增件）', () => {
  const stripCssComments = (s) => s.replace(/\/\*[\s\S]*?\*\//g, '');

  for (const [name, src] of [['help-center', page], ['feedback-dialog', dialog]]) {
    describe(name, () => {
      const style = stripCssComments(styleOf(src));

      it('style 块声明 lang="scss"', () => {
        expect(src).toMatch(/<style lang="scss">/);
      });

      it('不使用 uvue 不支持的属性与单位', () => {
        expect(/(^|[;{\s])gap\s*:/.test(style)).toBe(false);
        expect(style).not.toContain('var(--');
        expect(style).not.toContain('calc(');
        expect(style).not.toContain('white-space');
        expect(style).not.toContain('max-height');
        expect(/transition|animation/.test(style)).toBe(false);
      });

      it('只使用 class 选择器', () => {
        expect(/:(hover|active|focus|first-child|before|after)/.test(style)).toBe(false);
        expect(/^\s*(view|text|image|scroll-view|button|input|textarea)\s*\{/m.test(style)).toBe(false);
        expect(/^\s*#[\w-]+\s*\{/m.test(style)).toBe(false);
        expect(/^[^.@}\s][\w-]*\s*\{/m.test(style)).toBe(false);
      });

      it('模板 class 与样式定义一一对应', () => {
        const tpl = tplOf(src);
        const script = scriptOf(src);
        const used = new Set();
        for (const m of tpl.matchAll(/(?<!:)class="([^"]+)"/g)) {
          m[1].split(/\s+/).filter(Boolean).forEach((c) => used.add(c));
        }
        for (const m of tpl.matchAll(/:class="([^"]+)"/g)) {
          const expr = m[1];
          for (const q of expr.matchAll(/'?([A-Za-z][A-Za-z0-9_-]*)'?\s*:/g)) used.add(q[1]);
          for (const q of expr.matchAll(/'([^']+)'/g)) {
            const after = expr.slice(q.index + q[0].length);
            const before = expr.slice(0, q.index).trimEnd();
            if (q[1].length > 0 && !after.trimStart().startsWith(':') && !/(==|!=|===|!==)$/.test(before)) used.add(q[1]);
          }
          for (const fn of expr.matchAll(/([A-Za-z_$][\w$]*)\s*\(/g)) {
            const fstart = script.indexOf('function ' + fn[1] + '(');
            if (fstart === -1) continue;
            const fend = script.indexOf('\n    }', fstart);
            const body = script.slice(fstart, fend === -1 ? undefined : fend);
            for (const q of body.matchAll(/'([^'\n]+)'/g)) if (q[1].length > 0) used.add(q[1]);
          }
        }
        const defined = new Set();
        for (const m of style.matchAll(/\.([A-Za-z][A-Za-z0-9_-]*)/g)) defined.add(m[1]);

        expect({ undefinedClasses: [...used].filter((c) => !defined.has(c)) }).toEqual({ undefinedClasses: [] });
        expect({ deadClasses: [...defined].filter((c) => !used.has(c)) }).toEqual({ deadClasses: [] });
      });
    });
  }
});
