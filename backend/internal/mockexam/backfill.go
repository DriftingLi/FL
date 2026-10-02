// 存量回填：模拟考试总分（ADR-0068 决策 4）。
package mockexam

import (
	"encoding/json"

	"gorm.io/gorm"

	"forklift-training/internal/coerce"
	"forklift-training/internal/model"
)

// mockExamScoreBackfillBatch 回填批大小：一次 200 行，够小到不把整表读进内存，
// 够大到运维命令只发几十条 SQL。
const mockExamScoreBackfillBatch = 200

// MockExamScoreBackfillReport 回填统计。
type MockExamScoreBackfillReport struct {
	// Scanned 扫过的已交卷记录数。
	Scanned int
	// Updated 派生值（mock_exam.score / result.total_score / details[].score）确实被改写的记录数。
	// 幂等判据：同一条记录重跑一次必为 0 —— 回填只做「把派生值写成逐题事实的函数」，不重新判分。
	Updated int
	// NoFacts 没有可派生的逐题事实（result 缺失 / 不是 JSON 对象 / details 缺失或为 null）而原样保留的记录数。
	// 不猜、不补零：这类行的对错与得分事实本就不在库里，重算只会把「不知道」写成「0 分」。
	NoFacts int
}

// BackfillMockExamTotalScores 按逐题事实重算已交卷模拟考试的派生分值（ADR-0068 决策 4）。
//
// 口径：mock_exam.score 与 result.total_score、result.details[].score 都是**派生值**——
// 由 result.details[] 里已落库的逐题事实派生，本命令**不重新判分**：
//
//	本题得分 = max(details[].score, details[].ai_score)   // 短答旧口径：score=0，ai_score 才是 AI 给的分
//	total_score = Σ 本题得分
//	details[].score = 本题得分                              // 决策 3「明细同源」，也是回填后
//	                                                       // total_score == Σ details[].score 这条锁成立的前提
//
// 一律不动：is_correct / ai_score / max_score / correct_count / accuracy / 作答快照 / 状态。
// 这不是改判——旧口径下「多选半对」与「简答 AI 分」都没进总分，回填只是把同一份逐题事实
// 按新口径重新加总一次。
func BackfillMockExamTotalScores(db *gorm.DB) (MockExamScoreBackfillReport, error) {
	var report MockExamScoreBackfillReport
	var exams []model.MockExam
	err := db.
		Where("status = ?", StatusSubmitted).
		Order("id").
		FindInBatches(&exams, mockExamScoreBackfillBatch, func(_ *gorm.DB, _ int) error {
			for i := range exams {
				exam := &exams[i]
				report.Scanned++
				changed, hasFacts := rewriteMockExamDerivedScore(exam)
				if !hasFacts {
					report.NoFacts++
					continue
				}
				if !changed {
					continue
				}
				if err := db.Model(&model.MockExam{}).Where("id = ?", exam.ID).
					Updates(map[string]any{"score": exam.Score, "result": exam.Result}).Error; err != nil {
					return err
				}
				report.Updated++
			}
			return nil
		}).Error
	return report, err
}

// rewriteMockExamDerivedScore 就地重算一条记录的派生分值，返回（是否改动, 是否有可派生的逐题事实）。
// result 用 map 承载而不是反序列化进 MockExamSubmitDTO：回填不是迁移，除了本题得分与总分，
// 其余键（含未来新增键）必须逐字保持原样。
func rewriteMockExamDerivedScore(exam *model.MockExam) (changed bool, hasFacts bool) {
	if len(exam.Result) == 0 {
		return false, false
	}
	var payload map[string]any
	if err := json.Unmarshal(exam.Result, &payload); err != nil || payload == nil {
		return false, false
	}
	rawDetails, ok := payload["details"].([]any)
	if !ok {
		return false, false
	}

	total := 0.0
	for _, raw := range rawDetails {
		detail, ok := raw.(map[string]any)
		if !ok {
			continue
		}
		best := coerce.ToFloat(detail["score"])
		if aiScore, exists := detail["ai_score"]; exists && coerce.ToFloat(aiScore) > best {
			best = coerce.ToFloat(aiScore)
		}
		if coerce.ToFloat(detail["score"]) != best {
			detail["score"] = best
			changed = true
		}
		total += best
	}

	if coerce.ToFloat(payload["total_score"]) != total {
		payload["total_score"] = total
		changed = true
	}
	if exam.Score == nil || *exam.Score != total {
		exam.Score = coerce.FloatPtr(total)
		changed = true
	}
	if !changed {
		return false, true
	}

	buf, err := json.Marshal(payload)
	if err != nil {
		// 序列化失败：本行回滚成「不动」（changed/字段都还没被外层写库，因为外层只读改写后的值）
		return false, false
	}
	exam.Result = model.JSONB(buf)
	return true, true
}
