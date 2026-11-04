// 诊断引导的点选结果 → 发出去的句子（票 2241 要素 11 的落地：从服务端 compose 搬到前端纯函数）。
//
// 为什么在前端：v2 契约七个接口里**没有 compose**（§二接口总览只有 chat / chat·complete /
// intents / systems / faults / code_hint / health），而 `content` 是唯一文本通道（§5.1
// 请求体只有 content + images）。品牌与型号没有结构化字段可走，只能进正文。
//
// **能力边界（2241 已拍板，不要假装等价）**：对方的 compose（`guide.py:882-945`）还会往句子
// 里注入语料特征词——`_linde_topic_term()` 从林德维修手册抽词、`_zhongli_topic_term()` 补
// 「，疑似 X 或 Y」。这两份词表来自对方的维修手册语料，我方拿不到。这里只拼**朴素句子**：
// 命中率会低于对方，验收时不许声称等价。补充说明（故障码 / 时机 / 异响位置）由用户自己写，
// 那是唯一能补回一部分命中率的手段。
//
// 句子骨架照抄对方的三种诉求收尾（`guide.py:940-941`）：林德手册讲「注意事项 / 步骤」，
// 中力故障单元讲「排查 / 根因 / 方案」。

import type { DiagnosisIntentBrand } from '@/api/aiAssistant'

/** 引导面板里的已选状态；任一层为空即未成句 */
export interface DiagnosisSelection {
  /** 品牌 id（`intents.brands[].id`） */
  brandId: string
  /** 第二层 id（`systems[].id`）：中力是系统、林德是型号 */
  secondId?: string
  /** 第三层：现象名（中力）或维修主题名（林德）——用**名字**而不是 id，名字才是要进正文的词 */
  faultName?: string
  /** 补充说明：故障码 / 出现时机 / 异响位置，可空 */
  extra?: string
}

/**
 * 把「品牌 → 第二层 → 现象 → 补充」拼成一句发给诊断服务的话。
 *
 * @param brand   `intents.brands[]` 里选中的那一项（要 `name` 与 `picker`）
 * @param secondName 第二层选中项的**名字**（系统名 / 型号名）
 * @param selection  已选状态（`faultName` / `extra` 用这里的）
 * @returns 拼好的句子；第三层（现象）为空时返回空串——那是「还没选完」，不是错误
 */
export function composeDiagnosisSentence(
  brand: DiagnosisIntentBrand | null | undefined,
  secondName: string,
  selection: DiagnosisSelection
): string {
  const fault = (selection.faultName || '').trim()
  if (!brand || !fault) return ''
  const extra = (selection.extra || '').trim()
  // 第二层名按 v2 契约是可选（`faults?brand=&second=` 的 second 可缺省），缺了就不写「型号：」
  const second = (secondName || '').trim()

  // 林德语料是维修手册不是故障码库：问法必须跟着换，否则检索不到东西（对方 guide.py:915-925 的理由）
  if (brand.picker === 'model') {
    let s = `我的${brand.name}叉车，型号：${second || '（未选）'}，请按该型号维修手册说明「${fault}」的检查确认步骤、拆装顺序、所需工具和扭矩要求。`
    if (extra) s += `补充：${extra}。`
    s += '请按手册给出安全注意事项。'
    return s
  }

  // 中力走故障码库：系统名进括号、现象进主句
  const head = second ? `我的${brand.name}叉车（${second}）` : `我的${brand.name}叉车`
  let s = `${head}出现故障现象：${fault}。`
  if (extra) s += `补充：${extra}。`
  s += '请给我排查步骤、根因分析和维修方案。'
  return s
}

/**
 * 第二层该渲染「系统卡」还是「型号卡」—— 由 v2 的 `intents.brands[].picker` 决定
 * （值 `system` / `model`）。契约没写的取值一律按系统解释：中方 14 个系统是默认形态。
 */
export function pickerKindOf(brand: DiagnosisIntentBrand | null | undefined): 'system' | 'model' {
  return brand?.picker === 'model' ? 'model' : 'system'
}

/** 第三层标题：林德选的是「维修部位 / 主题」，中力选的是「故障现象」（对方同口径） */
export function faultStepTitle(brand: DiagnosisIntentBrand | null | undefined): string {
  return pickerKindOf(brand) === 'model' ? '选要查的维修部位 / 主题' : '选故障现象'
}
