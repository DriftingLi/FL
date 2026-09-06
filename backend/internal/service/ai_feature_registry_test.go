// Package service 实现业务服务层。
// 本文件：AI 功能注册表的派生面行为等价测试（ADR-0030 决策 1/2）——
// 钉住 AllAIFeatures / FeatureLabel / featureChatKeys / featureSystemPrompt /
// isValidFeature 与注册表声明的逐一对应，以及 billed 声明与 CONTEXT.md
// 「AI 计费」免费清单的一致性。
package service

import (
	"reflect"
	"slices"
	"testing"
)

// TestAIFeatureRegistry_DerivedSurfaces 派生面行为等价：注册表改造前的
// 功能集/标签/会话键/提示词字面量全部钉死，防止派生引入漂移。
func TestAIFeatureRegistry_DerivedSurfaces(t *testing.T) {
	wantAll := []string{
		FeatureGradeShortAnswer,
		FeatureGenerateChapterContent,
		FeatureAIAssistantNormal,
		FeatureAIAssistantExpert,
		FeatureQuestionExplanation,
		FeatureFaultConsult,
		FeatureFaultCodeQuery,
		FeatureMaintenanceKnowledge,
		FeatureDrawingRecognition,
		FeatureExerciseSolving,
	}
	if !reflect.DeepEqual(AllAIFeatures, wantAll) {
		t.Fatalf("AllAIFeatures 派生不符（含展示顺序）:\n got=%v\nwant=%v", AllAIFeatures, wantAll)
	}

	wantLabel := map[string]string{
		FeatureGradeShortAnswer:       "简答题 AI 评分",
		FeatureGenerateChapterContent: "课程内容生成",
		FeatureAIAssistantNormal:      "AI 助手 · 普通模式",
		FeatureAIAssistantExpert:      "AI 助手 · 专家模式",
		FeatureQuestionExplanation:    "题目 AI 解析",
		FeatureFaultConsult:           "故障咨询",
		FeatureFaultCodeQuery:         "故障代码查询",
		FeatureMaintenanceKnowledge:   "维保知识",
		FeatureDrawingRecognition:     "图纸识别",
		FeatureExerciseSolving:        "习题解答",
		FeatureAIAssistant:            "AI 助手对话", // 遗留兼容
	}
	if !reflect.DeepEqual(FeatureLabel, wantLabel) {
		t.Fatalf("FeatureLabel 派生不符: got=%v want=%v", FeatureLabel, wantLabel)
	}

	wantChatKeys := map[string]bool{
		FeatureFaultConsult:         true,
		FeatureFaultCodeQuery:       true,
		FeatureMaintenanceKnowledge: true,
		FeatureDrawingRecognition:   true,
		FeatureExerciseSolving:      true,
	}
	if !reflect.DeepEqual(featureChatKeys, wantChatKeys) {
		t.Fatalf("featureChatKeys 派生不符: got=%v want=%v", featureChatKeys, wantChatKeys)
	}

	// 专项聊天功能：注册行提示词 = 原多臂 switch 各臂
	wantPrompt := map[string]string{
		FeatureFaultConsult:         faultConsultSystemPrompt,
		FeatureFaultCodeQuery:       faultCodeQuerySystemPrompt,
		FeatureMaintenanceKnowledge: maintenanceKnowledgeSystemPrompt,
		FeatureDrawingRecognition:   drawingRecognitionSystemPrompt,
		FeatureExerciseSolving:      exerciseSolvingSystemPrompt,
		// 阻塞消费功能（评分/章节/解析）的提示词收编进注册表后同面可得
		FeatureGradeShortAnswer:       gradingSystemPrompt,
		FeatureGenerateChapterContent: chapterContentSystemPrompt,
		FeatureQuestionExplanation:    questionExplainSystemPrompt,
	}
	for key, want := range wantPrompt {
		if got := featureSystemPrompt(key); got != want {
			t.Fatalf("featureSystemPrompt(%s) 不符", key)
		}
	}
	// 通用助手（双模式/遗留/空键/未注册）一律兜底通用专家提示词（原 switch 默认臂）
	for _, key := range []string{"", FeatureAIAssistant, FeatureAIAssistantNormal, FeatureAIAssistantExpert, "no_such_feature"} {
		if got := featureSystemPrompt(key); got != forkliftExpertSystemPrompt {
			t.Fatalf("featureSystemPrompt(%q) 应兜底通用提示词", key)
		}
	}

	// 有效性：注册表全集（含遗留 ai_assistant）有效；空键/未注册无效
	for _, f := range aiFeatureRegistry {
		if !isValidFeature(f.name) {
			t.Fatalf("isValidFeature(%s) 应为 true（注册行）", f.name)
		}
	}
	for _, key := range []string{"", "no_such_feature"} {
		if isValidFeature(key) {
			t.Fatalf("isValidFeature(%q) 应为 false", key)
		}
	}
}

