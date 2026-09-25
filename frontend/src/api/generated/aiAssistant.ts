// 生成文件，勿手改（ADR-0019 契约 codegen 专项 / ADR-0048 按域解冻；spec #940 片五③、#952 片一）。
// 域：AI 助手信封面（/api/ai-assistant/*：模型 / 会话 / 消息 / 用户模型 / 诊断字典）；SSE 流式对话 POST /ai-assistant/chat 不走统一信封、事件 payload 不定型，不在本域生成面（ADR-0048 决策 6）
// 唯一事实源：后端注解 → backend/docs/swagger.json（CI 有新鲜度锁：backend-lint 的 swagger 步骤）。
// 再生成：cd backend && go run ./cmd/gen-apitypes
// 同步契约：backend/internal/apitypes/codegen_test.go 把本文件与注解渲染结果全等比对。
//
// 覆盖端点：
//   GET  /ai-assistant/models
//   GET  /ai-assistant/modes
//   GET  /ai-assistant/user-models
//   POST /ai-assistant/user-models
//   DELETE /ai-assistant/user-models/{id}
//   GET  /ai-assistant/sessions
//   POST /ai-assistant/sessions
//   DELETE /ai-assistant/sessions/{id}
//   PATCH /ai-assistant/sessions/{id}/title
//   GET  /ai-assistant/sessions/{id}/messages
//   POST /ai-assistant/upload-image
//   GET  /ai-assistant/diagnosis/brands
//   GET  /ai-assistant/diagnosis/models
//   GET  /ai-assistant/diagnosis/fault-codes
//   GET  /ai-assistant/diagnosis/manual/{filepath}
//
// 覆盖的 Go 类型：AIAssistantModeModels / AIChatMessageDTO / AIChatSessionDTO / AIImageUploadResultDTO / AISessionRenameResultDTO / DiagnosisBrandOption / DiagnosisFaultCodeItem / DiagnosisFaultCodePage / DiagnosisSource / DiagnosisSourceMetadata / ModelOption / UserModelDTO
//
// 可空性 / 缺省态由**注解层**表达，生成器只如实转写（Go 结构体 tag）：
//   - extensions:"x-nullable" → 字段渲染 'T | null'：键一定在，值为 null（Go 指针且无 omitempty）；
//   - extensions:"x-optional" → 字段渲染 'T?'：键**可能整个不存在**（Go omitempty）；
//   - 两者可同时标注（'T?' 且 '| null'）；未标注的一律按「键一定在、非 null」渲染 ——
//     swag 看不到 Go 的 omitempty，漏标即契约撒谎。
// 其余已知限制：
//   - Go 侧 any 字段在 swagger 里是空 schema，渲染 'unknown'（不猜结构）；
//   - 不生成 query / body 的入参类型（只生成响应形状）。
// 需要更精确的形状时先在注解层补齐（先例见 spec #940 片五②的差集清单）。

export interface AIAssistantModeModels {
  expert: ModelOption | null
  normal: ModelOption | null
}

export interface AIChatMessageDTO {
  content: string
  created_at: string
  id: number
  images: string[] | null
  role: string
  sources: DiagnosisSource[] | null
}

export interface AIChatSessionDTO {
  created_at: string
  feature_key: string
  id: number
  model_name: string
  title: string
  updated_at: string
}

export interface AIImageUploadResultDTO {
  url: string
}

export interface AISessionRenameResultDTO {
  message: string
}

export interface DiagnosisBrandOption {
  label: string
  value: string
}

export interface DiagnosisFaultCodeItem {
  brand: string
  brand_cn: string
  causes: string
  fault_code: string
  fault_name: string
  id: number
  model_series: string
  page_num: number
  part_numbers: string
  safety_warning: string
  sop_steps: string
  source_file: string
  symptom: string
}

export interface DiagnosisFaultCodePage {
  items: DiagnosisFaultCodeItem[] | null
  total: number
}

export interface DiagnosisSource {
  id: string
  metadata: DiagnosisSourceMetadata
  text: string
}

export interface DiagnosisSourceMetadata {
  page_end: number
  page_start: number
  source_url: string
}

export interface ModelOption {
  base_url: string
  id: number
  model: string
  name: string
}

export interface UserModelDTO {
  api_key: string
  base_url: string
  created_at: string
  id: number
  model: string
  name: string
  updated_at: string
}
