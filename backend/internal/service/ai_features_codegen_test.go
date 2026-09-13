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
		{Name: "fault_consult", Label: "故障咨询", BindingKind: string(bindingAdminSingle), Billed: true, FreePreview: true, Slug: "fault-consult", Adapter: string(aiAdapterDiagnosis)},
		{Name: "ai_assistant_normal", Label: "AI 助手 · 普通模式", BindingKind: string(bindingAssistantMode), Billed: true},
		{Name: "maintenance_knowledge", Label: "维保知识", BindingKind: string(bindingAdminSingle), Billed: true, Slug: "maintenance", Adapter: string(aiAdapterLLM)},
		{Name: "ai_assistant", Label: "AI 助手对话", BindingKind: string(bindingAssistantLegacy), Billed: true},
		{Name: "exercise_solving", Label: "习题解答", BindingKind: string(bindingAdminSingle), Billed: true, Slug: "exercise", Adapter: string(aiAdapterLLM)},
	}
}

// TestRenderFrontendAIFeaturesSnapshot 生成器结构断言（ADR-0030 验收 1 / ADR-0047 §7）：
// 用注册表样例钉住收录过滤、声明序，以及 slug / adapter / 助手双模式键三个派生面。
//
// 为什么不再是整文件快照：整份文件形状已由 TestFrontendAIFeaturesTSInSync 对**真实**注册表做
// 字节级比对；对样例再钉一遍全文只会在每次模板调整时制造无信息的假红。这里改为断言可读的
// 结构性质——收录集合、声明序、派生面的存在与取值。
func TestRenderFrontendAIFeaturesSnapshot(t *testing.T) {
	got, err := RenderFrontendAIFeaturesTS(sampleRegistry())
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	// 收录过滤 + 声明序：只留 admin-single ∧ billed 三行，且保持注册表声明序
	wantUnion := "export type AIFeatureKey =\n  | 'fault_consult'\n  | 'maintenance_knowledge'\n  | 'exercise_solving'\n"
	if !strings.Contains(got, wantUnion) {
		t.Fatalf("功能键联合类型与声明序不符:\n%s", got)
	}
	// 被过滤的行不进联合类型（免费阻塞 / 双模式 / 遗留兼容）
	for _, absent := range []string{"  | 'grade_short_answer'", "  | 'ai_assistant_normal'", "  | 'ai_assistant'"} {
		if strings.Contains(got, absent) {
			t.Fatalf("被过滤的功能不应进功能键联合类型: %s", absent)
		}
	}
	// slug 与 adapter 进生成表（ADR-0047 §7）
	wantPair := "['fault_consult', '故障咨询', true, 'fault-consult', 'diagnosis'],"
	if !strings.Contains(got, wantPair) {
		t.Fatalf("派生对缺 slug/adapter，want 含 %s:\n%s", wantPair, got)
	}
	// 专项功能 slug 白名单（供前端路由正则）
	wantSlugs := "export const AI_FEATURE_SLUG_PATTERN = 'fault-consult|maintenance|exercise'"
	if !strings.Contains(got, wantSlugs) {
		t.Fatalf("slug 白名单未派生，want 含 %s:\n%s", wantSlugs, got)
	}
	// 助手双模式键：独立于专项对话收录规则（设置页读它，替代硬编码）
	if !strings.Contains(got, "export const AI_ASSISTANT_MODE_KEYS = [") || !strings.Contains(got, "  'ai_assistant_normal'") {
		t.Fatalf("助手双模式键未派生:\n%s", got)
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
