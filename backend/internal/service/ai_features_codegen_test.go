// Package service 前端 AI 功能配置生成器测试（ADR-0030 决策 3，#613）：渲染纯函数的
// 快照断言（注册表样例 → 输出快照）与生成物一致性契约——真实注册表的渲染结果与
// frontend/src/config/aiFeatures.ts 字节级全等，生成物过期或注册表变更未再生成即红
// （无需改 CI workflow）；另钉住空收录集拒绝与功能键形态防御。
package service

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// sampleRegistry 注册表样例（非真实注册表）：覆盖收录规则四态——
// admin-single∧billed 进生成面（声明序保留，freePreview 位透出）；免费阻塞行、双模式行、遗留兼容位被过滤。
func sampleRegistry() []AIFeatureExport {
	return []AIFeatureExport{
		{Name: "grade_short_answer", Label: "简答题 AI 评分", BindingKind: string(bindingAdminSingle), Billed: false},
		{Name: "fault_consult", Label: "故障咨询", BindingKind: string(bindingAdminSingle), Billed: true, FreePreview: true},
		{Name: "ai_assistant_normal", Label: "AI 助手 · 普通模式", BindingKind: string(bindingAssistantMode), Billed: true},
		{Name: "maintenance_knowledge", Label: "维保知识", BindingKind: string(bindingAdminSingle), Billed: true},
		{Name: "ai_assistant", Label: "AI 助手对话", BindingKind: string(bindingAssistantLegacy), Billed: true},
		{Name: "exercise_solving", Label: "习题解答", BindingKind: string(bindingAdminSingle), Billed: true},
	}
}

// TestRenderFrontendAIFeaturesSnapshot 生成器单测（ADR-0030 验收 1）：注册表样例 → 输出快照
// （快照钉死收录过滤、声明序、键/名转义与整份文件形状）。
func TestRenderFrontendAIFeaturesSnapshot(t *testing.T) {
	got, err := RenderFrontendAIFeaturesTS(sampleRegistry())
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	want := `// 生成文件，勿手改（ADR-0030 前端 AI 功能配置窄域 codegen 试点，#613）。
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
  | 'maintenance_knowledge'
  | 'exercise_solving'

// 注册表派生对：[功能键, 展示名, 是否限免]（展示名即后端 FeatureLabel；限免位供前端角标）。
const AI_FEATURE_REGISTRY: ReadonlyArray<readonly [AIFeatureKey, string, boolean]> = [
  ['fault_consult', '故障咨询', true],
  ['maintenance_knowledge', '维保知识', false],
  ['exercise_solving', '习题解答', false],
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
  /** 限免声明位（注册表派生）：true 时展示「限免」角标，前端据此提示不扣积分 */
  freePreview?: boolean
}

export const AI_FEATURES: AIFeatureConfig[] = AI_FEATURE_REGISTRY.map(([key, title, freePreview]) => ({
  key,
  title,
  freePreview,
  ...aiFeatureUI[key]
}))

export function getAIFeatureByRoute(path: string): AIFeatureConfig | undefined {
  return AI_FEATURES.find(f => f.routePath === path)
}
`
	if got != want {
		t.Fatalf("渲染结果与快照不符:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// TestRenderFrontendAIFeaturesEmptyRejected 收录集为空（注册表事故）拒绝生成空配置。
func TestRenderFrontendAIFeaturesEmptyRejected(t *testing.T) {
	if _, err := RenderFrontendAIFeaturesTS(nil); err == nil {
		t.Fatal("空收录集应拒绝生成")
	}
	// 全部被过滤（免费/双模式/遗留）同样拒绝
	free := []AIFeatureExport{
		{Name: "grade_short_answer", Label: "简答题 AI 评分", BindingKind: string(bindingAdminSingle), Billed: false},
		{Name: "ai_assistant", Label: "AI 助手对话", BindingKind: string(bindingAssistantLegacy), Billed: true},
	}
	if _, err := RenderFrontendAIFeaturesTS(free); err == nil {
		t.Fatal("全被过滤的注册表应拒绝生成")
	}
}

// TestRenderFrontendAIFeaturesKeyShape 功能键形态防御：非法键使渲染报错，不产出坏 TS。
func TestRenderFrontendAIFeaturesKeyShape(t *testing.T) {
	bad := []AIFeatureExport{{Name: "Bad-Key", Label: "x", BindingKind: string(bindingAdminSingle), Billed: true}}
	if _, err := RenderFrontendAIFeaturesTS(bad); err == nil {
		t.Fatal("非 snake_case 功能键应报错")
	}
}

// TestFrontendIncludeMatchesChatKeys 收录谓词与 featureChatKeys 全表互等（消除双写漂移面）：
// 收录规则唯一编码于 aiFeatureIsChat，本断言钉住组合关系——前端收录行集合恒等于
// featureChatKeys 键集（任一侧被改动偏离即红）。
func TestFrontendIncludeMatchesChatKeys(t *testing.T) {
	var included []string
	for _, f := range ExportAIFeatureRegistry() {
		if aiFrontendFeatureInclude(f) {
			included = append(included, f.Name)
		}
	}
	if len(included) != len(featureChatKeys) {
		t.Fatalf("前端收录集合与 featureChatKeys 大小不等: included=%v chatKeys=%v", included, featureChatKeys)
	}
	for _, k := range included {
		if !featureChatKeys[k] {
			t.Fatalf("收录行 %q 不在 featureChatKeys 中", k)
		}
	}
	for k := range featureChatKeys {
		f, ok := lookupAIFeature(aiFeatureRegistry, k)
		if !ok {
			t.Fatalf("featureChatKeys 键 %q 不在注册表中", k)
		}
		if !aiFrontendFeatureInclude(AIFeatureExport{Name: f.name, BindingKind: string(f.bindingKind), Billed: f.billed}) {
			t.Fatalf("featureChatKeys 键 %q 未被收录谓词命中", k)
		}
	}
}

// TestFrontendAIFeaturesTSInSync 生成物一致性契约（ADR-0030 验收 2，#613）：真实注册表渲染
// 结果与 frontend/src/config/aiFeatures.ts 全等。手改生成物或改注册表未再生成时本测试红，
// 提示重新生成（go run ./cmd/gen-aifeatures）。行尾归一仅防 Windows 检出差异假红
// （.gitattributes 已强制 *.ts LF），内容以 LF 语义比对。
func TestFrontendAIFeaturesTSInSync(t *testing.T) {
	want, err := GenerateFrontendAIFeaturesTS()
	if err != nil {
		t.Fatalf("渲染真实注册表失败: %v", err)
	}
	path := filepath.Join("..", "..", "..", "frontend", "src", "config", "aiFeatures.ts")
	raw, err := os.ReadFile(filepath.FromSlash(path))
	if err != nil {
		t.Fatalf("读取前端生成物失败（%s）: %v", path, err)
	}
	got := strings.ReplaceAll(string(raw), "\r\n", "\n")
	if got != want {
		t.Fatalf("frontend/src/config/aiFeatures.ts 与注册表渲染结果不一致（生成物过期，请运行 cd backend && go run ./cmd/gen-aifeatures）:\n--- 渲染结果 ---\n%s\n--- 仓库文件 ---\n%s", want, got)
	}
}
