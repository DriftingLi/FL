package practicemode

import (
	"gorm.io/gorm"
)

// 练习统计 module（Ticket #226）：一次 GROUP BY + 过滤描述符，产出 typed StatsDTO。
// 三套并行统计（题库/练习/错题）收敛为同一依赖；题库那套（questionbank.QuestionBankStatsDTO /
// questionbank.GroupByCount）已随 3c-1 搬进 internal/questionbank，错题那套（WrongQuestionStatsDTO）
// 随 3c-2 留驻 internal/service（4c 随错题域再搬）。

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
	ByType   map[string]PracticeTypeStat `json:"by_type" nullability:"nonnil"`
}

// statGroupPairRow GROUP BY 双计数结果行（key + count + pairCount，练习按题型统计 total/correct 用）。
type statGroupPairRow struct {
	Key       string
	Count     int64
	PairCount int64
}

// GroupByCountWithFilter 聚合引擎：按 dimension 一次 GROUP BY，同时统计维度总数与满足 filterExpr 的计数。
// 返回两个字典 key→count 与 key→filteredCount；filterExpr 为聚合条件（如 is_correct 判定表达式）。
// 导出：随练习域 3c-2 搬包（原 internal/service/stats_aggregate.go 私有 groupByCountWithFilter）。
func GroupByCountWithFilter(base *gorm.DB, dimension, filterExpr string) (map[string]int64, map[string]int64, error) {
	var rows []statGroupPairRow
	err := base.Select(dimension + " AS key, COUNT(*) AS count, COALESCE(SUM(" + filterExpr + "), 0) AS pair_count").Group(dimension).Scan(&rows).Error
	all := make(map[string]int64, len(rows))
	filtered := make(map[string]int64, len(rows))
	for _, r := range rows {
		all[r.Key] = r.Count
		filtered[r.Key] = r.PairCount
	}
	return all, filtered, err
}
