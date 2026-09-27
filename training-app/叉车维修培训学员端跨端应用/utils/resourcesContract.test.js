/**
 * 资源上传模块契约测试（refs #710 → #754/#756 → #760）
 *
 * 架构演进：#710 接通投稿链路（先传后交 + mine）；#754/#756 上传页原型对齐与统一发布；
 * #760 用户裁定「复用发布新帖页」——forum-create 分类行（ADR-0040 后三段：广场/资源/知识问答），
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
 * 6) 域 api 出口收紧（#653 T15 批 B）：material 域四个有载荷出口走 getMapped + build*；
 *    无 mapped 便捷面的 multipart / 编排出口留裸并登记理由；机械坑位由全工程守护执法（#654 起无豁免）
 */
/** harness：读取层归一 + 模块归属面（ADR-0023 票 C 起，本文件不再自建 ROOT / read / walker） */
const h = require('./contractHarness');
const ROOT = h.ROOT;
const read = h.read;

const stripComments = (src) =>
  src.replace(/\/\*[\s\S]*?\*\//g, '').replace(/(^|\s)\/\/[^\n]*/g, '$1');

/**
 * 取 `function <name>` 的函数体（到行首 `}` 收口，先例 utils/registerContract.test.js:96）。
 * `api/material.uts` 全程制表符缩进 ⇒ 行首 `}` 只出现在函数闭合处，无需花括号配平。
 * 名字不存在时返回 `''`：调用方的正向用例随即判红，不静默放过。
 */
function fnBodyOf(src, name) {
  const start = src.indexOf('function ' + name + '(');
  if (start === -1) return '';
  const end = src.indexOf('\n}', start);
  return end === -1 ? src.slice(start) : src.slice(start, end);
}

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
  it('状态徽标覆盖后端五态值域（判定收在 utils/contributionStatus 单点，#1108）', () => {
    const status = read('utils/contributionStatus.uts');
    for (const s of ['pending', 'approved', 'rejected', 'withdrawn', 'archived']) {
      expect(status).toContain(s);
    }
    expect(src).toMatch(
      /import\s*\{[^}]*\bdescribeContributionStatus\b[^}]*\}\s*from\s*'\.\.\/\.\.\/utils\/contributionStatus'/
    );
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
    for (const d of ['api', 'pages', 'composables', 'utils', 'stores']) {
      for (const rel of h.sourceFilesIn(d)) {
        if (read(rel).includes('/pages/resources/upload-resource')) offenders.push(rel);
      }
    }
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
    // #811：回填源改为详情 DTO 的 category（旧写法读不存在的 scope，问答/经验帖一进编辑态即降级 discussion）
    expect(src).toMatch(/selectedCategory\.value\s*=\s*normalizeCategory\(data\.category\)/);
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

describe('material 域 api 出口收紧（#653 T15 批 B：有载荷出口全部走 getMapped）', () => {
  const code = stripComments(read('api/material.uts'));

  /**
   * 判据本体（一处定义、两支用例共用）：一条「有载荷出口」的函数体若退回裸通路，这里列出违例。
   * 下面的注入自检跑的是**同一个函数** —— 判据被改坏到失去判别力时，注入用例立刻红。
   */
  function outletViolations(body, dto) {
    const v = [];
    if (!body.includes(`getMapped<${dto}>(`)) v.push(`未走 getMapped<${dto}>`);
    if (/(?:^|[^A-Za-z0-9_$])get\(/.test(body)) v.push('仍有裸 get( 调用');
    if (body.includes('.then(')) v.push('仍有 .then() 拆包');
    return v;
  }

  it('出口家族 import：换成 getMapped，裸 get 通路不再引', () => {
    expect(code).toMatch(/import\s*\{\s*getMapped\s*\}\s*from\s*'\.\/request'/);
    expect(code).not.toMatch(/import\s*\{[^}]*\bget\b[^}]*\}\s*from\s*'\.\/request'/);
  });

  it.each([
    ['getMaterialsApi', 'MaterialListResult', "'/materials', params"],
    ['getMaterialDetailApi', 'MaterialItem', "'/materials/' + materialId.toString(), null"],
    ['getMaterialDownloadApi', 'MaterialDownloadInfo', "'/materials/' + materialId.toString() + '/download', null"],
    ['getStudentMaterialsApi', 'MaterialListResult', "'/student/materials', params"],
  ])('%s 经 getMapped<%s> 且旧 get().then() 形态不回潮', (name, dto, callShape) => {
    const body = fnBodyOf(code, name);
    expect(body).not.toBe('');
    expect(body).toContain(`return getMapped<${dto}>(${callShape}`);
    expect(outletViolations(body, dto)).toEqual([]);
  });

  it('mapper 一律箭头包裹 build*（守护规则 M 的形态面；M 本身由 utsAndroidCompile 全工程执法，此处不复述判据）', () => {
    expect(code).toContain('(data : UTSJSONObject) : MaterialListResult => buildMaterialListResult(data)');
    expect(code).toContain('(data : UTSJSONObject) : MaterialItem => buildMaterialItem(data)');
    expect(code).toContain('(data : UTSJSONObject) : MaterialDownloadInfo => buildMaterialDownloadInfo(data)');
  });

  it('映射真相收在三个私有 builder（不对外暴露，页面只消费 DTO）', () => {
    for (const [name, dto] of [
      ['buildMaterialItem', 'MaterialItem'],
      ['buildMaterialListResult', 'MaterialListResult'],
      ['buildMaterialDownloadInfo', 'MaterialDownloadInfo'],
    ]) {
      expect(code).toMatch(new RegExp(`function\\s+${name}\\s*\\(\\s*\\w+\\s*:\\s*UTSJSONObject\\s*\\)\\s*:\\s*${dto}\\b`));
      expect(code).not.toMatch(new RegExp(`export\\s+function\\s+${name}\\b`));
    }
  });

  it('列表信封四个键只在 buildMaterialListResult 一处拆（两支列表出口共用同一真相）', () => {
    const body = fnBodyOf(code, 'buildMaterialListResult');
    expect(body).not.toBe('');
    for (const k of ['materials', 'total', 'page', 'pages']) {
      expect(body).toContain(`data['${k}']`);
    }
    // page 缺省 1 是行为保持点：转换前写在 getMaterialsApi 的 then 里，现在只能平移、不能改值
    expect(body).toMatch(/toNumber\(data\['page'\],\s*1\)/);
    expect(body).toContain('buildMaterialItem(arr[i])');
  });

  it('数值/字符串强转不再本地复制（api/helpers.uts 是唯一实现）', () => {
    expect(code).toMatch(/import\s*\{\s*toNumber,\s*toStr\s*\}\s*from\s*'\.\/helpers'/);
    expect(code).not.toMatch(/(^|\n)function\s+to(Number|Str)\s*\(/);
  });

  it('编排出口保持裸通路：downloadAndOpenMaterialApi 无响应载荷，错误通道就是 string', () => {
    const body = fnBodyOf(code, 'downloadAndOpenMaterialApi');
    expect(body).toContain('Promise<string>');
    expect(body).not.toContain('Mapped<');
    // 六条用户可见失败串逐字保持（保留裸通路是决策，不是惯性：#653 票面「调用方零行为改动」）
    for (const msg of [
      '资料地址无效',
      '资料下载失败（',
      '无法打开此文件',
      '资料下载超时，请稍后重试',
      '资料下载失败，请检查网络后重试',
      '资料下载失败',
    ]) {
      expect(body).toContain(msg);
    }
    // 终态出口计数（实测 7 条 = 六条带文案的失败 + 一条成功态 resolve('')）：
    // 「留裸」指的是不换出口通路，不是这段可以随便重写 —— 多一条静默分支这里就红。
    expect((body.match(/resolve\(/g) || []).length).toBe(6);
    expect((body.match(/return '/g) || []).length).toBe(1);
    expect(body).toMatch(/encodeURI\(info\.file_url\)\s*\?\?\s*''/);
    expect(body).toMatch(/\.catch\(\(e\)\s*:\s*string\s*=>/);
    // 留裸 ≠ 留旧出口：它必须改调收紧后的 getMaterialDownloadApi
    expect(body).toContain('getMaterialDownloadApi(materialId)');
  });

  it('出口判据具备判别力（把一条出口写回 get().then()，必须被同一条判据抓到）', () => {
    const regressed = code.replace(
      "return getMapped<MaterialItem>('/materials/' + materialId.toString(), null, (data : UTSJSONObject) : MaterialItem => buildMaterialItem(data))",
      "return get('/materials/' + materialId.toString()).then((data : UTSJSONObject) : MaterialItem => buildMaterialItem(data))",
    );
    // 注入必须真的落地（否则「替换失败」会让下面的判据空转）
    expect(regressed).not.toBe(code);
    // 同一判据跑注入体：三条违例一条都不能少
    expect(outletViolations(fnBodyOf(regressed, 'getMaterialDetailApi'), 'MaterialItem')).toEqual([
      '未走 getMapped<MaterialItem>',
      '仍有裸 get( 调用',
      '仍有 .then() 拆包',
    ]);
    // 对照组：真源跑同一判据必须零违例（防判据退化成「永远报三条」）
    expect(outletViolations(fnBodyOf(code, 'getMaterialDetailApi'), 'MaterialItem')).toEqual([]);
  });
});

describe('调用方零适配的前提锁（#653 票面「页面零改动」的实测依据）', () => {
  it('materials 页只从域 api 引函数、类型一律走 types/index ⇒ 收紧出口不触碰 import 面', () => {
    const page = read('pages/resources/materials.uvue');
    expect(page).toMatch(/import type \{ MaterialItem \} from '\.\.\/\.\.\/types\/index'/);
    const clause = page.match(/import \{([^}]*)\} from '\.\.\/\.\.\/api\/material'/);
    expect(clause).not.toBeNull();
    const names = clause[1].split(',').map((s) => s.trim()).filter((s) => s.length > 0);
    expect(names.length).toBeGreaterThan(0);
    // 任何一个非 `…Api` 的名字跨过这条缝，都意味着页面开始从 api 文件取类型（本票口径：类型在 types/index）
    expect(names.filter((n) => !/Api$/.test(n))).toEqual([]);
  });
});

describe('api/contribution.uts 裸出口白名单（#653：multipart 无 mapped 便捷面，保留须带理由）', () => {
  const src = read('api/contribution.uts');
  const code = stripComments(src);

  it('uploadContributionFileApi 留裸 uploadFile 通路（出口家族只有 requestMapped/getMapped/postMapped）', () => {
    const body = fnBodyOf(code, 'uploadContributionFileApi');
    expect(body).toContain('uploadFile(');
    expect(body).not.toContain('Mapped<');
  });

  it('留裸理由写进文件头（不是只写在 PR 里）', () => {
    expect(src).toMatch(/@note 裸出口白名单（T15 批 B #653 登记）/);
    expect(src).toContain('multipart');
  });

  it('有载荷的两条仍走 mapped 出口（白名单没有顺手扩大到整文件）', () => {
    expect(code).toContain('postMapped<');
    expect(code).toContain('getMapped<ContributionPageResult>');
  });
});

describe('幻影路由锁（#662 口径）：material 域路由必须落在后端已注册清单内', () => {
  /** 后端注册表：material.go 的 `g.METHOD("<path>")`（组前缀为空串，见该文件 rg.Group("")） */
  function registeredRoutes() {
    const go = stripComments(read('../../backend/internal/api/material.go'));
    return [...go.matchAll(/\bg\.(?:GET|POST|PUT|DELETE|PATCH)\("([^"]+)"[^)]*\)/g)].map((m) => m[1]);
  }

  /** 前端路由：拼段的 `'/x/' + materialId.toString() + '/y'` 先归一成 `/x/:id/y`，再抽全部字面量 */
  function apiRoutes() {
    const normalized = stripComments(read('api/material.uts'))
      .replace(/'((?:\/materials|\/student\/materials)\/[^']*)'\s*\+\s*materialId\.toString\(\)\s*\+\s*'([^']*)'/g, "'$1:id$2'")
      .replace(/'((?:\/materials|\/student\/materials)\/[^']*)'\s*\+\s*materialId\.toString\(\)/g, "'$1:id'");
    return [...normalized.matchAll(/'((?:\/materials|\/student\/materials)[^']*)'/g)].map((m) => m[1]);
  }

  it('四条路由全部命中 material.go 注册面（收紧只换出口、不改请求形态）', () => {
    const registered = registeredRoutes();
    expect(registered.length).toBeGreaterThan(3);
    const used = apiRoutes();
    expect(used.sort()).toEqual(['/materials', '/materials/:id', '/materials/:id/download', '/student/materials']);
    expect(used.filter((u) => !registered.includes(u))).toEqual([]);
  });
});
