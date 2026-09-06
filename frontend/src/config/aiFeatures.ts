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
  | 'fault_consult'
  | 'fault_code_query'
  | 'maintenance_knowledge'
  | 'drawing_recognition'
  | 'exercise_solving'

// 注册表派生对：[功能键, 展示名]（展示名即后端 FeatureLabel）。
const AI_FEATURE_REGISTRY: ReadonlyArray<readonly [AIFeatureKey, string]> = [
  ['fault_consult', '故障咨询'],
  ['fault_code_query', '故障代码查询'],
  ['maintenance_knowledge', '维保知识'],
  ['drawing_recognition', '图纸识别'],
  ['exercise_solving', '习题解答'],
]

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
}

export const AI_FEATURES: AIFeatureConfig[] = AI_FEATURE_REGISTRY.map(([key, title]) => ({
  key,
  title,
  ...aiFeatureUI[key]
}))

export function getAIFeatureByRoute(path: string): AIFeatureConfig | undefined {
  return AI_FEATURES.find(f => f.routePath === path)
}
