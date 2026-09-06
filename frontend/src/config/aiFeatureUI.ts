// AI 专项功能前端展示数据（手写维护）：路由、欢迎语、入口描述、图标、预设提示词、
// 快捷选项与图片能力。功能键全集由后端注册表生成于 aiFeatures.ts，本文件以
// Record<AIFeatureKey, …> 锁定覆盖——后端新增功能并再生成后，vue-tsc 在此报缺键提示补齐。
// 功能清单事实不在本文件，勿在生成面之外增删功能键。
import type { Component } from 'vue'
import {
  Warning,
  Search,
  Reading,
  Picture,
  EditPen
} from '@element-plus/icons-vue'
import type { AIFeatureKey, AIFeatureQuickOption } from './aiFeatures'

export interface AIFeatureUIDescriptor {
  routePath: string
  welcome: string
  /** AI 助手欢迎区入口卡片的一句话描述 */
  entryDesc: string
  icon: Component
  suggestions: string[]
  quickOptions?: AIFeatureQuickOption[]
  supportsImage?: boolean
  maxImages?: number
}

export const aiFeatureUI: Record<AIFeatureKey, AIFeatureUIDescriptor> = {
  fault_consult: {
    routePath: '/ai-assistant/fault-consult',
    welcome: '描述您遇到的叉车故障现象，我将按「可能原因 → 排查步骤 → 处理方法」为您诊断。',
    entryDesc: '描述故障现象，按步骤排查',
    icon: Warning,
    suggestions: [
      '叉车启动困难怎么排查？',
      '液压升降缓慢的可能原因？',
      '转向沉重是什么问题？',
      '制动失灵如何应急处理？'
    ],
    quickOptions: [
      { label: '品牌', options: ['林德', '丰田', '杭叉', '合力', '永恒力', '其他'] },
      { label: '动力类型', options: ['电动', '内燃'] }
    ]
  },
  fault_code_query: {
    routePath: '/ai-assistant/fault-code',
    welcome: '输入叉车显示的故障代码，我将解读代码含义、严重程度与处理建议。不同品牌代码含义可能不同，建议同时选择品牌。',
    entryDesc: '解读代码含义与处理建议',
    icon: Search,
    suggestions: [
      '故障代码 E01 是什么意思？',
      'E24 代码怎么处理？',
      '报警灯闪烁 5 次代表什么？',
      '如何查询叉车故障码历史？'
    ],
    quickOptions: [
      { label: '品牌', options: ['林德', '丰田', '杭叉', '合力', '永恒力', '其他'] }
    ]
  },
  maintenance_knowledge: {
    routePath: '/ai-assistant/maintenance',
    welcome: '叉车维保专家为您解答保养周期、保养项目、执行标准与注意事项。',
    entryDesc: '保养周期、项目与标准',
    icon: Reading,
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
    routePath: '/ai-assistant/drawing',
    welcome: '上传叉车机械图纸、电路图或液压原理图，我将识别图中的部件、符号与参数并解读工作原理。',
    entryDesc: '上传图纸，识别部件与原理',
    icon: Picture,
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
    routePath: '/ai-assistant/exercise',
    welcome: '拍摄或上传叉车培训习题的照片，我将给出答案、解析与考查知识点。',
    entryDesc: '上传习题照片，给出答案与解析',
    icon: EditPen,
    suggestions: [
      '请解答图片中的习题',
      '这道题的考点是什么？'
    ],
    quickOptions: [
      { label: '解答模式', options: ['详细步骤', '仅答案', '考点分析'] }
    ],
    supportsImage: true,
    maxImages: 4
  }
}
