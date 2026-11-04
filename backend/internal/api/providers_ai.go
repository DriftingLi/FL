package api

import "forklift-training/internal/aiassistant"

// provideAI AI 域：配置、内容生成、助手与外部诊断代理。
func provideAI(c *coreSingletons, d *Deps) {
	d.AIConfigSvc = c.aiConfigSvc
	d.ContentGenSvc = c.contentGenSvc
	d.AIAssistantSvc = aiassistant.NewService(c.db, c.aiConfigSvc, c.fileSvc, c.cfg.SecretKey, c.logger, c.aiModelPort)
	// 回答配图回源代理：只服务 /diagnosis/ossimg/*，基准与 adapter 侧白名单同源（ADR-0091）
	d.DiagnosisProxySvc = aiassistant.NewDiagnosisProxyService(c.cfg.DiagnosisV2OSSBase, c.logger)
	// v2 引导只读面（intents / systems / faults / code-hint / health，票 2242）：
	// 与对话面共用同一组上游配置，但走一次性 JSON 而不是 SSE。
	d.DiagnosisCatalogSvc = aiassistant.NewDiagnosisCatalogService(c.cfg.DiagnosisAssistantURL, c.cfg.DiagnosisV2APIKey, c.logger)
}
