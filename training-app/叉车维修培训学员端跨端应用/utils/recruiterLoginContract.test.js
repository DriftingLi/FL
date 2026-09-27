/**
 * 招聘者登录页口令校验契约（#1295）
 *
 * 钉住 #1295 交付的那一档校验，形态照 `utils/loginContract.test.js` 的「四通道校验文案逐字仍在」：
 *
 * 1) `pages/recruiter/login.uvue` 的 `validate()` 三档齐备且**顺序正确**：账号非空 → 口令非空 →
 *    口令下限。顺序不能反 —— 反了会让「什么都没填」报成「密码至少 6 位」，把既有那条更准的文案挤掉。
 * 2) 口令档的**家族一致性**：本票交付时招聘者页与学员登录页的口令档逐字同行（都是 `密码至少 6 位`），
 *    这是本票的核心不变式 —— 缺口是「家族不一致」，修法若自造第三种文案，等于把不一致换个形式重新引入。
 *    #1262 把学员 / 注册 / 找回三页一起翻成 6-20 区间（三页的取值形态不同：`password.value.length` 与
 *    `password.length`，故同形只可能锁**文案**那一截）后，承重面从「招聘者 ↔ 学员同一行」挪成
 *    「三页共用同一句区间文案」＋「招聘者页仍是本票那一行」：任何一页自造说法即红，两页分叉由票号显式
 *    记录，不是漂移。#1297 记录的正是这类文案漂移（同一句话在不同页里差两个空格）。
 * 3) 本票**只补下限**：不判 20 位上限、不碰 `:maxlength`（保持 32）。上限口径归 #1262 的三页对齐一起裁 ——
 *    #1262 已裁毕：它翻的是学员登录 / 注册 / 找回三页，**招聘者页在其票面显式排除**，故本页仍是下限句
 *    属裁定结果而非漏改。反向锁留着 —— 哪天要给本页也加上限，必须显式翻锁并引用票号。
 *
 * 分类（`docs/agents/guards.md`）：**接线守护**（读源文本，不构成契约测试门的承重证据）。
 * 它守的是「这一档校验在不在、四页之间的口令文案有没有各自漂移」这条接线；行为兜底 = 本地编译门（Kotlin 形态）
 * + 真机门逐页冒烟（本页属常规运行时面：改动命中 `.uvue`）。
 *
 * 判别力自检：每条判据都配一条**注入变形的样本**（抽掉档位 / 调换顺序 / 改成第三种文案），
 * 证明检测器真的会红 —— 「只跑通过的那一次不算验收」。
 */
const h = require('./contractHarness');

const PAGE = 'pages/recruiter/login.uvue';
const STUDENT_FORM = 'pages/login/composables/useLoginForm.uts';

/** 口令档文案的两句话：#1262 后三页用区间句，招聘者页用下限句 */
const MSG_MIN = '密码至少 6 位';
const MSG_RANGE = '密码长度需为 6-20 位';
/** 用区间句的三页（招聘者页不在内，见头部第 3 条） */
const RANGE_PAGES = [
  ['学员登录页', STUDENT_FORM],
  ['注册页', 'pages/register/composables/useRegisterForm.uts'],
  ['找回密码页', 'pages/forgot-password/composables/useForgotPasswordForm.uts'],
];

const ARM_ACCOUNT = "if (username.value.trim().length == 0) return '请输入招聘者账号'";
const ARM_EMPTY = "if (password.value.length == 0) return '请输入密码'";
const ARM_MIN = "if (password.value.length < 6) return '" + MSG_MIN + "'";

/** 取函数体：从声明起、到大括号配平（页面内联函数与 composable 局部函数同口径） */
function fnBody(src, name) {
  const start = src.indexOf('function ' + name + '(');
  if (start === -1) return '';
  const open = src.indexOf('{', start);
  if (open === -1) return '';
  let depth = 0;
  for (let i = open; i < src.length; i++) {
    if (src[i] === '{') depth++;
    else if (src[i] === '}' && --depth === 0) return src.slice(start, i + 1);
  }
  return src.slice(start);
}

/** 三档 + 头部承诺的顺序：账号非空 → 口令非空 → 口令下限 */
const ARMS = [
  [ARM_ACCOUNT, '账号非空档'],
  [ARM_EMPTY, '口令非空档'],
  [ARM_MIN, '口令下限档（#1295）'],
];

/** 纯检测器：返回违规清单（空 = 合规）。提成函数是为了让注入样本能验证它真的会红。 */
function validateIssues(body) {
  const out = [];
  for (const [arm, what] of ARMS) {
    if (body.indexOf(arm) === -1) out.push('缺' + what);
  }
  for (let i = 1; i < ARMS.length; i++) {
    const prev = body.indexOf(ARMS[i - 1][0]);
    const cur = body.indexOf(ARMS[i][0]);
    if (prev !== -1 && cur !== -1 && cur < prev) {
      out.push('顺序错位：' + ARMS[i][1] + ' 排在 ' + ARMS[i - 1][1] + ' 之前');
    }
  }
  return out;
}

