// Package service 叉车实操练习记录，对应 Python practice_service。
package service

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// PracticeService 叉车实操记录服务（注意：非题库练习）。
type PracticeService struct {
	db *gorm.DB
}

// NewPracticeService 创建实操记录服务。
func NewPracticeService(db *gorm.DB) *PracticeService {
	return &PracticeService{db: db}
}

// practiceRecordToDict 将记录转为 dict，includeOperations 控制是否含操作详情。
func practiceRecordToDict(r *model.PracticeRecord, includeOperations bool) map[string]interface{} {
	d := map[string]interface{}{
		"record_id":      r.RecordID,
		"student_id":     r.StudentID,
		"practice_type":  r.PracticeType,
		"duration":       r.Duration,
		"score":          r.Score,
		"status":         r.Status,
		"difficulty":     r.Difficulty,
		"scenario_id":    r.ScenarioID,
		"time_limit":     r.TimeLimit,
		"wrong_attempts": r.WrongAttempts,
		"created_at":     formatISO(r.CreatedAt),
	}
	if includeOperations {
		var ops interface{}
		if len(r.Operations) > 0 {
			_ = json.Unmarshal(r.Operations, &ops)
		}
		d["operations"] = ops
		var cp interface{}
		if len(r.CorrectParts) > 0 {
			_ = json.Unmarshal(r.CorrectParts, &cp)
		}
		d["correct_parts"] = cp
	}
	return d
}

// SaveRecord 保存实操记录，对应 Python save_practice_record。
func (s *PracticeService) SaveRecord(studentID int, params map[string]interface{}) (map[string]interface{}, error) {
	practiceType, _ := params["practice_type"].(string)
	if practiceType == "" {
		return nil, errors.New("请指定实操类型")
	}
	operations := params["operations"]
	if operations == nil {
		operations = []interface{}{}
	}
	opsJSON, _ := json.Marshal(operations)
	correctParts := params["correct_parts"]
	var cpJSON model.JSONB
	if correctParts != nil {
		b, _ := json.Marshal(correctParts)
		cpJSON = model.JSONB(b)
	}
	wrongAttempts := toIntDefault(params["wrong_attempts"], 0)
	rec := model.PracticeRecord{
		StudentID:     studentID,
		PracticeType:  practiceType,
		Duration:      toIntDefault(params["duration"], 0),
		Score:         toIntDefault(params["score"], 0),
		Operations:    model.JSONB(opsJSON),
		Status:        orDefault(getString(params, "status"), "completed"),
		Difficulty:    orDefault(getString(params, "difficulty"), "normal"),
		ScenarioID:    nilOrInt(params["scenario_id"]),
		TimeLimit:     nilOrInt(params["time_limit"]),
		CorrectParts:  cpJSON,
		WrongAttempts: wrongAttempts,
		CreatedAt:     beijingNow(),
	}
	if err := s.db.Create(&rec).Error; err != nil {
		return nil, err
	}
	return practiceRecordToDict(&rec, true), nil
}

// GetRecord 查询单条记录（studentID!=0 时校验归属）。
func (s *PracticeService) GetRecord(recordID, studentID int) (map[string]interface{}, error) {
	q := s.db.Where("record_id = ?", recordID)
	if studentID != 0 {
		q = q.Where("student_id = ?", studentID)
	}
	var rec model.PracticeRecord
	if err := q.First(&rec).Error; err != nil {
		return nil, errors.New("记录不存在")
	}
	return practiceRecordToDict(&rec, true), nil
}

// GetRecords 分页查询学员记录。
func (s *PracticeService) GetRecords(studentID, page, pageSize int, practiceType string) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.PracticeRecord{}).Where("student_id = ?", studentID)
	if practiceType != "" {
		q = q.Where("practice_type = ?", practiceType)
	}
	var total int64
	q.Count(&total)
	var records []model.PracticeRecord
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records)
	out := make([]map[string]interface{}, 0, len(records))
	for i := range records {
		out = append(out, practiceRecordToDict(&records[i], false))
	}
	return map[string]interface{}{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"records":   out,
	}
}