// TestAIFeatureRegistry_BilledConsistency billed 声明位与 CONTEXT.md「AI 计费」口径一致：
// 仅助手对话计费（双模式/遗留兼容位/专项聊天 = featureChatKeys 派生集合）；
// 刷题解析、简答评分、章节生成等非对话消费免费。
func TestAIFeatureRegistry_BilledConsistency(t *testing.T) {
	// billed=false 集合钉死为 CONTEXT.md 免费清单（刷题 AI 解析、简答评分、章节内容生成）
	var billedFalse []string
	for _, f := range aiFeatureRegistry {
		if !f.billed {
			billedFalse = append(billedFalse, f.name)
		}
	}
	slices.Sort(billedFalse)
	wantFreeList := []string{FeatureQuestionExplanation, FeatureGenerateChapterContent, FeatureGradeShortAnswer}
	if !reflect.DeepEqual(billedFalse, wantFreeList) {
		t.Fatalf("billed=false 集合与免费清单不符: got=%v want=%v", billedFalse, wantFreeList)
	}

	// billed=true ⟺ 助手对话形态（双模式/遗留）或专项聊天（featureChatKeys 派生集合）
	for _, f := range aiFeatureRegistry {
		isAssistant := f.bindingKind == bindingAssistantMode || f.bindingKind == bindingAssistantLegacy
		isChat := featureChatKeys[f.name]
		if f.billed != (isAssistant || isChat) {
			t.Fatalf("%s billed=%v 与对话消费形态不符（assistant=%v chat=%v）", f.name, f.billed, isAssistant, isChat)
		}
	}
}

// TestAIFeatureRegistry_NewFeatureIsOneRow 验收演示：新增一个功能 = 一行注册，
// 四个派生面无需另改（以合成注册表验证派生函数，不污染全局）。
func TestAIFeatureRegistry_NewFeatureIsOneRow(t *testing.T) {
	// 对话型新功能：admin-single + billed → 全部派生面自动就位
	chatReg := append(slices.Clone(aiFeatureRegistry), aiFeature{
		name: "demo_chat", label: "演示对话", systemPrompt: "演示提示词",
		bindingKind: bindingAdminSingle, billed: true,
	})
	if got := deriveAllAIFeatures(chatReg); !slices.Contains(got, "demo_chat") || got[len(got)-1] != "demo_chat" {
		t.Fatalf("新功能应追加进功能全集末尾: %v", got)
	}
	if got := deriveFeatureLabel(chatReg)["demo_chat"]; got != "演示对话" {
		t.Fatalf("新功能标签未派生: %q", got)
	}
	if !deriveFeatureChatKeys(chatReg)["demo_chat"] {
		t.Fatal("billed 的 admin-single 新功能应进入 featureChatKeys")
	}

	// 阻塞型新功能：admin-single + 免费 → 进全集与标签，不进对话键
	blockReg := append(slices.Clone(aiFeatureRegistry), aiFeature{
		name: "demo_block", label: "演示评分", systemPrompt: "评分提示词",
		bindingKind: bindingAdminSingle, billed: false,
	})
	if !slices.Contains(deriveAllAIFeatures(blockReg), "demo_block") {
		t.Fatal("免费新功能也应进入功能全集")
	}
	if deriveFeatureChatKeys(blockReg)["demo_block"] {
		t.Fatal("免费（非对话消费）新功能不应进入 featureChatKeys")
	}
}

// TestAIFeatureRegistry_Hygiene 注册表卫生约束：键唯一、字段齐全、绑定形态合法、
// 遗留兼容位唯一且不进展示全集。
func TestAIFeatureRegistry_Hygiene(t *testing.T) {
	seen := map[string]bool{}
	legacyCount := 0
	for _, f := range aiFeatureRegistry {
		if f.name == "" {
			t.Fatal("注册行缺少功能键")
		}
		if seen[f.name] {
			t.Fatalf("功能键重复注册: %s", f.name)
		}
		seen[f.name] = true
		if f.label == "" {
			t.Fatalf("%s 缺少展示名", f.name)
		}
		if f.systemPrompt == "" {
			t.Fatalf("%s 缺少系统提示词（未声明的键走通用兜底，声明位必须显式）", f.name)
		}
		switch f.bindingKind {
		case bindingAdminSingle, bindingAssistantMode, bindingAssistantLegacy:
		default:
			t.Fatalf("%s 绑定形态非法: %s", f.name, f.bindingKind)
		}
		if f.bindingKind == bindingAssistantLegacy {
			legacyCount++
			if f.name != FeatureAIAssistant {
				t.Fatalf("遗留兼容位仅允许 ai_assistant，发现: %s", f.name)
			}
		}
	}
	if legacyCount != 1 {
		t.Fatalf("遗留兼容位应恰有 1 个（ai_assistant），got %d", legacyCount)
	}
	if len(AllAIFeatures) != len(aiFeatureRegistry)-1 {
		t.Fatalf("AllAIFeatures 应排除唯一的遗留兼容位: %d vs %d", len(AllAIFeatures), len(aiFeatureRegistry)-1)
	}
}
