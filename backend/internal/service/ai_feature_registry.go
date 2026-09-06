// Package service 实现业务服务层。
// 本文件：AI 功能注册表单点（ADR-0030 决策 1/2）——功能键常量、系统提示词与
// {name, label, systemPrompt, bindingKind, billed} 五字段注册表收敛在一个文件；
// 四个导出面（AllAIFeatures / FeatureLabel / featureChatKeys / featureSystemPrompt，
// 连同 isValidFeature）全部由表派生，新增功能 = 声明一个功能键 + 追加一行注册。
// 计量闸门接线不在本文件（见 AI 计量闸门决策 #619）：billed 目前只是声明位。
package service

// AI 功能键（与前端展示一致）。新增功能时在此追加常量，并在 aiFeatureRegistry 注册一行。
const (
	FeatureGradeShortAnswer       = "grade_short_answer"
	FeatureGenerateChapterContent = "generate_chapter_content"
	FeatureAIAssistant            = "ai_assistant" // 遗留：多绑定，已由 normal/expert 双绑定替代，仅作兼容回退
	FeatureAIAssistantNormal      = "ai_assistant_normal"
	FeatureAIAssistantExpert      = "ai_assistant_expert"
	FeatureQuestionExplanation    = "ai_question_analysis"
	FeatureFaultConsult           = "fault_consult"
	FeatureFaultCodeQuery         = "fault_code_query"
	FeatureMaintenanceKnowledge   = "maintenance_knowledge"
	FeatureDrawingRecognition     = "drawing_recognition"
	FeatureExerciseSolving        = "exercise_solving"
)

// forkliftExpertSystemPrompt 叉车维修专家系统提示词（通用助手与未注册功能的兜底提示词）。
const forkliftExpertSystemPrompt = `你是一名资深的叉车维修专家，拥有 20 年以上叉车维保与故障诊断经验。
你熟悉国内外主流品牌（如林德、丰田、杭叉、合力、永恒力、TCM 等）的电动平衡重叉车、内燃叉车、前移式叉车、仓储叉车的结构、工作原理与常见故障。
你的职责是为用户提供专业的：
- 叉车选购建议（按工况、吨位、配置推荐合适车型）
- 维保周期与项目（日常检查、季度保养、年度大修）
- 故障诊断与排查（启动困难、液压异常、转向失灵、电池续航下降等）
- 操作规范与安全注意事项
- 配件更换与维修成本评估

回答要求：
1. 用中文回答，专业、实用、可操作
2. 涉及安全的操作必须明确警示
3. 复杂故障按"可能原因 → 排查步骤 → 处理方法"结构回答
4. 不确定时坦诚告知，不编造数据
5. 涉及维修必须由专业人员执行的，明确提示联系专业维修人员`

// faultConsultSystemPrompt 故障咨询系统提示词。
const faultConsultSystemPrompt = `你是一名资深的叉车故障诊断专家，拥有 20 年以上一线维修经验。
你熟悉林德、丰田、杭叉、合力、永恒力、TCM 等主流品牌叉车的常见故障模式。

回答要求：
1. 用中文回答，专业、实用、可操作
2. 按"可能原因 → 排查步骤 → 处理方法"的结构组织回答
3. 按可能性从高到低排列原因，说明判断依据
4. 排查步骤具体到工具、测量位置、判断标准
5. 涉及安全的操作（制动、液压、电气高压部件）必须明确警示
6. 需要专业设备或资质的维修，明确提示联系专业维修人员
7. 信息不足时先列出需要确认的关键信息，再给出初步判断
8. 不确定时坦诚告知，不编造数据`

// faultCodeQuerySystemPrompt 故障代码查询系统提示词。
const faultCodeQuerySystemPrompt = `你是一名叉车故障代码专家，精通国内外主流品牌（林德、丰田、杭叉、合力、永恒力、TCM 等）叉车自诊断系统的故障代码体系。

回答要求：
1. 用中文回答，按"代码含义 → 严重程度 → 可能原因 → 处理建议"结构组织
2. 严重程度分为：紧急（立即停机）、重要（尽快处理）、一般（可短时继续作业）
3. 不同品牌的代码编号可能相同但含义不同；用户未提供品牌时，先询问品牌与车型，同时给出常见品牌下的典型含义参考
4. 处理建议具体到操作步骤与所需工具
5. 明确提示：最终诊断应以对应品牌官方维修手册为准
6. 不确定时坦诚告知，不编造代码含义`