// GetStats 学员练习统计，对应 Python get_practice_stats。
func (s *PracticeService) GetStats(studentID int) map[string]interface{} {
	var records []model.PracticeRecord
	s.db.Where("student_id = ?", studentID).Find(&records)
	if len(records) == 0 {
		return map[string]interface{}{
			"total_count":      0,
			"avg_score":        0,
			"total_duration":   0,
			"type_stats":       map[string]interface{}{},
			"difficulty_stats": map[string]interface{}{},
			"score_trend":      []interface{}{},
			"skill_scores":     map[string]interface{}{},
		}
	}
	totalCount := len(records)
	sumScore := 0
	sumDuration := 0
	typeStats := map[string]map[string]interface{}{}
	diffStats := map[string]map[string]interface{}{}
	for _, r := range records {
		sumScore += r.Score
		sumDuration += r.Duration
		if _, ok := typeStats[r.PracticeType]; !ok {
			typeStats[r.PracticeType] = map[string]interface{}{"count": 0, "scores": []int{}, "total_duration": 0}
		}
		t := typeStats[r.PracticeType]
		t["count"] = toInt(t["count"]) + 1
		t["total_duration"] = toInt(t["total_duration"]) + r.Duration
		t["scores"] = append(t["scores"].([]int), r.Score)
		if _, ok := diffStats[r.Difficulty]; !ok {
			diffStats[r.Difficulty] = map[string]interface{}{"count": 0, "scores": []int{}}
		}
		d := diffStats[r.Difficulty]
		d["count"] = toInt(d["count"]) + 1
		d["scores"] = append(d["scores"].([]int), r.Score)
	}
	for _, t := range typeStats {
		scores := t["scores"].([]int)
		avg := 0.0
		if len(scores) > 0 {
			s := 0
			for _, v := range scores {
				s += v
			}
			avg = float64(s) / float64(len(scores))
		}
		t["avg_score"] = roundFloat1(avg)
		delete(t, "scores")
	}
	for _, d := range diffStats {
		scores := d["scores"].([]int)
		avg := 0.0
		if len(scores) > 0 {
			s := 0
			for _, v := range scores {
				s += v
			}
			avg = float64(s) / float64(len(scores))
		}
		d["avg_score"] = roundFloat1(avg)
		delete(d, "scores")
	}
	// 趋势：按时间排序后分 10 段取平均
	sortPracticeByCreated(records)
	step := len(records) / 10
	if step < 1 {
		step = 1
	}
	trend := []map[string]interface{}{}
	for i := 0; i < len(records); i += step {
		end := i + step
		if end > len(records) {
			end = len(records)
		}
		batch := records[i:end]
		s := 0
		for _, r := range batch {
			s += r.Score
		}
		avg := float64(s) / float64(len(batch))
		trend = append(trend, map[string]interface{}{
			"date":      formatISO(batch[0].CreatedAt),
			"avg_score": roundFloat1(avg),
		})
	}
	return map[string]interface{}{
		"total_count":      totalCount,
		"avg_score":        roundFloat1(float64(sumScore) / float64(totalCount)),
		"total_duration":   sumDuration,
		"type_stats":       typeStats,
		"difficulty_stats": diffStats,
		"score_trend":      trend,
		"skill_scores":     calculateSkillScores(records),
	}
}

// calculateSkillScores 技能维度得分，对应 Python _calculate_skill_scores。
func calculateSkillScores(records []model.PracticeRecord) map[string]interface{} {
	skills := map[string]map[string]interface{}{
		"inspection": {"label": "日常检查", "total": 0, "score_sum": 0},
		"diagnosis":  {"label": "故障诊断", "total": 0, "score_sum": 0},
		"assembly":   {"label": "部件拆装", "total": 0, "score_sum": 0},
		"speed":      {"label": "操作速度", "total": 0, "score_sum": 0},
		"accuracy":   {"label": "操作准确", "total": 0, "score_sum": 0},
	}
	for _, r := range records {
		if r.PracticeType == "inspection" || r.PracticeType == "diagnosis" || r.PracticeType == "assembly" {
			sk := skills[r.PracticeType]
			sk["total"] = toInt(sk["total"]) + 1
			sk["score_sum"] = toInt(sk["score_sum"]) + r.Score
		}
		if r.Duration > 0 && r.TimeLimit != nil && *r.TimeLimit > 0 {
			speedScore := 100 - int(float64(r.Duration)/float64(*r.TimeLimit)*100) + 50
			if speedScore > 100 {
				speedScore = 100
			}
			sk := skills["speed"]
			sk["total"] = toInt(sk["total"]) + 1
			sk["score_sum"] = toInt(sk["score_sum"]) + speedScore
		}
		accuracyScore := 100 - r.WrongAttempts*20
		if accuracyScore < 0 {
			accuracyScore = 0
		}
		sk := skills["accuracy"]
		sk["total"] = toInt(sk["total"]) + 1
		sk["score_sum"] = toInt(sk["score_sum"]) + accuracyScore
	}
	result := map[string]interface{}{}
	for key, v := range skills {
		total := toInt(v["total"])
		score := 0.0
		if total > 0 {
			score = float64(toInt(v["score_sum"])) / float64(total)
		}
		result[key] = map[string]interface{}{
			"label": v["label"],
			"score": roundFloat1(score),
		}
	}
	return result
}

