import type { IconName } from '@/icons/registry'
// AI 专项功能前端展示数据（手写维护）：路由、欢迎语、入口描述、图标、预设提示词、
// 快捷选项与图片能力。功能键全集由后端注册表生成于 aiFeatures.ts，本文件以
// Record<AIFeatureKey, …> 锁定覆盖——后端新增功能并再生成后，vue-tsc 在此报缺键提示补齐。
// 功能清单事实不在本文件，勿在生成面之外增删功能键。
import type { AIFeatureKey, AIFeatureQuickOption } from './aiFeatures'

export interface AIFeatureUIDescriptor {
  /** AI 助手欢迎区入口卡片的一句话描述 */
  entryDesc: string
  icon: IconName
  suggestions: string[]
  quickOptions?: AIFeatureQuickOption[]
  supportsImage?: boolean
  maxImages?: number
}

export const aiFeatureUI: Record<AIFeatureKey, AIFeatureUIDescriptor> = {
  maintenance_knowledge: {
    entryDesc: '保养周期、项目与标准',
    icon: 'book-open',
    suggestions: [
      '叉车季度保养项目有哪些？',
      '日常检查清单是什么？',
      '液压油多久更换一次？',
      '电瓶日常维护注意事项？'
    ],
    quickOptions: [
      { label: '保养类型', options: ['日常', '周检', '月度', '季度', '年度'] }
    ]
  },
  drawing_recognition: {
    entryDesc: '上传图纸，识别部件与原理',
    icon: 'image',
    suggestions: [
      '帮我识别这张图纸中的叉车部件',
      '解释这张电路图的工作原理',
      '这张液压原理图的油路走向？'
    ],
    quickOptions: [
      { label: '识别模式', options: ['部件识别', '参数解读', '电路分析', '液压分析'] }
    ],
    supportsImage: true,
    maxImages: 4
  },
  exercise_solving: {
    entryDesc: '上传习题照片，给出答案与解析',
    icon: 'square-pen',
    suggestions: [
      '请解答图片中的习题',
      '这道题的考点是什么？'
    ],
    quickOptions: [
      { label: '解答模式', options: ['详细步骤', '仅答案', '考点分析'] }
    ],
    supportsImage: true,
    maxImages: 4
  },
  fault_diagnosis: {
    entryDesc: '基于维修手册生成排查 SOP',
    icon: 'briefcase-medical',
    // 预设提示词**空**（票 2305）：诊断页的欢迎区已经有品牌卡 + 六张诉求卡 + 上游 FAQ 三件套，
    // 而这 4 条静态句子里三条与它们同义（「仪表报警」「举升慢」「故障码含义」分别撞诉求卡与 FAQ），
    // 并排两份既重复又让欢迎区更高。对话页因此不渲染翻页气泡；其余功能页的提示词不受影响。
    suggestions: [],
    // 无静态 quickOptions：品牌/系统/故障现象三级点选由 DiagnosisGuide 走
    // GET /diagnosis/{intents,systems,faults,code-hint}（票 2242 的数据面，票 2244 的界面）。
    supportsImage: true,
    // 3 张：与外部诊断助手一致（其前端 MAX_IMGS=3，后端 adapter 的 diagnosisMaxImages 也是 3）。
    // 超过 3 张后端会截断并在正文尾追加提示，前端与后端同值才不至于让用户白传。
    maxImages: 3
  }
}