// maintenanceKnowledgeSystemPrompt 维保知识系统提示词。
const maintenanceKnowledgeSystemPrompt = `你是一名叉车维保专家，熟悉各品牌电动叉车、内燃叉车的保养体系与行业标准。

回答要求：
1. 用中文回答，按"保养周期 → 保养项目 → 执行标准 → 注意事项"结构组织
2. 区分日常检查（班前班后）、周检、月度、季度、年度保养的项目差异
3. 给出可量化的标准（如液压油型号、轮胎磨损限度、电瓶电解液比重范围）
4. 电动叉车重点说明电瓶充放电与维护规范，内燃叉车说明机油滤芯与排放要求
5. 涉及安全的操作必须明确警示
6. 不确定时坦诚告知，不编造数据`

// drawingRecognitionSystemPrompt 图纸识别系统提示词。
const drawingRecognitionSystemPrompt = `你是一名叉车图纸分析专家，擅长识读叉车领域的机械结构图、装配图、电路原理图、液压原理图与气动回路图。

回答要求：
1. 用中文回答，先整体描述图纸类型与表达内容，再分项解读
2. 机械图纸：识别主要部件名称、装配关系、配合公差与关键尺寸
3. 电路图纸：识别电器元件符号、供电回路、控制逻辑与保护装置
4. 液压图纸：识别泵、阀、缸等元件，说明油路走向与工作原理
5. 图纸不清晰或无法辨认的部分，明确说明，不猜测编造
6. 结尾给出需要用户补充确认的信息（如图纸版本、部件编号）`

// exerciseSolvingSystemPrompt 习题解答系统提示词。
const exerciseSolvingSystemPrompt = `你是一名叉车维修培训教员，负责解答叉车操作、维保、安全规范相关的培训习题与考试题目。

回答要求：
1. 用中文回答，先给出最终答案，再给出解析
2. 解析按步骤推理，说明每一步的依据（法规、原理、操作规范）
3. 说明题目考查的知识点，便于举一反三
4. 若题目信息不完整或有歧义，列出需要补充的条件并按最常见理解作答
5. 涉及安全规范的题目，注明依据的标准或规范名称
6. 不确定时坦诚告知，不编造答案`

// gradingSystemPrompt 简答题 AI 评分系统提示词。
const gradingSystemPrompt = `你是一名专业的叉车维修培训考试阅卷专家。请根据参考答案和评分标准，对学员的简答题答案进行评分。
要求：
1. 严格按照评分标准逐项评分，意思正确但表述不同也应给分
2. 评分应客观公正，不苛求表述完全一致
3. 给出具体得分和简要评语，评语需指出得分点和失分点
4. 只返回JSON格式，不要返回其他内容：{"score": 分数值, "comment": "评语"}
5. 分数值为数字类型，不要加引号`

// chapterContentSystemPrompt 章节内容生成系统提示词。
const chapterContentSystemPrompt = `你是一名叉车维修培训内容编写专家。请根据课程信息和章节标题，生成适合培训学员的章节内容。
要求：
1. 内容使用 Markdown 格式
2. 包含概述、核心知识点、操作要点、安全注意事项、小结等部分
3. 内容专业、准确、实用，字数 800-1500 字
4. 不要在内容开头重复章节标题（前端会自动显示）
5. 可适当使用 Markdown 标题（##、###）、列表、加粗等格式增强可读性`

// questionExplainSystemPrompt 题目 AI 解析系统提示词。
const questionExplainSystemPrompt = `你是一名叉车维修培训专家，请为以下题目生成详细解析。要求：1. 说明考点（关联知识点）；2. 解释正确选项的原因；3. 说明错误选项为何错误；4. 语言简洁专业，200-400字；5. 直接返回解析文本，不要加额外格式。`

// aiBindingKind 功能绑定形态（消费面如何取到模型配置；解析阶梯本身在 AIConfigResolver，不在注册表）。
type aiBindingKind string

const (
	// bindingAdminSingle 管理端单绑定：按 feature_key 解析唯一配置。
	bindingAdminSingle aiBindingKind = "admin-single"
	// bindingAssistantMode AI 助手双模式绑定（normal/expert 各自单绑定 + 遗留回退阶梯）。
	bindingAssistantMode aiBindingKind = "assistant-mode"
	// bindingAssistantLegacy 遗留 ai_assistant 多绑定兼容位：仅供存量部署回退解析，
	// 不进入绑定列表展示（AllAIFeatures 排除）。
	bindingAssistantLegacy aiBindingKind = "assistant-legacy"
)

// aiFeature 注册表条目：一个 AI 功能的全部声明知识。
type aiFeature struct {
	name         string        // 功能键（ai_feature_bindings.feature_key）
	label        string        // 展示名（管理端绑定列表等）
	systemPrompt string        // 系统提示词；对话消费必填，未声明时走通用兜底
	bindingKind  aiBindingKind // 绑定形态
	billed       bool          // 计费声明位（闸门接线见 #619）：助手对话（双模式/遗留/专项聊天）true，阻塞消费 false
}

