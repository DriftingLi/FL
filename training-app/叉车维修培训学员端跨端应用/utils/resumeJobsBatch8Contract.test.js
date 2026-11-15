/**
 * B8 #2524 招聘 C 端收口契约（job-list / resume 双态 / resume-edit 双态 + 三入口卡）
 *
 * 钉住本票的**裁定面**（不是像素面）：
 * 1) G2 G-05：job-list 的「综合排序」下拉与「匹配度」徽标是**整屏删除项**，不得以「原型为准」回潮；
 * 2) G2 G-10：卡面事实字段只渲染后端确有者（工作单位 / 任职要求），供招人数·公司规模·行业四类不硬塞；
 * 3) 「招聘者 tab → 485:5793」入口补线被 B1 #2517 登录页裁定第 1 条取代（招聘者 tab = 真实 B 端登录），
 *    故 login 页不得出现指向 resume 的跳转（防有人按旧票面回接线）；
 * 4) 切图落位：求职者 / 招聘-我的简历 两目录的 5 张 3D 图入包，模板写字面量、包内 PNG（禁 webp）；
 * 5) 三入口卡 = 贴片 486:5873（在线简历 / 附件简历 / 求职意向），每卡都真的能导航；
 * 6) resume-edit 完善度卡：3D 机器人 + 隐私保护 chip，仍只有 `completion` 一个 prop（T09 拆分判据不放宽）。
 *
 * 本套件是接线/结构守护；行为证据在 ③ 两腿与 ④c 编译门。
 */
const h = require('./contractHarness');

const read = h.read;

/** 剥注释（模板 `<!-- -->` + 样式/脚本 `/* *\/` 与行注释）：裁定面判据只判**代码**，
 *  注释里正当地写着「综合排序 / 匹配度已删」时不得被自己命中（T05 已录的「锁被断言字符串自身命中」坑位）。 */
