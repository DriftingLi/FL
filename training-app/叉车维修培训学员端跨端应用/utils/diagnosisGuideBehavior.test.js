/**
 * 诊断引导的**四步状态机与组句** · 行为级测试（#2296，行为腿）
 *
 * 为什么是行为而不是源码文本：票面验收写的是「四步状态机与拼句逻辑有**真执行**的行为测试
 * （不是读源码字面量）」。源码文本契约证明不了「换品牌后旧现象真的失效」「关闭再进真的没残留」
 * 「型号档的第二层真的去 faults 根层」——这些只有把状态往前推一遍才看得见
 * （判据与分类见 `docs/agents/guards.md`）。
 *
 * 缝：`utils/utsHarness.js` 的 `loadUts`。**两层都是真执行出来的源**：
 * `utils/diagnosisCompose.uts` 与 `utils/diagnosisGuide.uts` 都不是手抄镜像
 * ——镜像会让「真源改了而镜子没改」静默变绿（#1273 先例）。
 *
 * 成对取证：末组用注入变异的副本证明上面的判据**有牙**（两种真实坏实现：
 * ① 换品牌不清第二层/现象；② 型号档也走 systems）。
 */
const path = require('path');
const { loadUts, readText } = require('./utsHarness');

const COMPOSE_UTS = path.join(__dirname, 'diagnosisCompose.uts');
const GUIDE_UTS = path.join(__dirname, 'diagnosisGuide.uts');

/** 真源装配：guide 的两个依赖取自 compose 的真实导出（不手抄） */
function loadGuide(file) {
  const compose = loadUts(COMPOSE_UTS, {});
  const guide = loadUts(file || GUIDE_UTS, {
    emptyDiagnosisSelection: compose.emptyDiagnosisSelection,
    pickerKindOf: compose.pickerKindOf,
  });
  return { compose, guide };
}

const ZHONGLI = { id: 'zhongli', name: '中力', tagline: '', icon: '🏭', cls: '', picker: 'system' };
const LINDE = { id: 'linde', name: '林德', tagline: '', icon: '🏗', cls: '', picker: 'model' };
/** 契约里没有的第三个取值：按系统档处理（已知未定项，不报警） */
const WEIRD = { id: 'x', name: '某品牌', tagline: '', icon: '', cls: '', picker: 'series' };

