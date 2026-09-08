/**
 * 个人信息手术契约测试（refs #676 / T03d，parent #641）
 *
 * 沿用源码契约缝（.uvue 不可 jest import）。先例：wrongQuestionsContract、mallPilotContract。
 * 钉住：拆出物存在与接线、状态所有权（页面持 ref / flows 零可变状态）、
 * auth 触点回调注入、UI 像素结构、600 预算机检、allowlist 不回潮。
 */
const fs = require('fs');
const path = require('path');

const ROOT = path.join(__dirname, '..');
const read = (rel) => fs.readFileSync(path.join(ROOT, rel), 'utf8');

const PAGE = 'pages/profile/personal-info.uvue';
const SHELL = 'pages/profile/components/info-dialog.uvue';
const FLOWS = 'pages/profile/composables/personal-info-flows.uts';

describe('拆出物存在契约（T03d 安置：模块私有 components/composables）', () => {
  it.each([[SHELL, 'info-dialog.uvue'], [FLOWS, 'personal-info-flows.uts']])('%s 存在', (rel) => {
    expect(fs.existsSync(path.join(ROOT, rel))).toBe(true);
  });
});

describe('InfoDialog 壳接线契约', () => {
  const page = () => read(PAGE);
  const shell = () => read(SHELL);

  it('页面显式 import 壳组件（Q17 安置，非 easycom）', () => {
    expect(page()).toContain("./components/info-dialog.uvue");
  });

  it('页面挂载 6 个弹窗全部走 <InfoDialog :show>（对应 6 个显隐 ref）', () => {
    const p = page();
    expect(p.match(/<InfoDialog :show="show\w+Dialog"/g)).toHaveLength(6);
    for (const s of ['showNicknameDialog', 'showPasswordDialog', 'showPhoneDialog',
      'showEmailDialog', 'showAccountDialog', 'showCompanyDialog']) {
      expect(p).toContain(`:show="${s}"`);
      expect(p).toContain(`@close="${s} = false"`);
    }
  });

  it('壳纯展示：零 api/零 store/零定时器，仅 close/confirm 两事件', () => {
    const c = shell();
    expect(c).not.toMatch(/import.*(api|stores)\//);
    expect(c).not.toMatch(/setInterval|showToast|showLoading/);
    expect(c).toContain("defineEmits(['close', 'confirm'])");
    expect(c).toContain('<slot></slot>');
  });
});

describe('flows 纯函数契约（状态所有权留在页面）', () => {
  const page = () => read(PAGE);
  const flows = () => read(FLOWS);

  it('flows 零模块级可变状态（export const 只允许 Ref 实例化除外——本模块一处都不得有）', () => {
    expect(flows()).not.toMatch(/^export const/m);
    expect(flows()).not.toMatch(/^let \w+/m);
  });

  it('弹窗显隐与输入 ref 全部留在页面（模板 v-model 绑页面局部 ref）', () => {
    const p = page();
    for (const r of ['nicknameInput', 'passwordCode', 'newPassword', 'confirmPassword',
      'newPhone', 'phoneCode', 'emailInput', 'emailCode', 'accountInput', 'accountCode', 'companyInput']) {
      expect(p).toContain(`const ${r} = ref<string>('')`);
    }
    expect(p.match(/v-model="/g)).toHaveLength(11);
  });

  it('页面 api 触点下沉：页面不 import api/auth 与 api/helpers；flows 统一 import', () => {
    expect(page()).not.toMatch(/from '\.\.\/\.\.\/api\//);
    expect(flows()).toContain("from '../../../api/auth'");
    expect(flows()).toContain("from '../../../api/helpers'");
  });

  it('flows 零 store 依赖：登录态替换/清空经回调回交页面（error18 桥接教训，同 #687 stats 归属原则）', () => {
    expect(flows()).not.toMatch(/stores\/auth|useAuthStore/);
    expect(flows()).toContain('onAuthed(result)');
    expect(flows()).toContain('onDeleted()');
    const p = page();
    expect(p).toContain('authStore.setAuthData(r.token, r.user, r.refresh_token)');
    expect(p).toContain('authStore.clearAuthData()');
  });

  it('倒计时收敛于 Countdown 类（单一实现）；页面 4 实例、onUnload 全停', () => {
    const f = flows();
    expect(f.match(/class Countdown/g)).toHaveLength(1);
    expect(f).not.toMatch(/startPwdCd|stopPwdTimer|startPhoneCd|stopPhoneTimer/);
    const p = page();
    expect(p.match(/new Countdown\(\)/g)).toHaveLength(4);
    expect(p).toMatch(/onUnload\(\(\) => \{[\s\S]*?pwdCd\.stop\(\)[\s\S]*?phoneCd\.stop\(\)[\s\S]*?emailCd\.stop\(\)[\s\S]*?accountCd\.stop\(\)/);
    // 页面不得残留裸定时器管理
    expect(p).not.toMatch(/setInterval|clearInterval/);
  });

  it('展示文案纯函数唯一定义点在 flows，页面 computed 全部委托（无第二实现）', () => {
    for (const fn of ['displayNameOf', 'avatarUrlOf', 'hasPendingOf', 'accountTextOf', 'phoneTextOf',
      'emailTextOf', 'companyTextOf', 'passwordTitleOf', 'passwordHintOf']) {
      expect(flows()).toContain(`export function ${fn}(`);
      expect(read(PAGE)).toContain(`${fn}(authStore.user.value)`);
    }
  });

  it('页面对已搬走符号零悬空使用（编译盲区锁：定义删了调用必须同步）', () => {
    const p = page();
    for (const sym of ['doDeleteAccount', 'startPwdCd', 'stopPwdTimer', 'validatePhoneNum',
      'validateEmail', 'errMsg(', 'uploadAvatarApi', 'updateProfileApi', 'changePasswordApi',
      'profileBindPhoneApi', 'profileBindEmailApi', 'updateAccountApi', 'deleteAccountApi',
      'pwdTimer', 'phoneTimer', 'emailTimer', 'accountTimer']) {
      expect(p).not.toContain(sym);
    }
  });
});

describe('UI 像素结构锁定（重构不动 UI：cells 与弹窗内控件逐数核对）', () => {
  const p = read(PAGE);

  it('七个设置 cell + 注销 cell + 退出按钮的标签与回调名不变', () => {
    for (const t of ['修改头像', '昵称', '登录账号', '手机号', '邮箱', '单位']) {
      expect(p).toContain(`>${t}<`);
    }
    expect(p).toContain('cell-label-danger">注销账号<');
    expect(p).toContain('logout-text">退出当前账号<');
    expect(p).toContain('<text v-if="hasPending" class="cell-badge">审核中</text>');
  });

  it('弹窗内控件形态不变：验证码按钮 ×4、提示行 ×3、确认文案（提交审核/确认/保存）', () => {
    expect(p.match(/class="code-btn"/g)).toHaveLength(4);
    expect(p.match(/class="dialog-tip"/g)).toHaveLength(3);
    expect(p.match(/获取验证码/g)).toHaveLength(4);
    expect(p).toContain('confirm-text="提交审核"');
    expect(p.match(/confirm-text="确认"/g)).toHaveLength(4);
    expect(p).toContain('confirm-text="保存"');
    expect(p).toContain(':title="emailText == \'未绑定\' ? \'绑定邮箱\' : \'修改邮箱\'"');
  });

  it('页面保留 slot 内容样式（input/code-btn/dialog-tip），壳样式已入 InfoDialog（无双写）', () => {
    for (const cls of ['.input', '.input-row', '.code-btn', '.code-btn-disabled', '.code-btn-text', '.dialog-tip']) {
      expect(p).toContain(cls);
      expect(read(SHELL)).not.toContain(cls);
    }
    for (const cls of ['.dialog-mask', '.dialog-title', '.dialog-body', '.dialog-footer', '.dialog-btn-text-primary']) {
      expect(read(SHELL)).toContain(cls);
      expect(p).not.toContain(cls);
    }
  });
});

describe('600 行软预算机检（personal-info 手术文件）', () => {
  it('页面 + 壳 + flows 全部 ≤600 行', () => {
    const over = [PAGE, SHELL, FLOWS]
      .map((rel) => ({ file: rel, lines: read(rel).split('\n').length }))
      .filter((x) => x.lines > 600);
    expect(over).toEqual([]);
  });
});

describe('allowlist 不回潮（个人信息域违例清零的锁）', () => {
  it('GUARD_ALLOWLIST 不含 personal-info 手术相关文件', () => {
    const guardSrc = read('utils/utsAndroidCompile.test.js');
    const start = guardSrc.indexOf('const GUARD_ALLOWLIST');
    expect(start).toBeGreaterThan(-1);
    const block = guardSrc.slice(start, guardSrc.indexOf('};', start));
    expect(block).not.toMatch(/personal-info|info-dialog|personal-info-flows/);
  });
});
