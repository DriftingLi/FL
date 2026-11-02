// 本文件：易错项（「最多人选错的选项」）的唯一实现（#2155）。
//
// 这个值有两个对外出口，必须永远同值：
//  1. 错题本列表 top_wrong_option（internal/wrongquestion，批量装配整页）；
//  2. 练习提交 / 错题重做的结果卡 common_wrong（FinalizeSubmitResult，单题）。
//
// #2068 只统一了 ①，② 仍是旧口径——分母取**总**作答次数、不过滤空答案、并列无二级排序。
// 同一道题因此可能两处不同值，甚至一边有值一边缺省（票 #2155 的现象）。本文件把 ② 收编到
// 同一份 SQL 判据上：改口径只改这里，两个出口自动同进同退。
package practicemode

import (
	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// topWrongOptionMinSample 易错项的最少样本量，分母是**该题判错的作答次数**（#2068/#2155）：
// 样本太小时「最多人选错」只是一两个人的偶然，不当作系统性误区。按**总**作答数算的话，
// 「10 人做对、4 人选错」也会凑够阈值，把一个 4 票的答案当成系统性误区——错误作答的分布本身就是分母。
const topWrongOptionMinSample = 5

// shortAnswerQuestionType 简答题**没有选项**：它的 user_answer 是自由文本，把「最多人写的那段话」
// 当易错项外发等于泄漏别人的作答，且字段名/语义都对不上。守卫只写在这份实现的 SQL 里。
const shortAnswerQuestionType = "short_answer"

// CommonWrongOption 单题易错项：结果卡 common_wrong 的取值点（#2155）。
//
// 刻意不接 qType 入参——简答题守卫归 SQL（JOIN question 判 type）。调用方再写一条 Go 判据就是
// 第二套口径：改了 SQL 忘了 Go 那侧，两处就会静默分飞，而这正是本票要收掉的老病。
// 代价是简答题也付一次 JOIN 查询，换的是「两个出口永远同值」。
func CommonWrongOption(db *gorm.DB, questionID int) *string {
	if ans, ok := LoadTopWrongOptions(db, []int{questionID})[questionID]; ok {
		return &ans
	}
	return nil
}

// LoadTopWrongOptions 批量取每题的「最多人选错的选项」（#2068 起是错题本的装载器，#2155 起
// 同时是结果卡 common_wrong 的唯一实现）。
//
// 口径（可复算）：分母 = 该题**判错**的作答次数（question_practice_record，按题目聚合、不限学员；
// 简答题整行排除）；分子 = 按 user_answer 分组的 is_correct=false 次数；空答案不计
// （空提交不是「选错某个选项」）。正确项因此不进候选——判错行的 user_answer 本来就不等于正确答案。
// 并列按答案字典序升序取第一条，保证同一份数据每次返回同一个值。
// 一次查询覆盖整页（禁 N+1）；调用方按缺失即「无易错项」取值。
func LoadTopWrongOptions(db *gorm.DB, questionIDs []int) map[int]string {
	out := make(map[int]string, len(questionIDs))
	if len(questionIDs) == 0 {
		return out
	}
	var rows []struct {
		QuestionID int    `gorm:"column:question_id"`
		UserAnswer string `gorm:"column:user_answer"`
		Cnt        int64  `gorm:"column:cnt"`
	}
	db.Model(&model.QuestionPracticeRecord{}).
		Joins("JOIN question ON question.id = question_practice_record.question_id").
		Select("question_practice_record.question_id AS question_id, user_answer, COUNT(*) AS cnt").
		Where("question_practice_record.question_id IN ? AND question_practice_record.is_correct = ? "+
			"AND question_practice_record.user_answer <> '' AND question.type <> ?",
			questionIDs, false, shortAnswerQuestionType).
		Group("question_practice_record.question_id, user_answer").Scan(&rows)

	wrongByQuestion := make(map[int]int64, len(questionIDs))
	for _, r := range rows {
		wrongByQuestion[r.QuestionID] += r.Cnt
	}
	best := make(map[int]struct {
		Answer string
		Cnt    int64
	}, len(rows))
	for _, r := range rows {
		if wrongByQuestion[r.QuestionID] < topWrongOptionMinSample {
			continue
		}
		cur, seen := best[r.QuestionID]
		if !seen || r.Cnt > cur.Cnt || (r.Cnt == cur.Cnt && r.UserAnswer < cur.Answer) {
			best[r.QuestionID] = struct {
				Answer string
				Cnt    int64
			}{Answer: r.UserAnswer, Cnt: r.Cnt}
		}
	}
	for q, b := range best {
		out[q] = b.Answer
	}
	return out
}
