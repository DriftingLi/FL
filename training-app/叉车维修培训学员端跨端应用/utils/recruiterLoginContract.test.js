/**
 * 招聘者登录页口令校验契约（#1295）
 *
 * 钉住 #1295 交付的那一档校验，形态照 `utils/loginContract.test.js` 的「四通道校验文案逐字仍在」：
 *
 * 1) `pages/recruiter/login.uvue` 的 `validate()` 三档齐备且**顺序正确**：账号非空 → 口令非空 →
 *    口令下限。顺序不能反 —— 反了会让「什么都没填」报成「密码至少 6 位」，把既有那条更准的文案挤掉。
 * 2) 下限档的**整行逐字与学员登录页同形**（`pages/login/composables/useLoginForm.uts` 的 `validate()`
 *    里同一行）。这是本票的核心不变式：缺口是「家族不一致」，修法若自造第三种文案，等于把不一致换个
 *    形式重新引入。#1297 记录的正是这类文案漂移（同一句话在不同页里差两个空格）。
 * 3) 本票**只补下限**：不判 20 位上限、不碰 `:maxlength`（保持 32）。上限口径归 #1262 的三页对齐一起裁，
 *    故这里有一条反向锁 —— 哪天要放宽，必须显式翻锁并引用票号。
 *
 * 分类（`docs/agents/guards.md`）：**接线守护**（读源文本，不构成契约测试门的承重证据）。
 * 它守的是「这一档校验在不在、文案与学员登录页是否同一行」这条接线；行为兜底 = 本地编译门（Kotlin 形态）
 * + 真机门逐页冒烟（本页属常规运行时面：改动命中 `.uvue`）。
 *
 * 判别力自检：每条判据都配一条**注入变形的样本**（抽掉档位 / 调换顺序 / 改成第三种文案），
 * 证明检测器真的会红 —— 「只跑通过的那一次不算验收」。
 */
const h = require('./contractHarness');

const PAGE = 'pages/recruiter/login.uvue';
const STUDENT_FORM = 'pages/login/composables/useLoginForm.uts';

const ARM_ACCOUNT = "if (username.value.trim().length == 0) return '请输入招聘者账号'";
const ARM_EMPTY = "if (password.value.length == 0) return '请输入密码'";
const ARM_MIN = "if (password.value.length < 6) return '密码至少 6 位'";

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

/** 纯检测器：返回违规清单（空 = 合规）。提成函数是为了让注入样本能验证它真的会红。 */
function validateIssues(body) {
  const out = [];
  const at = (arm) => body.indexOf(arm);
  for (const [arm, what] of [
    [ARM_ACCOUNT, '缺账号非空档'],
    [ARM_EMPTY, '缺口令非空档'],
    [ARM_MIN, '缺口令下限档（#1295）'],
  ]) {
    if (at(arm) === -1) out.push(what);
  }
  if (at(ARM_EMPTY) !== -1 && at(ARM_MIN) !== -1 && at(ARM_MIN) < at(ARM_EMPTY)) {
    out.push('口令下限档排在非空档之前（空口令会报成「至少 6 位」）');
  }
  return out;
}

describe('招聘者登录页口令校验契约（#1295）', () => {
  const page = () => h.read(PAGE);
  const body = () => fnBody(page(), 'validate');

  it('validate() 三档齐备且顺序正确（账号非空 → 口令非空 → 口令下限）', () => {
    expect(fnBody(page(), 'validate')).not.toBe('');
    expect(validateIssues(body())).toEqual([]);
  });

  it('下限档整行与学员登录页同形（防自造第三种文案）', () => {
    const student = fnBody(h.read(STUDENT_FORM), 'validate');
    expect(student).toContain(ARM_MIN);
    expect(body()).toContain(ARM_MIN);
  });

  it('只补下限：本页不判 20 位上限、输入上界保持 32（口径归 #1262 三页对齐）', () => {
    expect(body()).not.toContain('password.value.length > 20');
    expect(page().split(':maxlength="32"').length - 1).toBe(1);
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
      expect(validateIssues(swapped)).toEqual(['口令下限档排在非空档之前（空口令会报成「至少 6 位」）']);
    });

    it('改成第三种文案（补上上限、换个说法）⇒ 两条判据同时失配', () => {
      const reworded = body().replace(ARM_MIN, "if (password.value.length < 6) return '密码长度需为6-20位'");
      expect(reworded).not.toBe(body());
      expect(validateIssues(reworded)).toContain('缺口令下限档（#1295）');
      expect(reworded).not.toContain(ARM_MIN);
    });
  });
});
