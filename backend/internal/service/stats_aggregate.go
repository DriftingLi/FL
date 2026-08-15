package service

import (
	"gorm.io/gorm"
)

// 统计聚合 module（Ticket #226）。三套并行统计（题库/练习/错题）收敛为同一依赖：
// 一次 GROUP BY + 过滤描述符，产出 typed StatsDTO。
// 各消费方保留各自业务语义（是否零填充维度 / 是否含正确率），shape-lock 测试冻结契约。

// QuestionBankStatsDTO 题库统计（旧 question_service GetStats map 输出）。
type QuestionBankStatsDTO struct {
	Total    int64            `json:"total"`
	ByType   map[string]int64 `json:"by_type"`
	ByStatus map[string]int64 `json:"by_status"`
}

// PracticeTypeStat 练习统计按题型明细（旧内层 map {total, correct}），accuracy 为加性新增 key（#226）。
type PracticeTypeStat struct {
	Total    int64   `json:"total"`
	Correct  int64   `json:"correct"`
	Accuracy float64 `json:"accuracy"`
}

// PracticeStatsDTO 练习统计（旧 practice_mode GetStats map 输出；by_type 每项新增 accuracy）。
type PracticeStatsDTO struct {
	Total    int64                       `json:"total"`
	Correct  int64                       `json:"correct"`
	Wrong    int64                       `json:"wrong"`
	Accuracy float64                     `json:"accuracy"`
	ByType   map[string]PracticeTypeStat `json:"by_type"`
}

// WrongQuestionStatsDTO 错题统计（旧 wrong_question GetStats map 输出）。
type WrongQuestionStatsDTO struct {
	Total  int64            `json:"total"`
	ByType map[string]int64 `json:"by_type"`
}

// statGroupRow GROUP BY 单维结果行（key=维度值，count=行数）。
type statGroupRow struct {
	Key   string
	Count int64
}

// statGroupPairRow GROUP BY 双计数结果行（key + count + pairCount，练习按题型统计 total/correct 用）。
type statGroupPairRow struct {
	Key       string
	Count     int64
	PairCount int64
}

// groupByCount 聚合引擎：按 dimension 列对 base 查询一次 GROUP BY，返回维度→计数字典
// （仅含实际存在分组的维度；零填充由调用方按业务语义决定）。
// base 为已含 WHERE/JOIN 的查询骨架，dimension 为分组列（可带限定如 question.type）。
func groupByCount(base *gorm.DB, dimension string) map[string]int64 {
	var rows []statGroupRow
	base.Select(dimension + " AS key, COUNT(*) AS count").Group(dimension).Scan(&rows)
	m := make(map[string]int64, len(rows))
	for _, r := range rows {
		m[r.Key] = r.Count
	}
	return m
}

// groupByCountWithFilter 聚合引擎：按 dimension 一次 GROUP BY，同时统计维度总数与满足 filterExpr 的计数。
// 返回两个字典 key→count 与 key→filteredCount；filterExpr 为聚合条件（如 is_correct 判定表达式）。
func groupByCountWithFilter(base *gorm.DB, dimension, filterExpr string) (map[string]int64, map[string]int64) {
	var rows []statGroupPairRow
	base.Select(dimension + " AS key, COUNT(*) AS count, COALESCE(SUM(" + filterExpr + "), 0) AS pair_count").Group(dimension).Scan(&rows)
	all := make(map[string]int64, len(rows))
	filtered := make(map[string]int64, len(rows))
	for _, r := range rows {
		all[r.Key] = r.Count
		filtered[r.Key] = r.PairCount
	}
	return all, filtered
}
