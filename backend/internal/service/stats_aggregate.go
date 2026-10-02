package service

import (
	"gorm.io/gorm"
)

// 统计聚合 module（Ticket #226）：一次 GROUP BY + 过滤描述符，产出 typed StatsDTO。
// 三套并行统计（题库/练习/错题）收敛为同一依赖；题库那套（questionbank.QuestionBankStatsDTO / questionbank.GroupByCount）
// 已随 3c-1 搬进 internal/questionbank，本文件只剩练习与错题两套。
// 各消费方保留各自业务语义（是否零填充维度 / 是否含正确率），shape-lock 测试冻结契约。

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

// WrongQuestionStatsDTO 错题统计（旧 wrong_question GetStats map 输出）。
// swaggertype 显式钉住 map 值类型：swag 对 map[string]int64 的 format 推断**不稳定** ——
// 同一份代码在不同环境生成的 swagger 产物，这一处会有/无 "format: int64"（实测 CI 与本机各执
// 一边，正是它把「再生成后工作树必须干净」的新鲜度锁变成随机红）。钉住值类型即消除推断。
type WrongQuestionStatsDTO struct {
	Total  int64            `json:"total"`
	ByType map[string]int64 `json:"by_type" swaggertype:"object,integer" nullability:"nonnil"`
}

// statGroupPairRow GROUP BY 双计数结果行（key + count + pairCount，练习按题型统计 total/correct 用）。
type statGroupPairRow struct {
	Key       string
	Count     int64
	PairCount int64
}

// groupByCountWithFilter 聚合引擎：按 dimension 一次 GROUP BY，同时统计维度总数与满足 filterExpr 的计数。
// 返回两个字典 key→count 与 key→filteredCount；filterExpr 为聚合条件（如 is_correct 判定表达式）。
func groupByCountWithFilter(base *gorm.DB, dimension, filterExpr string) (map[string]int64, map[string]int64, error) {
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
