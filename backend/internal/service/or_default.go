package service

// orDefault AI 域出包后本包自用副本（原 ai_service.go:227，随波 2c 搬进 internal/aiassistant）。
// 两处调用：practice_mode_service.go:370、question_service.go:405；纯函数，读参数不读服务状态，
// P3 随 internal/core 收编或进叶子包时再合并（口径见 docs/adr/ADR-0070 的 2c 回写段）。
func orDefault(s, def string) string {
	if s != "" {
		return s
	}
	return def
}
