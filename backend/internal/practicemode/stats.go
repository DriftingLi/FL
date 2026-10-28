package practicemode

import (
	"gorm.io/gorm"

	"forklift-training/internal/coerce"
)

// 练习统计 module（Ticket #226）：一次 GROUP BY + 过滤描述符，产出 typed StatsDTO。
// 三套并行统计（题库/练习/错题）收敛为同一依赖；题库那套（questionbank.QuestionBankStatsDTO /
// questionbank.GroupByCount）已随 3c-1 搬进 internal/questionbank，错题那套（WrongQuestionStatsDTO）
// 4c 已随错题域搬进 internal/wrongquestion/stats.go。

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
	// ByTag 考点标签维度的正确率（#2068）：与 by_type 并列、不替换；只含有作答的标签。
	ByTag []PracticeTagStat `json:"by_tag" nullability:"nonnil"`
}

// PracticeTagStat 练习统计按题库标签（考点）明细（#2068）。
type PracticeTagStat struct {
	TagID    int     `json:"tag_id"`
	TagName  string  `json:"tag_name"`
	Total    int64   `json:"total"`
	Correct  int64   `json:"correct"`
	Accuracy float64 `json:"accuracy"`
}

// practiceTagStats 按题库标签聚合正确率（#2068）。
//
// 口径（可复算）：分母 = 该学员在本标签题目上的**全部作答次数**（含同一题重做，来源
// question_practice_record）；分子 = 其中 is_correct 的次数；不设时间窗，与 /stats 总览一致。
// 一题多标签 ⇒ 该次作答在每个标签下各计一次（标签是维度，不是互斥分类）。
// 只返回 total > 0 的标签 ⇒ 不会出现 0 分母的 NaN；排序按标签 sort_order、id（确定性）。
func practiceTagStats(base func() *gorm.DB) ([]PracticeTagStat, error) {
	var rows []struct {
		TagID   int    `gorm:"column:tag_id"`
		TagName string `gorm:"column:tag_name"`
		Total   int64  `gorm:"column:total"`
		Correct int64  `gorm:"column:correct"`
	}
	err := base().
		Joins("JOIN question_tag_relation AS r ON r.question_id = question_practice_record.question_id").
		Joins("JOIN question_tag AS t ON t.id = r.tag_id").
		Select("t.id AS tag_id, t.name AS tag_name, COUNT(*) AS total, " +
			"COALESCE(SUM(CASE WHEN question_practice_record.is_correct THEN 1 ELSE 0 END), 0) AS correct").
		Group("t.id, t.name, t.sort_order").
		Order("t.sort_order ASC, t.id ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	out := make([]PracticeTagStat, 0, len(rows))
	for _, r := range rows {
		acc := 0.0
		if r.Total > 0 {
			acc = coerce.RoundFloat1(float64(r.Correct) / float64(r.Total) * 100)
		}
		out = append(out, PracticeTagStat{TagID: r.TagID, TagName: r.TagName, Total: r.Total, Correct: r.Correct, Accuracy: acc})
	}
	return out, nil
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
