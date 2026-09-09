/**
 * 资源上传模块契约测试（refs #710 → #754/#756 → #760）
 *
 * 架构演进：#710 接通投稿链路（先传后交 + mine）；#754/#756 上传页原型对齐与统一发布；
 * #760 用户裁定「复用发布新帖页」——forum-create 分类行四 tab（广场/资源/知识问答/备考经验），
 * 「资源」tab 从假 chip（映射 discussion）变真投稿模式，upload-resource 独立页退役删除。
 * 后端投稿域（contributions，#517/#611/#702）：POST /contributions/upload-file 暂存 →
 * POST /contributions 建稿；GET /contributions/mine 我的投稿。发帖接口永不收到 resource 类别。
 *
 * 缝：api/*.uts 与页面 .uvue 无法在 jest 中 import，沿用源码契约测试缝
 * （先例 utils/checkinWiringContract.test.js）。
 *
 * 钉住的契约：
 * 1) 提交链路：forum-create 资源模式消费 api/contribution（上传+建稿+证件门槛+白名单前置校验）
 * 2) 数据源：my-uploads→getMyContributionsApi；my-purchases→getStudentCoursesApi
 * 3) 入口改跳：资源面板格子/forum 资源 tab/my-uploads 去上传 → forum-create?scope=resource；
 *    已删页 upload-resource 全域零引用
 * 4) 路由与类型：api/contribution.uts 三路由、字段对齐后端 json tag（守护规则 H）
 * 5) 归一：forum-create 默认 discussion；URL scope=resource 进资源 tab，其余非法/历史归一；
 *    编辑回填 normalizeCategory（帖子永不回填 resource）
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

const stripComments = (src) =>
  src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|\s)\/\/[^\n]*/g, '$1');

describe('提交链路契约（#760：forum-contribution-form 组件承载投稿）', () => {
  const src = read('pages/forum/components/forum-contribution-form.uvue');
  const shell = read('pages/forum/forum-create.uvue');
  it('上传与创建投稿函数 import 自 api/contribution（组件层）', () => {
    const importRe =
      /import\s*\{[^}]*\buploadContributionFileApi\b[^}]*\}\s*from\s*'\.\.\/\.\.\/\.\.\/api\/contribution'/;
    expect(importRe.test(src)).toBe(true);
  });
  it('选文件走 chooseFile/chooseMessageFile，投稿白名单无图片', () => {
    expect(src).toContain('uni.chooseFile');
    expect(src).toContain('uni.chooseMessageFile');
    const docExtLine = src.match(/const docExt\s*=\s*\[([^\]]*)\]/)[1];
    expect(docExtLine).not.toContain('jpg');
    expect(docExtLine).not.toContain('png');
    expect(docExtLine).not.toContain('jpeg');
  });
  it('客户端前置校验齐备（白名单/20MB/50MB/5 个文件）', () => {
    expect(src).toContain("'pdf'");
    expect(src).toContain('20 * 1024 * 1024');
    expect(src).toContain('50 * 1024 * 1024');
    expect(src).toMatch(/maxFiles\s*=\s*5/);
  });
  it('证件门槛：壳层取证件下发，组件无证件阻断投稿', () => {
    expect(shell).toContain('getCurrentCredentialApi');
    expect(src).toMatch(/props\.credentialId\s*<=\s*0/);
  });
  it('壳层接线：组件 import + 资源模式渲染并下发 title/intro/credential', () => {
    expect(shell).toMatch(/import ForumContributionForm from '\.\/components\/forum-contribution-form\.uvue'/);
    expect(shell).toMatch(/<ForumContributionForm v-if="isResourceMode"[\s\S]*?:credential-id="credentialId"/);
  });
});

describe('my-uploads 接真数据契约', () => {
  const src = read('pages/resources/my-uploads.uvue');
  it('消费 getMyContributionsApi（import 自 api/contribution）', () => {
    expect(src).toMatch(
      /import\s*\{[^}]*\bgetMyContributionsApi\b[^}]*\}\s*from\s*'\.\.\/\.\.\/api\/contribution'/
    );
  });
  it('onShow 刷新（投稿后返回可见最新）', () => {
    expect(src).toMatch(/onShow\s*\(/);
  });
  it('状态徽标覆盖后端五态值域', () => {
    for (const s of ['approved', 'rejected', 'withdrawn', 'archived', '待审核']) {
      expect(src).toContain(s);
    }
  });
});

describe('my-purchases 数据源修正契约', () => {
  const src = read('pages/resources/my-purchases.uvue');
  it('消费 getStudentCoursesApi（ADR-0017 我的课程）', () => {
    expect(src).toMatch(
      /import\s*\{[^}]*\bgetStudentCoursesApi\b[^}]*\}\s*from\s*'\.\.\/\.\.\/api\/student'/
    );
  });
  it('代码体不再引用 profile/course_progress 假数据源', () => {
    const code = stripComments(src);
    expect(code).not.toContain('getProfileApi');
    expect(code).not.toContain('course_progress');
  });
});