describe('诊断引导状态机（#2296）', () => {
  const { compose, guide } = loadGuide();

  describe('① 判档：第二层走 systems 还是 faults 根层', () => {
    it('系统档（中力）⇒ systems；型号档（林德）⇒ faults 根层', () => {
      expect(guide.secondSourceOf(ZHONGLI)).toBe('systems');
      expect(guide.secondSourceOf(LINDE)).toBe('faults-root');
    });

    it('品牌未定 / picker 是未知取值 ⇒ 没有可取列表 / 按系统档', () => {
      expect(guide.secondSourceOf(null)).toBe('');
      expect(compose.pickerKindOf(WEIRD)).toBe('system');
      expect(guide.secondSourceOf(WEIRD)).toBe('systems');
      expect(compose.pickerKindOf(null)).toBe('system');
    });

    it('步骤标题跟着档位变（这是学员看到的第一层线索）', () => {
      expect(compose.secondStepTitle(ZHONGLI)).toBe('选系统');
      expect(compose.secondStepTitle(LINDE)).toBe('选型号');
      expect(compose.faultStepTitle(ZHONGLI)).toBe('选故障现象');
      expect(compose.faultStepTitle(LINDE)).toBe('选要查的维修部位 / 主题');
    });
  });

  describe('② 四步推进', () => {
    it('初始态停在品牌步，且没有任何已选', () => {
      const s = guide.initialGuideState();
      expect(s.step).toBe(guide.GUIDE_STEP_BRAND);
      expect(s.brandId).toBe('');
      expect(s.selection.faultName).toBe('');
    });

    it('选品牌 → 第二步；选第二层 → 第三步；选现象 → 确认步', () => {
      let s = guide.initialGuideState();
      s = guide.selectBrand(s, 'zhongli');
      expect(s.step).toBe(guide.GUIDE_STEP_SECOND);
      s = guide.applySecond(s, 'battery', '电池系统', false);
      expect(s.step).toBe(guide.GUIDE_STEP_FAULT);
      expect(s.selection.secondId).toBe('battery');
      s = guide.selectFault(s, '举升无力');
      expect(s.step).toBe(guide.GUIDE_STEP_CONFIRM);
      expect(s.selection.faultName).toBe('举升无力');
    });

    it('每一步都**不改入参**（状态是不可变对象：关闭面板时丢掉它就等于清干净）', () => {
      const s0 = guide.initialGuideState();
      const s1 = guide.selectBrand(s0, 'zhongli');
      expect(s0.brandId).toBe('');
      expect(s1.brandId).toBe('zhongli');
      const s2 = guide.applySecond(s1, 'battery', '电池系统', false);
      expect(s1.secondId).toBe('');
      expect(s2.secondId).toBe('battery');
      const s3 = guide.selectFault(s2, '举升无力');
      expect(s2.selection.faultName).toBe('');
      expect(s3.selection.faultName).toBe('举升无力');
    });

    it('能跳的步受已选约束：没选品牌跳不到第二步，没选第二层跳不到第三步', () => {
      const s = guide.initialGuideState();
      expect(guide.canJumpTo(s, 0)).toBe(true);
      expect(guide.canJumpTo(s, 1)).toBe(false);
      expect(guide.canJumpTo(s, 2)).toBe(false);
      expect(guide.canJumpTo(s, 3)).toBe(false);
      expect(guide.jumpTo(s, 2).step).toBe(guide.GUIDE_STEP_BRAND); // 不允许 ⇒ 原样
      const s1 = guide.selectBrand(s, 'zhongli');
      expect(guide.canJumpTo(s1, 1)).toBe(true);
      expect(guide.canJumpTo(s1, 2)).toBe(false);
    });

    it('越界步号不抛（面板上点错不该炸），原样返回', () => {
      const s = guide.initialGuideState();
      expect(guide.jumpTo(s, -1).step).toBe(0);
      expect(guide.jumpTo(s, 9).step).toBe(0);
    });
  });

  describe('③ 换品牌必须让旧选择失效（票面判据：不残留上次预选）', () => {
    it('换品牌清掉第二层与现象', () => {
      let s = guide.initialGuideState();
      s = guide.selectBrand(s, 'zhongli');
      s = guide.applySecond(s, 'battery', '电池系统', false);
      s = guide.selectFault(s, '举升无力');
      s = guide.setExtra(s, 'E01');
      const switched = guide.selectBrand(s, 'linde');
      expect(switched.secondId).toBe('');
      expect(switched.secondName).toBe('');
      expect(switched.selection.faultName).toBe('');
      expect(switched.step).toBe(guide.GUIDE_STEP_SECOND);
    });

    it('换第二层清掉现象（拿 A 系统的现象去查 B 系统的词表是错的）', () => {
      let s = guide.initialGuideState();
      s = guide.selectBrand(s, 'zhongli');
      s = guide.applySecond(s, 'battery', '电池系统', true);
      s = guide.selectFault(s, '举升无力');
      const other = guide.applySecond(s, 'brake', '制动系统', false);
      expect(other.selection.faultName).toBe('');
      expect(other.step).toBe(guide.GUIDE_STEP_FAULT);
    });

    it('重新进入 = 全新状态（关闭面板再进来不残留）', () => {
      let s = guide.initialGuideState();
      s = guide.selectBrand(s, 'zhongli');
      s = guide.selectFault(s, '举升无力');
      s = guide.setExtra(s, 'E01');
      // 关闭面板 = 丢掉那个状态对象；再进 = 重新拿初始态（这是「不残留」的实现机制本身）
      const fresh = guide.initialGuideState();
      expect(fresh.step).toBe(0);
      expect(fresh.brandId).toBe('');
      expect(fresh.selection.faultName).toBe('');
      expect(fresh.selection.extra).toBe('');
    });
  });

  describe('④ 预选兑现（欢迎区点品牌卡 / 诉求卡进来）', () => {
    it('点品牌卡进来 ⇒ 直接落到第二步（品牌已定，不再问一遍）', () => {
      const s = guide.openWithBrand(guide.initialGuideState(), 'zhongli');
      expect(s.step).toBe(guide.GUIDE_STEP_SECOND);
      expect(s.brandId).toBe('zhongli');
      expect(s.selection.brandId).toBe('zhongli');
    });

    it('点诉求卡进来（品牌未定）⇒ 停在第零步选品牌，现象先寄存', () => {
      const s = guide.openWithFault(guide.initialGuideState(), '', '举升无力');
      expect(s.step).toBe(guide.GUIDE_STEP_BRAND);
      expect(s.pendingFault).toBe('举升无力');
      // 寄存期间**不**写进已选现象：品牌与系统都还没定，此时成句是不成立的
      expect(s.selection.faultName).toBe('');
    });

    it('预选现象在词表里 ⇒ 选完第二层直接进确认步（诉求卡的完整路径）', () => {
      let s = guide.openWithFault(guide.initialGuideState(), '', '举升无力');
      s = guide.selectBrand(s, 'zhongli');
      s = guide.applySecond(s, 'battery', '电池系统', true);
      expect(s.selection.faultName).toBe('举升无力');
      expect(s.pendingFault).toBe('');
      expect(s.step).toBe(guide.GUIDE_STEP_CONFIRM);
    });

    it('预选现象不在词表里 ⇒ 停在第三步让人自己选/自己写（不假装已选）', () => {
      let s = guide.openWithFault(guide.initialGuideState(), '', '完全没有的现象');
      s = guide.selectBrand(s, 'zhongli');
      s = guide.applySecond(s, 'battery', '电池系统', false);
      expect(s.selection.faultName).toBe('');
      expect(s.step).toBe(guide.GUIDE_STEP_FAULT);
    });

    it('自己点了一条现象 ⇒ 预选作废，换品牌时不再冒出来', () => {
      let s = guide.openWithFault(guide.initialGuideState(), '', '举升无力');
      s = guide.selectBrand(s, 'zhongli');
      s = guide.selectFault(s, '制动失灵');
      expect(s.pendingFault).toBe('');
      const switched = guide.selectBrand(s, 'linde');
      expect(switched.selection.faultName).toBe('');
      expect(switched.pendingFault).toBe('');
    });
  });

  describe('⑤ 第二层搜索（林德 87 个型号要能搜）', () => {
    it('空搜索词给全量；按名字小写包含过滤；空格被忽略', () => {
      const names = ['E-322', 'R14', '牵引车 T20', '拣选车'];
      expect(guide.filterSecondItems(names, '')).toEqual(names);
      expect(guide.filterSecondItems(names, '  ')).toEqual(names);
      expect(guide.filterSecondItems(names, 'e-3')).toEqual(['E-322']);
      expect(guide.filterSecondItems(names, '牵引')).toEqual(['牵引车 T20']);
      expect(guide.filterSecondItems(names, '不存在的')).toEqual([]);
    });
  });

  describe('⑥ 组句（真执行后断言产物，不读源码字面量）', () => {
    it('系统档：系统名进括号、现象进主句、按排查三段收尾', () => {
      const sel = { brandId: 'zhongli', secondId: 'battery', faultName: '举升无力', extra: '' };
      expect(compose.composeDiagnosisSentence(ZHONGLI, '电池系统', sel)).toBe(
        '我的中力叉车（电池系统）出现故障现象：举升无力。请给我排查步骤、根因分析和维修方案。'
      );
    });

    it('型号档：问法换成维修手册口径（检查步骤 / 拆装顺序 / 扭矩）', () => {
      const sel = { brandId: 'linde', secondId: 'e322', faultName: '制动系统', extra: '' };
      expect(compose.composeDiagnosisSentence(LINDE, 'E-322', sel)).toBe(
        '我的林德叉车，型号：E-322，请按该型号维修手册说明「制动系统」的检查确认步骤、拆装顺序、所需工具和扭矩要求。请按手册给出安全注意事项。'
      );
    });

    it('补充说明进正文（那是学员唯一能补回命中率的手段）', () => {
      const sel = { brandId: 'zhongli', secondId: 'battery', faultName: '举升无力', extra: ' E01，冷车时出现 ' };
      const s = compose.composeDiagnosisSentence(ZHONGLI, '电池系统', sel);
      expect(s).toContain('补充：E01，冷车时出现。');
    });

    it('第二层缺省时降级形态成立（不带括号 / 型号写「未选」）', () => {
      const sys = { brandId: 'zhongli', secondId: '', faultName: '举升无力', extra: '' };
      expect(compose.composeDiagnosisSentence(ZHONGLI, '', sys)).toBe(
        '我的中力叉车出现故障现象：举升无力。请给我排查步骤、根因分析和维修方案。'
      );
      const model = { brandId: 'linde', secondId: '', faultName: '制动系统', extra: '' };
      expect(compose.composeDiagnosisSentence(LINDE, '', model)).toContain('型号：（未选）');
    });

    it('现象为空 ⇒ 空串（「还没选完」不是错误，页面据此灰住发送）', () => {
      const sel = { brandId: 'zhongli', secondId: 'battery', faultName: '  ', extra: '' };
      expect(compose.composeDiagnosisSentence(ZHONGLI, '电池系统', sel)).toBe('');
      expect(compose.composeDiagnosisSentence(null, '电池系统', sel)).toBe('');
    });

    it('同一组输入恒得同一句（组句是纯函数，没有隐藏状态）', () => {
      const sel = { brandId: 'zhongli', secondId: 'battery', faultName: '举升无力', extra: 'E01' };
      const a = compose.composeDiagnosisSentence(ZHONGLI, '电池系统', sel);
      const b = compose.composeDiagnosisSentence(ZHONGLI, '电池系统', sel);
      expect(a).toBe(b);
    });
  });

  describe('⑦ 检测器有牙（注入变异的副本，证明上面判据真能判红）', () => {
    const fs = require('fs');

    it('把「换品牌清现象」删掉 ⇒ ③ 组第一类断言必须判红', () => {
      const src = readText(GUIDE_UTS);
      const broken = src.replace(
        'next.selection.faultName = \'\'\n\tnext.step = GUIDE_STEP_SECOND',
        'next.step = GUIDE_STEP_SECOND'
      );
      expect(broken).not.toBe(src); // 变异必须真落上
      const tmp = path.join(require('os').tmpdir(), 'diagnosisGuideMutated1.uts');
      fs.writeFileSync(tmp, broken);
      const mutated = loadGuide(tmp).guide;
      let s = mutated.initialGuideState();
      s = mutated.selectBrand(s, 'zhongli');
      s = mutated.applySecond(s, 'battery', '电池系统', false);
      s = mutated.selectFault(s, '举升无力');
      const switched = mutated.selectBrand(s, 'linde');
      expect(switched.selection.faultName).toBe('举升无力'); // 坏实现：残留
    });

    it('把型号档也改走 systems ⇒ ① 组判档断言必须判红', () => {
      const src = readText(GUIDE_UTS);
      const broken = src.replace(
        "return pickerKindOf(brand) == 'model' ? 'faults-root' : 'systems'",
        "return 'systems'"
      );
      expect(broken).not.toBe(src);
      const tmp = path.join(require('os').tmpdir(), 'diagnosisGuideMutated2.uts');
      fs.writeFileSync(tmp, broken);
      const mutated = loadGuide(tmp).guide;
      expect(mutated.secondSourceOf(LINDE)).toBe('systems'); // 坏实现
    });
  });
});