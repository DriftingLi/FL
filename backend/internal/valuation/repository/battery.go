// Package repository - 电池 RUL 评估数据访问
// 与 sqlc 生成的查询并存，使用裸 pgx 操作
// 物理独立，不修改任何现有 sqlc 文件
package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"forklift-training/internal/valuation/model"
)

// BatteryRepository 电池 RUL 评估仓储
// 持有独立的连接池引用，与 sqlc Queries 完全隔离
type BatteryRepository struct {
	pool *pgxpool.Pool
}

// NewBatteryRepository 构造仓储
func NewBatteryRepository(pool *pgxpool.Pool) *BatteryRepository {
	return &BatteryRepository{pool: pool}
}

// CreateEvaluation 插入评估主记录，返回完整行（含 id/timestamps）
// userID>0 时写入归属；userID=0 时落 NULL（匿名/历史数据）
func (r *BatteryRepository) CreateEvaluation(ctx context.Context, eval *model.BatteryEvaluation, features []model.CycleFeature, userID int) (*model.BatteryEvaluation, error) {
	tx, err := r.pool.BeginTx(ctx, pgx.TxOptions{})
	if err != nil {
		return nil, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	modelText := ""
	if eval.BatteryModel != "" {
		modelText = eval.BatteryModel
	}
	importanceJSON, _ := json.Marshal(eval.FeatureImportance)
	if importanceJSON == nil {
		importanceJSON = []byte("[]")
	}
	reportPath := ""
	if eval.ReportPdfPath != "" {
		reportPath = eval.ReportPdfPath
	}

	row := tx.QueryRow(ctx, `
		INSERT INTO battery_evaluations (
			battery_type, battery_model, cycle_count, rul_cycles, soh_percent,
			confidence, confidence_low, confidence_high, feature_importance, report_pdf_path,
			user_id
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id, created_at, updated_at
	`,
		string(eval.BatteryType), nullableString(modelText), eval.CycleCount, eval.RulCycles, eval.SohPercent,
		eval.Confidence, int32(eval.ConfidenceLow), int32(eval.ConfidenceHigh), importanceJSON, nullableString(reportPath),
		nullableUserID(userID),
	)
	var id int64
	var createdAt, updatedAt time.Time
	if err := row.Scan(&id, &createdAt, &updatedAt); err != nil {
		return nil, fmt.Errorf("插入评估记录失败: %w", err)
	}

	// 批量插入周期特征
	for _, f := range features {
		fvJSON, _ := json.Marshal(f.FeatureVector.AsSlice())
		statsJSON, _ := json.Marshal(f.RawStats)
		_, err := tx.Exec(ctx, `
			INSERT INTO battery_cycle_features (
				evaluation_id, cycle_index, feature_vector, raw_stats, soh_at_cycle
			) VALUES ($1, $2, $3, $4, $5)
		`, id, f.CycleIndex, fvJSON, statsJSON, f.SohAtCycle)
		if err != nil {
			return nil, fmt.Errorf("插入周期特征失败 cycle=%d: %w", f.CycleIndex, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("提交事务失败: %w", err)
	}

	eval.ID = id
	eval.CreatedAt = createdAt.Format("2006-01-02T15:04:05Z07:00")
	eval.UpdatedAt = updatedAt.Format("2006-01-02T15:04:05Z07:00")
	eval.CycleFeatures = features
	return eval, nil
}

// GetEvaluation 查询评估详情（含周期特征，不按用户过滤）
// 用于公开的报告生成/下载场景；鉴权详情请用 GetEvaluationByUser
func (r *BatteryRepository) GetEvaluation(ctx context.Context, id int64) (*model.BatteryEvaluation, error) {
	return r.getBatteryEvaluation(ctx, id, 0, false)
}

// GetEvaluationByUser 查询评估详情并校验归属（user_id 必须等于 userID）
// 用于登录用户查看自己的电池评估详情；不属于该用户的记录返回 pgx.ErrNoRows
func (r *BatteryRepository) GetEvaluationByUser(ctx context.Context, id int64, userID int) (*model.BatteryEvaluation, error) {
	return r.getBatteryEvaluation(ctx, id, userID, true)
}

// getBatteryEvaluation 主表查询 + 周期特征加载。
// enforceOwner=true 时追加 user_id 过滤；=false 时仅按 id 查询（公开场景）
func (r *BatteryRepository) getBatteryEvaluation(ctx context.Context, id int64, userID int, enforceOwner bool) (*model.BatteryEvaluation, error) {
	var (
		bid           int64
		batteryType   string
		batteryModel  *string
		cycleCount    int32
		rulCycles     int32
		soh           float64
		conf          float64
		confLow       int32
		confHigh      int32
		importanceRaw []byte
		reportPath    *string
		createdAt     time.Time
		updatedAt     time.Time
	)
	if enforceOwner {
		row := r.pool.QueryRow(ctx, `
			SELECT id, battery_type, battery_model, cycle_count, rul_cycles, soh_percent,
			       confidence, confidence_low, confidence_high, feature_importance, report_pdf_path,
			       created_at, updated_at
			FROM battery_evaluations WHERE id = $1 AND user_id = $2
		`, id, userID)
		if err := row.Scan(&bid, &batteryType, &batteryModel, &cycleCount, &rulCycles, &soh,
			&conf, &confLow, &confHigh, &importanceRaw, &reportPath, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
	} else {
		row := r.pool.QueryRow(ctx, `
			SELECT id, battery_type, battery_model, cycle_count, rul_cycles, soh_percent,
			       confidence, confidence_low, confidence_high, feature_importance, report_pdf_path,
			       created_at, updated_at
			FROM battery_evaluations WHERE id = $1
		`, id)
		if err := row.Scan(&bid, &batteryType, &batteryModel, &cycleCount, &rulCycles, &soh,
			&conf, &confLow, &confHigh, &importanceRaw, &reportPath, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
	}
	eval := &model.BatteryEvaluation{
		ID:             bid,
		BatteryType:    model.BatteryType(batteryType),
		CycleCount:     int(cycleCount),
		RulCycles:      int(rulCycles),
		SohPercent:     soh,
		Confidence:     conf,
		ConfidenceLow:  int(confLow),
		ConfidenceHigh: int(confHigh),
		CreatedAt:      createdAt.Format("2006-01-02T15:04:05Z07:00"),
		UpdatedAt:      updatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}
	if batteryModel != nil {
		eval.BatteryModel = *batteryModel
	}
	if reportPath != nil {
		eval.ReportPdfPath = *reportPath
	}
	if len(importanceRaw) > 0 {
		var imp []model.FeatureImportance
		if err := json.Unmarshal(importanceRaw, &imp); err == nil {
			eval.FeatureImportance = imp
		}
	}
	// 查询周期特征
	rows, err := r.pool.Query(ctx, `
		SELECT id, cycle_index, feature_vector, raw_stats, soh_at_cycle
		FROM battery_cycle_features WHERE evaluation_id = $1 ORDER BY cycle_index ASC
	`, id)
	if err != nil {
		return nil, fmt.Errorf("查询周期特征失败: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var (
			fid        int64
			cycleIdx   int32
			fvJSON     []byte
			statsJSON  []byte
			sohAtCycle float64
		)
		if err := rows.Scan(&fid, &cycleIdx, &fvJSON, &statsJSON, &sohAtCycle); err != nil {
			return nil, fmt.Errorf("扫描周期特征失败: %w", err)
		}
		var fvSlice []float64
		if err := json.Unmarshal(fvJSON, &fvSlice); err != nil {
			return nil, fmt.Errorf("解析特征向量失败: %w", err)
		}
		var fv model.FeatureVector
		for i := 0; i < 20 && i < len(fvSlice); i++ {
			fv[i] = fvSlice[i]
		}
		var stats model.RawStats
		if err := json.Unmarshal(statsJSON, &stats); err != nil {
			return nil, fmt.Errorf("解析原始统计失败: %w", err)
		}
		eval.CycleFeatures = append(eval.CycleFeatures, model.CycleFeature{
			ID:            fid,
			EvaluationID:  bid,
			CycleIndex:    int(cycleIdx),
			FeatureVector: fv,
			RawStats:      stats,
			SohAtCycle:    sohAtCycle,
		})
	}
	return eval, nil
}

// ListEvaluations 分页查询摘要
// batteryType 为空时不过滤；userID>0 时仅返回该用户的记录，userID=0 时返回全部
func (r *BatteryRepository) ListEvaluations(ctx context.Context, batteryType string, userID int, limit, offset int) ([]model.BatteryEvaluationSummary, int, error) {
	// 动态拼装 WHERE：battery_type / user_id 均为可选过滤
	where := make([]string, 0, 2)
	args := make([]any, 0, 3)
	argIdx := 1
	if batteryType != "" {
		where = append(where, fmt.Sprintf("battery_type = $%d", argIdx))
		args = append(args, batteryType)
		argIdx++
	}
	if userID > 0 {
		where = append(where, fmt.Sprintf("user_id = $%d", argIdx))
		args = append(args, userID)
		argIdx++
	}
	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + joinStrings(where, " AND ")
	}

	// 总数
	var total int
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM battery_evaluations %s`, whereClause)
	if err := r.pool.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// 列表
	listArgs := append(args, limit, offset)
	listQuery := fmt.Sprintf(`
		SELECT id, battery_type, battery_model, cycle_count, rul_cycles, soh_percent, confidence, created_at
		FROM battery_evaluations %s
		ORDER BY created_at DESC LIMIT $%d OFFSET $%d
	`, whereClause, argIdx, argIdx+1)
	rows, err := r.pool.Query(ctx, listQuery, listArgs...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	out := make([]model.BatteryEvaluationSummary, 0, limit)
	for rows.Next() {
		var (
			id        int64
			bt        string
			modelPtr  *string
			count     int32
			rul       int32
			soh       float64
			conf      float64
			createdAt time.Time
		)
		if err := rows.Scan(&id, &bt, &modelPtr, &count, &rul, &soh, &conf, &createdAt); err != nil {
			return nil, 0, err
		}
		s := model.BatteryEvaluationSummary{
			ID:          id,
			BatteryType: model.BatteryType(bt),
			CycleCount:  int(count),
			RulCycles:   int(rul),
			SohPercent:  soh,
			Confidence:  conf,
			CreatedAt:   createdAt.Format("2006-01-02T15:04:05Z07:00"),
		}
		if modelPtr != nil {
			s.BatteryModel = *modelPtr
		}
		out = append(out, s)
	}
	return out, total, nil
}

// UpdateReportPath 更新 PDF 报告路径
func (r *BatteryRepository) UpdateReportPath(ctx context.Context, id int64, path string) error {
	_, err := r.pool.Exec(ctx, `UPDATE battery_evaluations SET report_pdf_path = $1, updated_at = NOW() WHERE id = $2`, path, id)
	return err
}

// EvaluationExists 仅检查记录是否存在（用于轻量级 404 判断）
func (r *BatteryRepository) EvaluationExists(ctx context.Context, id int64) (bool, error) {
	var exists bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM battery_evaluations WHERE id = $1)`, id).Scan(&exists)
	return exists, err
}

// nullableString 把空串转 nil 便于 NULL 落库
func nullableString(s string) any {
	if s == "" {
		return nil
	}
	return s
}
