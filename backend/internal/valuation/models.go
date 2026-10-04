// 仓储层数据模型（原 internal/valuation/repository，#1514 波 7 并回域包）。
// 仅保留 battery 相关 DB 实体（手写仓储直接使用业务 model，不再生成 sqlc 模型）
package valuation

// 本文件原本由 sqlc 生成，重构后已无 sqlc 模型。
// battery 相关持久化直接使用 BatteryEvaluation 等业务 DTO，
// 故此处不再保留任何数据库实体结构体，避免冗余。
