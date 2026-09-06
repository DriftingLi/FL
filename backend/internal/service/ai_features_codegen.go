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
}

// ExportAIFeatureRegistry 导出注册表快照：声明序、全行（含遗留兼容位），收录过滤由渲染方按规则做。
func ExportAIFeatureRegistry() []AIFeatureExport {
	out := make([]AIFeatureExport, 0, len(aiFeatureRegistry))
	for _, f := range aiFeatureRegistry {
		out = append(out, AIFeatureExport{
			Name: f.name, Label: f.label, BindingKind: string(f.bindingKind), Billed: f.billed,
		})
	}
	return out
}

// aiFrontendFeatureInclude 前端功能配置收录规则 = 管理端单绑定 ∧ 声明计费（deriveFeatureChatKeys
// 同一口径：学员可直接发起专项对话的功能）。路由/文案/图标等展示数据属前端域，在
// aiFeatureUI.ts 手写维护，不进生成面。
func aiFrontendFeatureInclude(f AIFeatureExport) bool {
	return f.BindingKind == string(bindingAdminSingle) && f.Billed
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

// 注册表派生对：[功能键, 展示名]（展示名即后端 FeatureLabel）。
const AI_FEATURE_REGISTRY: ReadonlyArray<readonly [AIFeatureKey, string]> = [
%s
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
`

// tsQuote TS 单引号字符串字面量转义（展示名含引号/反斜杠时保持生成物合法）。
func tsQuote(s string) string {
	s = strings.ReplaceAll(s, `\`, `\\`)
	return strings.ReplaceAll(s, "'", `\'`)
}

// RenderFrontendAIFeaturesTS 渲染前端功能配置文件内容（纯函数，确定性输出）。
// 收录规则筛出 0 行时报错拒绝生成——前端专项对话清单为空属注册表事故，不应静默产出空配置。
func RenderFrontendAIFeaturesTS(rows []AIFeatureExport) (string, error) {
	var union, pairs strings.Builder
	n := 0
	for _, f := range rows {
		if !aiFrontendFeatureInclude(f) {
			continue
		}
		if !aiFeatureKeyPattern.MatchString(f.Name) {
			return "", fmt.Errorf("功能键 %q 不符合 snake_case 约定，无法生成前端类型", f.Name)
		}
		n++
		fmt.Fprintf(&union, "  | '%s'\n", f.Name)
		fmt.Fprintf(&pairs, "  ['%s', '%s'],\n", f.Name, tsQuote(f.Label))
	}
	if n == 0 {
		return "", errors.New("注册表筛出 0 个前端专项对话功能（admin-single ∧ billed），拒绝生成空配置")
	}
	return fmt.Sprintf(aiFeaturesTSTemplate,
		strings.TrimSuffix(union.String(), "\n"),
		strings.TrimSuffix(pairs.String(), "\n")), nil
}

// GenerateFrontendAIFeaturesTS 渲染当前注册表（cmd/gen-aifeatures 与契约测试共用入口）。
func GenerateFrontendAIFeaturesTS() (string, error) {
	return RenderFrontendAIFeaturesTS(ExportAIFeatureRegistry())
}