function stripComments(src) {
  return src
    .replace(/<!--[\s\S]*?-->/g, '')
    .replace(/\/\*[\s\S]*?\*\//g, '')
    .replace(/(^|[^:])\/\/[^\n]*/g, '$1');
}

const LIST = 'pages/jobs/job-list.uvue';
const RESUME = 'pages/resume/resume.uvue';
const EDIT = 'pages/resume/resume-edit.uvue';
const PROGRESS = 'pages/resume/components/resume-progress-card.uvue';

describe('job-list（原型 700:7139）：删除项不回潮 + 事实字段只渲染确有的', () => {
  const list = stripComments(read(LIST));

  it('G-05：综合排序下拉与匹配度徽标不得回潮（含 match_score 字段名）', () => {
    expect(list).not.toContain('综合排序');
    expect(list).not.toContain('匹配度');
    expect(list).not.toContain('match_score');
  });

  it('G-10：工作单位 / 任职要求来自真实字段，供招人数·公司规模·行业不硬塞', () => {
    expect(list).toContain('工作单位：');
    expect(list).toContain('任职要求：');
    for (const banned of ['供招', '公司规模', 'headcount', 'company_scale', 'industry']) {
      expect(list).not.toContain(banned);
    }
  });

  it('布局：tab 行与筛选条同处一张白卡（.panel），职位卡走 $radius-card', () => {
    const panel = /<view class="panel">([\s\S]*?)<scroll-view/.exec(list);
    expect(panel).not.toBeNull();
    expect(panel[1]).toContain('class="tab-row"');
    expect(panel[1]).toContain('class="filter-row"');
    expect(list).toContain('border-radius: $radius-card;');
  });
});

describe('resume 三入口卡（贴片 486:5873）与切图落位', () => {
  const resume = read(RESUME);

  it('三张入口卡都在：在线简历 / 附件简历 / 求职意向，且各自绑定了导航', () => {
    expect(resume).toContain('>在线简历<');
    expect(resume).toContain('>附件简历<');
    expect(resume).toContain('>求职意向<');
    expect(resume).toMatch(/class="func-card" @click="onOnlineResume"/);
    expect(resume).toMatch(/class="func-card" @click="onAttachmentResume"/);
    expect(resume).toMatch(/class="func-card" @click="onIntention"/);
  });

  it('求职意向落点 = resume-edit 的「求职期望」卡（全原型无独立屏，按裁定 9 上下文推断）', () => {
    expect(resume).toMatch(/function onIntention\(\)[\s\S]*?\/pages\/resume\/resume-edit/);
    expect(resume).not.toContain('/pages/jobs/jobs-intention');
  });

  it('「去填写简历」卡：3D 卷轴切图替代 emoji 字形，完善度条与文案按既有契约保留', () => {
    expect(resume).toContain('src="/static/icons/resume/resume-guide.png"');
    expect(resume).not.toContain('resume-guide-icon');
    expect(resume).toContain('完善度 {{ resumePct }}%');
    expect(resume).toMatch(/width: resumePct \+ '%'/);
  });
});

describe('切图入包：5 张 3D 图在位、模板写字面量、包内一律 PNG', () => {
  const IMAGES = [
    ['static/icons/resume/resume-guide.png', '/static/icons/resume/resume-guide.png'],
    ['static/icons/resume/online.png', '/static/icons/resume/online.png'],
    ['static/icons/resume/attach.png', '/static/icons/resume/attach.png'],
    ['static/icons/resume/intention.png', '/static/icons/resume/intention.png'],
    ['static/icons/resume/progress.png', '/static/icons/resume/progress.png'],
  ];

  it.each(IMAGES)('%s 文件在位且被模板以字面量引用', (file, literal) => {
    expect(h.exists(file)).toBe(true);
    // 包内图不含 webp（小程序 <image> 不解析包内 webp，ADR-0066）：文件名后缀即判据
    expect(file.endsWith('.webp')).toBe(false);
    const pages = read(RESUME) + read(EDIT) + read(PROGRESS);
    expect(pages).toContain('"' + literal + '"');
  });
});

describe('resume-edit 完善度卡（原型 671:5543 0% / 737:1711 40%）', () => {
  const progress = read(PROGRESS);
  const edit = read(EDIT);

  it('结构：请完善你的简历信息 + 隐私保护 chip + 3D 机器人 + 百分比', () => {
    expect(progress).toContain('请完善你的简历信息');
    expect(progress).toContain('隐私保护');
    expect(progress).toContain('src="/static/icons/resume/progress.png"');
    expect(progress).toContain('{{ completion }}%');
    expect(progress).toMatch(/width: completion \+ '%'/);
  });

  it('接口面不放宽：组件仍只有 completion 一个 prop、零 emit', () => {
    const props = /defineProps<\{([\s\S]*?)\}>/.exec(progress);
    expect(props).not.toBeNull();
    expect([...props[1].matchAll(/([A-Za-z_$][\w$]*)\s*\??\s*:/g)].map((m) => m[1])).toEqual(['completion']);
    expect(progress).not.toMatch(/defineEmits/);
  });

  it('标题仍是「在线简历」（原型框名「上传」是误标，票面已目检纠正）', () => {
    expect(edit).toMatch(/<app-nav-bar title="在线简历"/);
    expect(edit).not.toContain('title="上传"');
  });
});

describe('入口补线：B1 #2517 裁定取代旧票面的「招聘者 tab → resume」', () => {
  const login = read('pages/login/login.uvue');
  const useLoginForm = read('pages/login/composables/useLoginForm.uts');

  it('招聘者 tab 仍是真实 B 端登录落 recruiter/home，不得改指 C 端简历域', () => {
    expect(useLoginForm).toContain("uni.reLaunch({ url: '/pages/recruiter/home' })");
    expect(useLoginForm).not.toContain('/pages/resume/resume');
    expect(login).not.toContain('/pages/resume/resume');
  });

  it('C 端入口链仍在：我的 → 就业在线 → job-list → 头像 → resume', () => {
    expect(read('pages/profile/profile.uvue')).toContain("path: '/pages/jobs/job-list'");
    expect(read(LIST)).toMatch(/class="tab-avatar-btn" @click="onMine"/);
    expect(read(LIST)).toContain("uni.navigateTo({ url: '/pages/resume/resume' })");
  });
});
