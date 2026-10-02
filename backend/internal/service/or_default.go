package service

// orDefault AI 域出包后本包自用副本（原 ai_service.go:227，随波 2c 搬进 internal/aiassistant）。
// 3c-1 后仅剩 practice_mode_service.go 一处调用（题库域已带自己的 OrDefault 副本）；
// 3c-2 练习域搬走后本文件即删。纯函数，读参数不读服务状态，
// P3 随 internal/core 收编或进叶子包时再合并（口径见 docs/adr/ADR-0070 的 2c 回写段）。
func orDefault(s, def string) string {
	if s != "" {
		return s
	}
	return def
}
