package api

import "forklift-training/internal/service"

// provideAI AI 域：配置、内容生成、助手与外部诊断代理。
func provideAI(c *coreSingletons, d *Deps) {
	d.AIConfigSvc = c.aiConfigSvc
	d.ContentGenSvc = c.contentGenSvc
	d.AIAssistantSvc = service.NewAIAssistantService(c.db, c.aiConfigSvc, c.fileSvc, c.cfg.SecretKey, c.logger, c.aiModelPort)
	d.DiagnosisProxySvc = service.NewDiagnosisProxyService(c.cfg.DiagnosisAssistantURL, c.logger)
}