// GetAdminStats 管理员练习统计，对应 Python get_admin_practice_stats。
func (s *PracticeService) GetAdminStats() map[string]interface{} {
	var totalRecords int64
	s.db.Model(&model.PracticeRecord{}).Count(&totalRecords)

	type aggRow struct {
		Key      string
		Count    int64
		AvgScore float64
	}

	var totalDuration int64
	s.db.Model(&model.PracticeRecord{}).Select("COALESCE(SUM(duration),0)").Scan(&totalDuration)
	var avgScore float64
	s.db.Model(&model.PracticeRecord{}).Select("COALESCE(AVG(score),0)").Scan(&avgScore)

	weekAgo := beijingNow().Add(-7 * 24 * time.Hour)
	todayAgo := beijingNow().Add(-24 * time.Hour)
	var recentCount, todayCount int64
	s.db.Model(&model.PracticeRecord{}).Where("created_at >= ?", weekAgo).Count(&recentCount)
	s.db.Model(&model.PracticeRecord{}).Where("created_at >= ?", todayAgo).Count(&todayCount)

	// 类型分布
	var typeRows []aggRow
	s.db.Model(&model.PracticeRecord{}).
		Select("practice_type AS key, COUNT(*) AS count, AVG(score) AS avg_score").
		Group("practice_type").Scan(&typeRows)
	typeDist := make([]map[string]interface{}, 0, len(typeRows))
	for _, r := range typeRows {
		typeDist = append(typeDist, map[string]interface{}{"type": r.Key, "count": r.Count, "avg_score": roundFloat1(r.AvgScore)})
	}

	var diffRows []aggRow
	s.db.Model(&model.PracticeRecord{}).
		Select("difficulty AS key, COUNT(*) AS count, AVG(score) AS avg_score").
		Group("difficulty").Scan(&diffRows)
	diffDist := make([]map[string]interface{}, 0, len(diffRows))
	for _, r := range diffRows {
		diffDist = append(diffDist, map[string]interface{}{"difficulty": r.Key, "count": r.Count, "avg_score": roundFloat1(r.AvgScore)})
	}

	return map[string]interface{}{
		"total_records":           totalRecords,
		"total_duration":          totalDuration,
		"avg_score":               roundFloat1(avgScore),
		"today_count":             todayCount,
		"recent_count":            recentCount,
		"type_distribution":       typeDist,
		"difficulty_distribution": diffDist,
	}
}

// GetAdminRecords 管理员查看所有练习记录，对应 Python get_admin_practice_records。
func (s *PracticeService) GetAdminRecords(page, pageSize int, practiceType string, studentID *int) map[string]interface{} {
	if page <= 0 {
		page = 1
	}
	if pageSize <= 0 {
		pageSize = 10
	}
	q := s.db.Model(&model.PracticeRecord{})
	if practiceType != "" {
		q = q.Where("practice_type = ?", practiceType)
	}
	if studentID != nil {
		q = q.Where("student_id = ?", *studentID)
	}
	var total int64
	q.Count(&total)
	var records []model.PracticeRecord
	q.Order("created_at DESC").Offset((page - 1) * pageSize).Limit(pageSize).Find(&records)
	out := make([]map[string]interface{}, 0, len(records))
	for i := range records {
		out = append(out, practiceRecordToDict(&records[i], true))
	}
	return map[string]interface{}{
		"total":     total,
		"page":      page,
		"page_size": pageSize,
		"records":   out,
	}
}

// nilOrInt 将值转为 *int，nil/0 返回 nil。
func nilOrInt(v interface{}) *int {
	if v == nil {
		return nil
	}
	i := toInt(v)
	if i == 0 {
		return nil
	}
	return &i
}

func sortPracticeByCreated(records []model.PracticeRecord) {
	for i := 1; i < len(records); i++ {
		for j := i; j > 0 && records[j-1].CreatedAt.After(records[j].CreatedAt); j-- {
			records[j-1], records[j] = records[j], records[j-1]
		}
	}
}

func roundFloat1(f float64) float64 {
	return float64(int(f*10+0.5)) / 10
}