describe('forum 资源入口改跳契约（#760：统一进 forum-create 资源 tab）', () => {
  it('资源面板上传格子跳 forum-create?scope=resource', () => {
    const src = read('pages/forum/components/forum-resource-panel.uvue');
    expect(src).toContain("'/pages/forum/forum-create?scope=resource'");
  });
  it('forum onCreate 资源 tab 分支跳 forum-create?scope=resource', () => {
    const src = read('pages/forum/forum.uvue');
    expect(src).toContain("'/pages/forum/forum-create?scope=resource'");
  });
  it('my-uploads 去上传跳 forum-create?scope=resource', () => {
    const src = read('pages/resources/my-uploads.uvue');
    expect(src).toContain("'/pages/forum/forum-create?scope=resource'");
  });
  it('已删页 upload-resource 全域零引用（源码）', () => {
    const offenders = [];
    const walk = (dir) => {
      for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) { walk(p); continue; }
        if (!/\.(uts|uvue)$/.test(e.name) || /\.test\./.test(e.name)) continue;
        if (fs.readFileSync(p, 'utf8').includes('/pages/resources/upload-resource')) {
          offenders.push(path.relative(ROOT, p));
        }
      }
    };
    for (const d of ['api', 'pages', 'composables', 'utils', 'stores']) walk(path.join(ROOT, d));
    expect(offenders).toEqual([]);
  });
  it('forum-create 资源 tab 绝不把 resource 发进发帖接口（按钮隐藏 + onSubmit 早退双守卫）', () => {
    const src = read('pages/forum/forum-create.uvue');
    expect(src).toMatch(/if\s*\(isResourceMode\.value\)\s*return/);
    expect(src).toMatch(/<view v-if="!isResourceMode" class="submit-section">/);
  });
});

describe('api/contribution.uts 路由与类型契约', () => {
  const src = read('api/contribution.uts');
  it('三条路由与后端注册一致（contribution.go）', () => {
    expect(src).toContain("'/contributions/upload-file'");
    expect(src).toContain("'/contributions'");
    expect(src).toContain("'/contributions/mine'");
  });
  it('类型字段对齐后端 ContributionItemDTO/ContributionFileDTO json tag', () => {
    for (const f of ['id', 'credential_id', 'title', 'intro', 'status', 'is_anonymous',
      'downloads_count', 'reject_reason', 'created_at',
      'file_id', 'file_name', 'file_url', 'file_size', 'content_type']) {
      expect(src).toMatch(new RegExp('\\b' + f + '\\s*:'));
    }
  });
  it('catch 参数零 : any 注解（守护规则 H）', () => {
    expect(src).not.toMatch(/catch\s*\(\s*e\s*:\s*any/);
  });
});

describe('forum-create 归一与资源模式契约（#710 + #760）', () => {
  const src = read('pages/forum/forum-create.uvue');
  it('默认类别为 discussion（非 general/resource）', () => {
    expect(src).toMatch(/selectedCategory\s*=\s*ref<string>\('discussion'\)/);
    expect(src).not.toMatch(/ref<string>\('general'\)/);
  });
  it('分类行含资源 tab（value=resource，非假映射 discussion）', () => {
    expect(src).toMatch(/label:\s*'资源',\s*value:\s*'resource'/);
  });
  it('URL scope=resource 进资源模式；其余非法/历史归一', () => {
    expect(src).toMatch(/s == 'resource' \? 'resource' : normalizeCategory\(s\)/);
  });
  it('编辑回填过 normalizeCategory（帖子永不回填 resource）', () => {
    expect(src).toMatch(/selectedCategory\.value\s*=\s*normalizeCategory\(data\.scope\)/);
  });
  it('isResourceMode 计算属性且编辑态互斥', () => {
    expect(src).toMatch(/const isResourceMode\s*=\s*computed<boolean>/);
    expect(src).toMatch(/selectedCategory\.value == 'resource' && !isEdit\.value/);
  });
});

describe('resources 列表页视觉语言对齐契约（#758）', () => {
  it('my-uploads：圆形图标 + 胶囊状态徽标 + 卡片无描边 + 驳回提示条', () => {
    const src = read('pages/resources/my-uploads.uvue');
    expect(src).toMatch(/\.upload-item-icon-box \{[^}]*border-radius: 40rpx/);
    expect(src).toContain('upload-item-badge');
    expect(src).toMatch(/\.upload-item-badge \{[^}]*border-radius: 24rpx/);
    const itemRule = src.match(/\.upload-item \{[^}]*\}/)[0];
    expect(itemRule).not.toContain('border:');
    expect(src).toContain('upload-item-reason-box');
  });
  it('my-purchases：圆形图标 + chevron + 卡片无描边 + 空态去商城按钮', () => {
    const src = read('pages/resources/my-purchases.uvue');
    expect(src).toMatch(/\.course-item-icon-box \{[^}]*border-radius: 40rpx/);
    expect(src).toContain('course-item-arrow');
    const itemRule = src.match(/\.course-item \{[^}]*\}/)[0];
    expect(itemRule).not.toContain('border:');
    expect(src).toContain('去商城逛逛');
    expect(src).toMatch(/reLaunch\(\{ url: '\/pages\/courses\/courses' \}\)/);
  });
});