/**
 * 家族文案检测器：返回违规清单（空 = 合规）。每页要么用区间句、要么用下限句，
 * 且**只许用一句** —— 半翻态（两句并存）与自造第三种说法都要被抓出来。
 */
function familyIssues(entries) {
  const out = [];
  for (const [what, body, want] of entries) {
    const [wantMsg, otherMsg] = want === 'range' ? [MSG_RANGE, MSG_MIN] : [MSG_MIN, MSG_RANGE];
    if (body.indexOf(wantMsg) === -1) out.push(what + ' 缺 ' + wantMsg);
    if (body.indexOf(otherMsg) !== -1) out.push(what + ' 多出 ' + otherMsg);
  }
  return out;
}

/** 四页（三页区间 + 招聘者下限）的家族清单，真源与注入样本同口径喂给 familyIssues */
function familyEntries() {
  const entries = RANGE_PAGES.map(([what, path]) => [what, fnBody(h.read(path), 'validate'), 'range']);
  entries.push(['招聘者页', fnBody(h.read(PAGE), 'validate'), 'min']);
  return entries;
}

describe('招聘者登录页口令校验契约（#1295）', () => {
  const page = () => h.read(PAGE);
  const body = () => fnBody(page(), 'validate');

  it('validate() 三档齐备且顺序正确（账号非空 → 口令非空 → 口令下限）', () => {
    expect(fnBody(page(), 'validate')).not.toBe('');
    expect(validateIssues(body())).toEqual([]);
  });

  it('口令档四页文案家族一致（#1262 后：三页区间句 + 招聘者页下限句，一句不多一句不少）', () => {
    expect(familyIssues(familyEntries())).toEqual([]);
  });

  it('只补下限：本页不判 20 位上限（上限口径归 #1262 三页对齐）', () => {
    expect(body()).not.toContain('password.value.length > 20');
  });

  describe('注入自检（检测器必须真的会红，防「只跑通过的那一次」）', () => {
    it('抽掉下限档 ⇒ 判红', () => {
      const broken = body().replace(ARM_MIN + '\n', '');
      expect(broken).not.toBe(body());
      expect(validateIssues(broken)).toEqual(['缺口令下限档（#1295）']);
    });

    it('下限档挪到非空档之前 ⇒ 判红（顺序会吞掉更准的文案）', () => {
      const swapped = body()
        .replace(ARM_EMPTY + '\n        ' + ARM_MIN, ARM_MIN + '\n        ' + ARM_EMPTY);
      expect(swapped).not.toBe(body());
      expect(validateIssues(swapped)).toEqual(['顺序错位：口令下限档（#1295） 排在 口令非空档 之前']);
    });

    it('账号档挪到两档口令之后 ⇒ 判红（三档的顺序都承重，不是只盯新加那一档）', () => {
      const moved = body().replace(ARM_ACCOUNT + '\n        ', '').replace("return ''", ARM_ACCOUNT + "\n        return ''");
      expect(moved).not.toBe(body());
      expect(validateIssues(moved)).toEqual(['顺序错位：口令非空档 排在 账号非空档 之前']);
    });

    it('改成第三种文案（补上上限、换个说法）⇒ 两条判据同时失配', () => {
      const reworded = body().replace(ARM_MIN, "if (password.value.length < 6) return '密码长度需为6-20位'");
      expect(reworded).not.toBe(body());
      expect(validateIssues(reworded)).toContain('缺口令下限档（#1295）');
      expect(reworded).not.toContain(ARM_MIN);
    });

    it('学员登录页改回下限句（半翻态、两句并存）⇒ 家族检测器两条都报', () => {
      const student = fnBody(h.read(STUDENT_FORM), 'validate');
      const half = student.replace(MSG_RANGE, MSG_MIN);
      expect(half).not.toBe(student);
      expect(familyIssues([['学员登录页', student, 'range']])).toEqual([]);
      expect(familyIssues([['学员登录页', half, 'range']]))
        .toEqual(['学员登录页 缺 ' + MSG_RANGE, '学员登录页 多出 ' + MSG_MIN]);
    });

    it('某一页自造第三种说法（同句去掉空格）⇒ 家族检测器只报「缺」，不误报「多出」', () => {
      const student = fnBody(h.read(STUDENT_FORM), 'validate');
      const third = student.replace(MSG_RANGE, '密码长度需为6-20位');
      expect(third).not.toBe(student);
      expect(familyIssues([['学员登录页', third, 'range']])).toEqual(['学员登录页 缺 ' + MSG_RANGE]);
    });
  });
});
