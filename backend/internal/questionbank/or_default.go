package questionbank

// OrDefault AI 域出包后本包自用副本（原 ai_service.go:227，随波 2c 搬进 internal/aiassistant）。
// 3c-1 随题库域搬进本包：原先 service 侧两处调用，question_service.go 那处随域进包、
// practice_mode_service.go 那处留到期 3c-2 —— 3c-2 已落地（练习域改调本函数，
// internal/service/or_default.go 随之删除），本包这份成为全仓唯一副本。
// 纯函数，读参数不读服务状态；原 service 侧副本已删，本包这份是全仓唯一出处。
func OrDefault(s, def string) string {
	if s != "" {
		return s
	}
	return def
}
