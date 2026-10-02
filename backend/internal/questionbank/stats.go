package questionbank

import (
	"gorm.io/gorm"
)

// 统计聚合（Ticket #226）：一次 GROUP BY + 过滤描述符，产出 typed StatsDTO。
// 题库侧只留 QuestionBankStatsDTO / GroupByCount；练习与错题两套仍留在 internal/service
// （3c-2 随练习域搬 PracticeStatsDTO / PracticeTypeStat / GroupByCountWithFilter）。

// QuestionBankStatsDTO 题库统计（旧 question_service GetStats map 输出）。
//
// 两个 map 与 WrongQuestionStatsDTO 同因同解：swag 对 map[string]int64 的 format 推断不稳定
// （同一份代码在不同环境会生成 有/无 "format: int64" 两种产物），显式 swaggertype 钉住值类型即消除
// —— 该定义在片六随 questionBank 端点首次进入 swagger，随即把新鲜度锁变成随机红（CI 实测）。
type QuestionBankStatsDTO struct {
	Total    int64            `json:"total"`
	ByType   map[string]int64 `json:"by_type" swaggertype:"object,integer" nullability:"nonnil"`
	ByStatus map[string]int64 `json:"by_status" swaggertype:"object,integer" nullability:"nonnil"`
}

// statGroupRow GROUP BY 单维结果行（key=维度值，count=行数）。
type statGroupRow struct {
	Key   string
	Count int64
}

// GroupByCount 聚合引擎：按 dimension 列对 base 查询一次 GROUP BY，返回维度→计数字典
// （仅含实际存在分组的维度；零填充由调用方按业务语义决定）。
// base 为已含 WHERE/JOIN 的查询骨架，dimension 为分组列（可带限定如 question.type）。
// 导出：错题统计在用（internal/wrongquestion/stats.go 的 GetStats，波 4c 随域搬入）。
func GroupByCount(base *gorm.DB, dimension string) map[string]int64 {
	var rows []statGroupRow
	base.Select(dimension + " AS key, COUNT(*) AS count").Group(dimension).Scan(&rows)
	m := make(map[string]int64, len(rows))
	for _, r := range rows {
		m[r.Key] = r.Count
	}
	return m
}