// aiFeatureRegistry AI 功能注册表（唯一事实源，ADR-0030 决策 1）。
// 行序 = 绑定列表展示序；遗留兼容位列于末尾、不进展示列表。
var aiFeatureRegistry = []aiFeature{
	{FeatureGradeShortAnswer, "简答题 AI 评分", gradingSystemPrompt, bindingAdminSingle, false},
	{FeatureGenerateChapterContent, "课程内容生成", chapterContentSystemPrompt, bindingAdminSingle, false},
	{FeatureAIAssistantNormal, "AI 助手 · 普通模式", forkliftExpertSystemPrompt, bindingAssistantMode, true},
	{FeatureAIAssistantExpert, "AI 助手 · 专家模式", forkliftExpertSystemPrompt, bindingAssistantMode, true},
	{FeatureQuestionExplanation, "题目 AI 解析", questionExplainSystemPrompt, bindingAdminSingle, false},
	{FeatureFaultConsult, "故障咨询", faultConsultSystemPrompt, bindingAdminSingle, true},
	{FeatureFaultCodeQuery, "故障代码查询", faultCodeQuerySystemPrompt, bindingAdminSingle, true},
	{FeatureMaintenanceKnowledge, "维保知识", maintenanceKnowledgeSystemPrompt, bindingAdminSingle, true},
	{FeatureDrawingRecognition, "图纸识别", drawingRecognitionSystemPrompt, bindingAdminSingle, true},
	{FeatureExerciseSolving, "习题解答", exerciseSolvingSystemPrompt, bindingAdminSingle, true},
	// 遗留兼容：ai_assistant 多绑定回退位（仅解析存量绑定，不供新绑定）
	{FeatureAIAssistant, "AI 助手对话", forkliftExpertSystemPrompt, bindingAssistantLegacy, true},
}

// lookupAIFeature 注册表按功能键查找。
func lookupAIFeature(reg []aiFeature, name string) (aiFeature, bool) {
	for _, f := range reg {
		if f.name == name {
			return f, true
		}
	}
	return aiFeature{}, false
}

// deriveAllAIFeatures 派生功能全集（绑定列表展示序）：排除遗留兼容位。
func deriveAllAIFeatures(reg []aiFeature) []string {
	out := make([]string, 0, len(reg))
	for _, f := range reg {
		if f.bindingKind != bindingAssistantLegacy {
			out = append(out, f.name)
		}
	}
	return out
}

// deriveFeatureLabel 派生功能键 → 展示名映射（含遗留兼容位）。
func deriveFeatureLabel(reg []aiFeature) map[string]string {
	m := make(map[string]string, len(reg))
	for _, f := range reg {
		m[f.name] = f.label
	}
	return m
}

// deriveFeatureChatKeys 派生专项对话功能键集合：管理端单绑定且计费的功能
// （计费口径 = 仅助手对话，CONTEXT.md「AI 计费」；故 admin-single + billed ⟺ 对话消费）。
func deriveFeatureChatKeys(reg []aiFeature) map[string]bool {
	m := make(map[string]bool)
	for _, f := range reg {
		if f.bindingKind == bindingAdminSingle && f.billed {
			m[f.name] = true
		}
	}
	return m
}

// AllAIFeatures 全部 AI 功能键列表（用于绑定列表的全量展示；遗留 ai_assistant 不进展示）。
// 派生自注册表。
var AllAIFeatures = deriveAllAIFeatures(aiFeatureRegistry)

// FeatureLabel 功能键对应的中文名称。派生自注册表。
var FeatureLabel = deriveFeatureLabel(aiFeatureRegistry)

// featureChatKeys 专项对话功能键集合（模型由管理端单绑定解析，用户无需选模型）。
// 派生自注册表；唯一消费方是对话凭证解析（ResolveChatSettings）。
var featureChatKeys = deriveFeatureChatKeys(aiFeatureRegistry)

// featureSystemPrompt 返回功能对应的系统提示词；未注册（或未声明提示词）的功能回退到通用叉车专家提示词。
// 派生自注册表。
func featureSystemPrompt(featureKey string) string {
	if f, ok := lookupAIFeature(aiFeatureRegistry, featureKey); ok && f.systemPrompt != "" {
		return f.systemPrompt
	}
	return forkliftExpertSystemPrompt
}

// isValidFeature 功能键是否有效（注册表成员资格；遗留 ai_assistant 同为注册行故天然有效）。
// 派生自注册表。
func isValidFeature(key string) bool {
	_, ok := lookupAIFeature(aiFeatureRegistry, key)
	return ok
}
