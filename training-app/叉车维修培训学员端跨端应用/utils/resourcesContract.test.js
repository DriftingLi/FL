/**
 * 资源上传模块完工契约测试（refs #710）
 *
 * 背景：resources 三页半成品——upload-resource 提交死端（「上传功能开发中」toast）、
 * my-uploads 永远空列表（零 API 调用）、my-purchases 拿 profile 的课程进度假充已购、
 * forum 资源 tab 上传入口跳 forum-create?scope=resource（后端 category 仅认
 * discussion/question/experience，resource 必 400「帖子类别无效」）。
 * 后端投稿域（contributions，#517/#611/#702）即资源上传 endpoint：先传后交 + mine 列表。
 *
 * 缝：api/*.uts 与页面 .uvue 无法在 jest 中 import，沿用源码契约测试缝
 * （先例 utils/checkinWiringContract.test.js）。
 *
 * 钉住的契约：
 * 1) 提交链路：upload-resource 消费 api/contribution 的上传+创建投稿函数
 * 2) 数据源：my-uploads 消费 getMyContributionsApi；my-purchases 消费 getStudentCoursesApi
 * 3) 死端清零：「上传功能开发中」与 forum-create?scope=resource 导航字面量全域清零
 * 4) 路由与类型：api/contribution.uts 三条路由、字段对齐后端 json tag（守护规则 H）
 * 5) 400 家族：forum-create 默认 discussion + 非法 scope/回填归一
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

/** 去注释后比对（避免自述性注释误伤 not-contains 断言） */
const stripComments = (src) =>
  src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|\s)\/\/[^\n]*/g, '$1');

describe('提交链路契约（upload-resource 消费 api/contribution）', () => {
  const src = read('pages/resources/upload-resource.uvue');
  it('上传与创建投稿函数 import 自 api/contribution', () => {
    const importRe =
      /import\s*\{[^}]*\b(uploadContributionFileApi|createContributionApi)\b[^}]*\}\s*from\s*'\.\.\/\.\.\/api\/contribution'/;
    expect(importRe.test(src)).toBe(true);
  });
  it('提交死端文案「上传功能开发中」清零', () => {
    expect(src).not.toContain('上传功能开发中');
  });
  it('客户端前置校验齐备（白名单/20MB/50MB/5 个文件）', () => {
    expect(src).toContain("'pdf'");
    expect(src).toContain('20 * 1024 * 1024');
    expect(src).toContain('50 * 1024 * 1024');
    expect(src).toMatch(/maxFiles\s*=\s*5/);
  });
  it('证件门槛：无证件阻断提交（credentialId <= 0）', () => {
    expect(src).toContain('getCurrentCredentialApi');
    expect(src).toMatch(/credentialId\.value\s*<=\s*0/);
  });
  it('App 端选文件走 uni.chooseFile，chooseImage 不再用于资源投稿', () => {
    expect(src).toContain('uni.chooseFile');
    expect(src).not.toContain('uni.chooseImage');
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

describe('forum 资源入口改跳契约', () => {
  it('资源面板上传格子改跳 /pages/resources/upload-resource', () => {
    const src = read('pages/forum/components/forum-resource-panel.uvue');
    expect(src).toContain("'/pages/resources/upload-resource'");
    expect(src).not.toMatch(/['"]\/pages\/forum\/forum-create\?scope=resource['"]/);
  });
  it('forum onCreate 资源 tab 分支改跳上传资源页；scope 不再产 resource/general', () => {
    const src = read('pages/forum/forum.uvue');
    expect(src).toContain("'/pages/resources/upload-resource'");
    expect(src).not.toMatch(/scope\s*=\s*'resource'/);
    expect(src).not.toMatch(/scope\s*=\s*'general'/);
  });
  it('全域不再以导航字面量发送 forum-create?scope=resource', () => {
    const offenders = [];
    const walk = (dir) => {
      for (const e of fs.readdirSync(dir, { withFileTypes: true })) {
        const p = path.join(dir, e.name);
        if (e.isDirectory()) { walk(p); continue; }
        if (!/\.(uts|uvue)$/.test(e.name) || /\.test\./.test(e.name)) continue;
        const src = fs.readFileSync(p, 'utf8');
        if (/['"]\/pages\/forum\/forum-create\?scope=resource['"]/.test(src)) {
          offenders.push(path.relative(ROOT, p));
        }
      }
    };
    walk(path.join(ROOT, 'api'));
    walk(path.join(ROOT, 'pages'));
    walk(path.join(ROOT, 'composables'));
    walk(path.join(ROOT, 'utils'));
    expect(offenders).toEqual([]);
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

describe('forum-create 非法类别归一契约', () => {
  const src = read('pages/forum/forum-create.uvue');
  it('默认类别为 discussion（非 general）', () => {
    expect(src).toMatch(/selectedCategory\s*=\s*ref<string>\('discussion'\)/);
    expect(src).not.toMatch(/ref<string>\('general'\)/);
  });
  it('URL scope 与编辑回填均过 normalizeCategory', () => {
    expect(src).toContain('normalizeCategory');
    expect(src).toMatch(/selectedCategory\.value\s*=\s*normalizeCategory\(`\$\{scope\}`\)/);
    expect(src).toMatch(/selectedCategory\.value\s*=\s*normalizeCategory\(data\.scope\)/);
  });
});

describe('上传页原型对齐契约（#754：模块分流 + 文件方式行）', () => {
  const src = read('pages/resources/upload-resource.uvue');
  const code = stripComments(src);
  it('选择模块 chips：资源=本页选中态，其余三 chip 跳 forum-create 对应 scope', () => {
    expect(src).toContain('选择模块');
    for (const s of ['discussion', 'question', 'experience']) {
      expect(src).toMatch(new RegExp("scope: '" + s + "'"));
    }
    expect(src).toMatch(/navigateTo\(\{ url: '\/pages\/forum\/forum-create\?scope=' \+ scope \}\)/);
  });
  it('文件方式三行齐备（微信聊天文档/本地文件上传/选择压缩文件）', () => {
    expect(src).toContain('微信聊天文档');
    expect(src).toContain('本地文件上传');
    expect(src).toContain('选择压缩文件');
  });
  it('图片上传行与第三方入口不实现（原型冲突项零残留）', () => {
    expect(code).not.toContain('图片上传');
    expect(code).not.toContain('金山');
    expect(code).not.toContain('WPS');
    expect(code).not.toContain('钉钉');
    expect(code).not.toContain('QQ文档');
  });
  it('微信端两行 chooseMessageFile 按扩展名过滤（文档行/zip 行），App 端本地行走 chooseFile', () => {
    expect(src).toContain('uni.chooseMessageFile');
    expect(src).toMatch(/pickFromMessage\(docExt\)/);
    expect(src).toMatch(/pickFromMessage\(\['zip'\]\)/);
    expect(src).toMatch(/extension: allowedExt/);
  });
});
