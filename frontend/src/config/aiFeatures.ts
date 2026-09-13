// 生成文件，勿手改（ADR-0030 前端 AI 功能配置窄域 codegen 试点，#613）。
// 唯一事实源：后端 AI 功能注册表 backend/internal/service/ai_feature_registry.go。
// 再生成：cd backend && go run ./cmd/gen-aifeatures
// 同步契约：backend/internal/service/ai_features_codegen_test.go 将本文件与注册表渲染结果
// 全等比对，手改或注册表变更未再生成时后端测试即红。功能键与展示名由注册表派生；
// 路由/文案/图标等展示数据在 aiFeatureUI.ts 手写维护，新增功能键时需同步补齐。
import type { Component } from 'vue'
import { aiFeatureUI } from './aiFeatureUI'

// 收录规则：注册表中管理端单绑定且声明计费的专项对话功能（与后端 featureChatKeys 同口径），
// 键序 = 注册表声明序（助手主页入口卡片顺序）。
export type AIFeatureKey =
  | 'maintenance_knowledge'
  | 'drawing_recognition'
  | 'exercise_solving'
  | 'fault_diagnosis'

// 注册表派生对：[功能键, 展示名, 是否限免, slug, 传输适配器]（展示名即后端 FeatureLabel）。
const AI_FEATURE_REGISTRY: ReadonlyArray<readonly [AIFeatureKey, string, boolean, string, AIFeatureAdapter]> = [
  ['maintenance_knowledge', '维保知识', false, 'maintenance', 'llm'],
  ['drawing_recognition', '图纸识别', false, 'drawing', 'llm'],
  ['exercise_solving', '习题解答', false, 'exercise', 'llm'],
  ['fault_diagnosis', '智能维修诊断', true, 'fault-diagnosis', 'diagnosis'],
]

/** 传输适配器（注册表派生，ADR-0047 §7）：专用 UI 判定读它，不再比较功能键字符串。 */
export type AIFeatureAdapter = 'llm' | 'diagnosis'

/** 专项功能路由片段白名单（注册表派生，供路由正则）：新增功能只改注册表 + 再生成。 */
export const AI_FEATURE_SLUG_PATTERN = 'maintenance|drawing|exercise|fault-diagnosis'

/** AI 助手双模式键（注册表派生，供设置页读写绑定，替代页面硬编码）。 */
export const AI_ASSISTANT_MODE_KEYS = [
  'ai_assistant_normal',
  'ai_assistant_expert'
] as const

export interface AIFeatureQuickOption {
  label: string
  options: string[]
}

export interface AIFeatureConfig {
  key: AIFeatureKey
  title: string
  routePath: string
  welcome: string
  /** AI 助手欢迎区入口卡片的一句话描述 */
  entryDesc: string
  icon: Component
  suggestions: string[]
  quickOptions?: AIFeatureQuickOption[]
  supportsImage?: boolean
  maxImages?: number
  /** 限免声明位（注册表派生）：true 时展示「限免」角标，前端据此提示不扣积分 */
  freePreview?: boolean
  /** 传输适配器（注册表派生）：专用 UI 按其分支，不再比较功能键字符串 */
  adapter?: AIFeatureAdapter
}

export const AI_FEATURES: AIFeatureConfig[] = AI_FEATURE_REGISTRY.map(
  ([key, title, freePreview, slug, adapter]) => ({
    ...aiFeatureUI[key],
    key,
    title,
    freePreview,
    adapter,
    // routePath 由注册表 slug 派生（不再在 aiFeatureUI 手写，避免与后端 slug 漂移）
    routePath: '/ai-assistant/' + slug
  })
)

/** 是否走外部诊断适配器（注册表派生）：专用 UI 判定的唯一入口。 */
export function isDiagnosisFeature(key: string | undefined | null): boolean {
  if (!key) return false
  const feature = AI_FEATURES.find(f => f.key === key)
  return feature?.adapter === 'diagnosis'
}

export function getAIFeatureByRoute(path: string): AIFeatureConfig | undefined {
  return AI_FEATURES.find(f => f.routePath === path)
}
