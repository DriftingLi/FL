// Package service 前端 AI 功能配置窄域生成器（ADR-0030 决策 3，#613）：把功能注册表的
// 派生面（专项对话功能键与展示名）渲染为 frontend/src/config/aiFeatures.ts（生成勿改）。
// 渲染是纯函数（注册表快照 → 输出字符串）：输出确定性（无时间戳、无随机序）是契约测试
// 「字节级全等比对」的前提，生成物过期由 backend/internal/service/ai_features_codegen_test.go
// 直接变红暴露，无需改 CI workflow；再生成入口在 cmd/gen-aifeatures。
package service

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// AIFeatureExport 注册表 → 生成管道的结构化导出行（ADR-0030「读后端注册表的结构化导出」）。
type AIFeatureExport struct {
	Name        string // 功能键（ai_feature_bindings.feature_key）
	Label       string // 展示名（与后端 FeatureLabel 同源）
	BindingKind string // 绑定形态（string(aiBindingKind)）
	Billed      bool   // 计费声明位
	FreePreview bool   // 限免声明位（透出供前端限免角标）
	Slug        string // 路由片段（ADR-0047 §7；空串 = 无独立页面）
	Adapter     string // 传输适配器（string(aiAdapter)）
}

// ExportAIFeatureRegistry 导出注册表快照：声明序、全行（含遗留兼容位），收录过滤由渲染方按规则做。
func ExportAIFeatureRegistry() []AIFeatureExport {
	out := make([]AIFeatureExport, 0, len(aiFeatureRegistry))
	for _, f := range aiFeatureRegistry {
		out = append(out, AIFeatureExport{
			Name: f.name, Label: f.label, BindingKind: string(f.bindingKind), Billed: f.billed, FreePreview: f.freePreview,
			Slug: f.slug, Adapter: string(f.adapter),
		})
	}
	return out
}

// aiFrontendFeatureInclude 前端功能配置收录规则 = aiFeatureIsChat（管理端单绑定 ∧ 声明计费，
// 规则唯一编码于 ai_feature_registry.go，与 deriveFeatureChatKeys 同一谓词；组合关系由
// ai_features_codegen_test.go 全表互等断言钉住）：学员可直接发起专项对话的功能。
// 路由/文案/图标等展示数据属前端域，在 aiFeatureUI.ts 手写维护，不进生成面。
func aiFrontendFeatureInclude(f AIFeatureExport) bool {
	return aiFeatureIsChat(aiBindingKind(f.BindingKind), f.Billed)
}

// aiFeatureKeyPattern 功能键合法形态（snake_case）：生成 TS 联合类型与对象键的前提。
var aiFeatureKeyPattern = regexp.MustCompile(`^[a-z][a-z0-9_]*$`)

// aiFeaturesTSTemplate 生成物模板（第一个 %s = 功能键联合类型成员，第二个 %s = 注册表派生对）。
// 无生成时间戳：时间戳使再生成永不幂等，全等契约随之失效——过期改由契约测试暴露。
const aiFeaturesTSTemplate = `// 生成文件，勿手改（ADR-0030 前端 AI 功能配置窄域 codegen 试点，#613）。
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
%s

// 注册表派生对：[功能键, 展示名, 是否限免, slug, 传输适配器]（展示名即后端 FeatureLabel）。
const AI_FEATURE_REGISTRY: ReadonlyArray<readonly [AIFeatureKey, string, boolean, string, AIFeatureAdapter]> = [
%s
]

/** 传输适配器（注册表派生，ADR-0047 §7）：专用 UI 判定读它，不再比较功能键字符串。 */
export type AIFeatureAdapter = 'llm' | 'diagnosis'

/** 专项功能路由片段白名单（注册表派生，供路由正则）：新增功能只改注册表 + 再生成。 */
export const AI_FEATURE_SLUG_PATTERN = '%s'

/** AI 助手双模式键（注册表派生，供设置页读写绑定，替代页面硬编码）。 */
export const AI_ASSISTANT_MODE_KEYS = [
%s
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
`

// tsQuote TS 单引号字符串字面量转义（展示名含引号/反斜杠时保持生成物合法）。
func tsQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "'", `\'`)
}

// RenderFrontendAIFeaturesTS 渲染前端功能配置文件内容（纯函数，确定性输出）。
// 收录规则筛出 0 行时报错拒绝生成——前端专项对话清单为空属注册表事故，不应静默产出空配置。
func RenderFrontendAIFeaturesTS(rows []AIFeatureExport) (string, error) {
	var union, pairs, slugs, modes strings.Builder
	n := 0
	seenSlug := map[string]string{}
	for _, f := range rows {
		// 助手双模式键（供设置页读写绑定）：不受专项对话收录规则限制
		if f.BindingKind == string(bindingAssistantMode) {
			fmt.Fprintf(&modes, "  '%s',\n", f.Name)
		}
		if !aiFrontendFeatureInclude(f) {
			continue
		}
		if !aiFeatureKeyPattern.MatchString(f.Name) {
			return "", fmt.Errorf("功能键 %q 不符合 snake_case 约定，无法生成前端类型", f.Name)
		}
		if f.Slug == "" {
			return "", fmt.Errorf("专项对话功能 %q 缺少 slug：前端路由白名单无法派生（ADR-0047 §7）", f.Name)
		}
		if prev, dup := seenSlug[f.Slug]; dup {
			return "", fmt.Errorf("slug %q 重复：%s 与 %s", f.Slug, prev, f.Name)
		}
		seenSlug[f.Slug] = f.Name
		n++
		fmt.Fprintf(&union, "  | '%s'\n", f.Name)
		fmt.Fprintf(&pairs, "  ['%s', '%s', %v, '%s', '%s'],\n", f.Name, tsQuote(f.Label), f.FreePreview, f.Slug, f.Adapter)
		if slugs.Len() > 0 {
			slugs.WriteString("|")
		}
		slugs.WriteString(f.Slug)
	}
	if n == 0 {
		return "", errors.New("注册表筛出 0 个前端专项对话功能（admin-single ∧ billed），拒绝生成空配置")
	}
	if modes.Len() == 0 {
		return "", errors.New("注册表没有助手双模式键（assistant-mode），拒绝生成空配置")
	}
	return fmt.Sprintf(aiFeaturesTSTemplate,
		strings.TrimSuffix(union.String(), "\n"),
		strings.TrimSuffix(pairs.String(), "\n"),
		slugs.String(),
		strings.TrimSuffix(modes.String(), ",\n")), nil
}

// GenerateFrontendAIFeaturesTS 渲染当前注册表（cmd/gen-aifeatures 与契约测试共用入口）。
func GenerateFrontendAIFeaturesTS() (string, error) {
	return RenderFrontendAIFeaturesTS(ExportAIFeatureRegistry())
}
