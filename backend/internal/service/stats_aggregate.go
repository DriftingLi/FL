package service

// 统计聚合 module（Ticket #226）：一次 GROUP BY + 过滤描述符，产出 typed StatsDTO。
// 三套并行统计（题库/练习/错题）收敛为同一依赖；题库那套随 3c-1 搬进 internal/questionbank、
// 练习那套（PracticeStatsDTO / PracticeTypeStat / GroupByCountWithFilter）随 3c-2 搬进 internal/practicemode，
// 本文件只剩错题一套（4c 随错题域再搬）。各消费方保留各自业务语义（是否零填充维度 / 是否含正确率），
// shape-lock 测试冻结契约。

// WrongQuestionStatsDTO 错题统计（旧 wrong_question GetStats map 输出）。
// swaggertype 显式钉住 map 值类型：swag 对 map[string]int64 的 format 推断**不稳定** ——
// 同一份代码在不同环境生成的 swagger 产物，这一处会有/无 "format: int64"（实测 CI 与本机各执
// 一边，正是它把「再生成后工作树必须干净」的新鲜度锁变成随机红）。钉住值类型即消除推断。
type WrongQuestionStatsDTO struct {
	Total  int64            `json:"total"`
	ByType map[string]int64 `json:"by_type" swaggertype:"object,integer" nullability:"nonnil"`
}
