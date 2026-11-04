// diagnosisCompose 纯函数 spec（票 2244）：把「品牌 → 第二层 → 现象 → 补充」拼成一句话。
//
// 为什么值得单独锁：v2 的请求体只有 content 一个文本通道（契约 §5.1），品牌与型号
// **没有结构化字段可走**，只能进正文。这句话就是唯一的信息载体 —— 拼错了等于点选白点。
// 同时锁住两条已拍板的边界：
//   ① 我方拿不到对方的语料特征词表（`_linde_topic_term` / `_zhongli_topic_term`），
//      所以句子是朴素的，**不许**在验收时假装等价；
//   ② 林德是维修手册语料、中力是故障码库，两种问法的收尾要求本来就不同。
import { describe, it, expect } from 'vitest'
import {
  composeDiagnosisSentence,
  faultStepTitle,
  pickerKindOf,
  type DiagnosisSelection
} from '../diagnosisCompose'
import type { DiagnosisIntentBrand } from '@/api/aiAssistant'

function brand(over: Partial<DiagnosisIntentBrand> = {}): DiagnosisIntentBrand {
  return {
    id: 'zhongli',
    name: '中力 EP',
    tagline: '电动叉车 · 可由故障码输出维修方案',
    icon: '🔋',
    cls: 'brand-zhongli',
    picker: 'system',
    ...over
  }
}

const linde = brand({ id: 'linde', name: '林德 Linde', picker: 'model', cls: 'brand-linde' })

function sel(over: Partial<DiagnosisSelection> = {}): DiagnosisSelection {
  return { brandId: 'zhongli', secondId: 'hydraulic', faultName: '举升无力 / 不能起升', ...over }
}

describe('中力（picker=system）：系统名进括号、现象进主句', () => {
  it('组出含品牌名、系统名、现象与排查收尾的句子', () => {
    const s = composeDiagnosisSentence(brand(), '液压系统', sel())
    expect(s).toContain('中力 EP')
    expect(s).toContain('（液压系统）')
    expect(s).toContain('举升无力 / 不能起升')
    // 收尾诉求跟着语料走：中力单元讲「排查 / 根因 / 方案」
    expect(s).toContain('请给我排查步骤、根因分析和维修方案。')
  })

  it('补充说明拼成「补充：…。」', () => {
    const s = composeDiagnosisSentence(brand(), '液压系统', sel({ extra: 'E41，冷车时出现' }))
    expect(s).toContain('补充：E41，冷车时出现。')
  })

  it('空补充说明不产出空句子片段', () => {
    const s = composeDiagnosisSentence(brand(), '液压系统', sel({ extra: '   ' }))
    expect(s).not.toContain('补充：')
  })

  it('第二层缺失时不写括号（faults 的 second 本就可缺省）', () => {
    const s = composeDiagnosisSentence(brand(), '', sel())
    expect(s).toContain('我的中力 EP叉车出现故障现象：')
    expect(s).not.toContain('（）')
  })
})

describe('林德（picker=model）：按维修手册问法', () => {
  it('问的是手册条目（步骤 / 顺序 / 工具 / 扭矩）而不是「出现故障现象」', () => {
    const s = composeDiagnosisSentence(linde, 'E-322', sel({ faultName: '制动器拆装' }))
    expect(s).toContain('型号：E-322')
    expect(s).toContain('「制动器拆装」')
    expect(s).toContain('检查确认步骤、拆装顺序、所需工具和扭矩要求')
    // 收尾跟着手册走
    expect(s).toContain('请按手册给出安全注意事项。')
    // 关键否定断言：问法不能滑回中力那套
    expect(s).not.toContain('出现故障现象')
    expect(s).not.toContain('根因分析')
  })

  it('型号缺失时占位而不是静默省略（模型是林德必选项）', () => {
    const s = composeDiagnosisSentence(linde, '', sel({ faultName: '制动器拆装' }))
    expect(s).toContain('型号：（未选）')
  })
})

describe('未选完 / 非法入参', () => {
  it('现象未选时返回空串（那是「还没选完」，不是错误）', () => {
    expect(composeDiagnosisSentence(brand(), '液压系统', sel({ faultName: '' }))).toBe('')
    expect(composeDiagnosisSentence(brand(), '液压系统', sel({ faultName: '   ' }))).toBe('')
  })

  it('品牌缺失时返回空串', () => {
    expect(composeDiagnosisSentence(null, '液压系统', sel())).toBe('')
    expect(composeDiagnosisSentence(undefined, '液压系统', sel())).toBe('')
  })
})

describe('picker 分档', () => {
  it('picker=model 解释为型号档，其余（含未知值 / 缺省）一律按系统档', () => {
    expect(pickerKindOf(linde)).toBe('model')
    expect(pickerKindOf(brand())).toBe('system')
    expect(pickerKindOf(brand({ picker: 'some-new-value' }))).toBe('system')
    expect(pickerKindOf(null)).toBe('system')
  })

  it('第三层标题按 picker 分支（林德是维修部位 / 主题，中力是故障现象）', () => {
    expect(faultStepTitle(linde)).toBe('选要查的维修部位 / 主题')
    expect(faultStepTitle(brand())).toBe('选故障现象')
    expect(faultStepTitle(null)).toBe('选故障现象')
  })
})
