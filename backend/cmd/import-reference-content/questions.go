// 题库导入管线：存量加载、合并去重（导入↔存量、存量内部）、写入与真题标记。
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

// realExamTagCode 真题标签（沿 baseline 考点标签写法 seed）。
const realExamTagCode = "real_exam"

// importScore 客观题分值：与 baseline 种子及 practice/mock 分值表一致
// （mock 流中 q.Score>0 会覆盖分值表，取表内同值才不会改变现有满分语义）。
var importScore = map[string]int{
	"single_choice": 3,
	"multi_choice":  4,
	"true_false":    2,
}

// ExistingQ 存量题的比对所需字段。
type ExistingQ struct {
	ID           int
	Type         string
	CredentialID int // NULL 记 0
	Content      string
	Explanation  string
}

// EnrichAction 对存量题的就地更新（解析补全，id 不变保引用）。
type EnrichAction struct {
	Existing ExistingQ
	NewExpl  string
	Source   *SourceFile
	Line     int
}

// ExistingDedupAction 存量内部重复的处理结果。
type ExistingDedupAction struct {
	Keep       ExistingQ
	Remove     ExistingQ
	Referenced bool // 有引用 → 保留并报告；无引用 → 删除
}

// CategoryStat 按资料分类聚合的解析统计。
type CategoryStat struct {
	Parsed int            // 解析出的题目（含跳过）
	Valid  int            // 计划入库（去重后）
	Skips  map[string]int // 原因 → 数量
}

func newCategoryStat() *CategoryStat { return &CategoryStat{Skips: map[string]int{}} }

// QuestionPlan 干跑/写入共用的题库操作计划。
type QuestionPlan struct {
	Inserts    []ParsedQuestion      // 待插入（已去重）
	Enriches   []EnrichAction        // 存量题解析补全
	ExistingDD []ExistingDedupAction // 存量内部重复
	Skips      []SkipNote            // 校验跳过明细
	Stats      map[string]*CategoryStat
}

// credKey 去重键：证件内按归一化题干哈希。
type credKey struct {
	credID int
	hash   string
}

// LoadExistingQuestions 读取存量题（比对所需字段）。
func LoadExistingQuestions(ctx context.Context, pool *pgxpool.Pool) ([]ExistingQ, error) {
	rows, err := pool.Query(ctx, `
		SELECT id, type, COALESCE(credential_id, 0), content, COALESCE(explanation, '')
		FROM question`)
	if err != nil {
		return nil, fmt.Errorf("读取存量题失败: %w", err)
	}
	defer rows.Close()
	var out []ExistingQ
	for rows.Next() {
		var q ExistingQ
		if err := rows.Scan(&q.ID, &q.Type, &q.CredentialID, &q.Content, &q.Explanation); err != nil {
			return nil, fmt.Errorf("扫描存量题失败: %w", err)
		}
		out = append(out, q)
	}
	return out, rows.Err()
}

// BuildQuestionPlan 汇总解析结果并生成操作计划：
//  1. 同证件内去重（保留解析更优版本，处理顺序稳定）；
//  2. 与存量比对：命中 → 跳过插入，可补解析时生成 EnrichAction（存量 id 不变）；
//  3. 存量内部重复分组：保留解析最优，其余视引用情况删除/保留（写入阶段处置）。
func BuildQuestionPlan(parsed []ParsedQuestion, skips []SkipNote, existing []ExistingQ, credIDs map[string]int) *QuestionPlan {
	plan := &QuestionPlan{Skips: skips, Stats: map[string]*CategoryStat{}}
	for _, s := range skips {
		if plan.Stats[s.Source.Category] == nil {
			plan.Stats[s.Source.Category] = newCategoryStat()
		}
		plan.Stats[s.Source.Category].Skips[s.Reason]++
	}

	// 存量索引：证件 id → 归一化题干哈希 → 行列表。
	existingIdx := map[int]map[string][]ExistingQ{}
	for _, e := range existing {
		h := stemHash(e.Content)
		if existingIdx[e.CredentialID] == nil {
			existingIdx[e.CredentialID] = map[string][]ExistingQ{}
		}
		existingIdx[e.CredentialID][h] = append(existingIdx[e.CredentialID][h], e)
	}

	// 导入侧：同证件去重 + 与存量比对。
	importedIdx := map[credKey]int{} // key → plan.Inserts 下标
	for i := range parsed {
		q := &parsed[i]
		if plan.Stats[q.Source.Category] == nil {
			plan.Stats[q.Source.Category] = newCategoryStat()
		}
		plan.Stats[q.Source.Category].Parsed++

		credID, ok := credIDs[credentialFor(q.Source)]
		if !ok {
			plan.Stats[q.Source.Category].Skips["证件缺失"]++
			continue
		}
		kk := credKey{credID, stemHash(q.Content)}

		if rows := existingIdx[credID][kk.hash]; len(rows) > 0 {
			plan.Stats[q.Source.Category].Skips["与存量题重复"]++
			best := rows[0]
			for _, e := range rows[1:] {
				if betterExplanation(e.Explanation, best.Explanation) {
					best = e
				}
			}
			if betterExplanation(q.Explanation, best.Explanation) {
				plan.Enriches = append(plan.Enriches, EnrichAction{Existing: best, NewExpl: q.Explanation, Source: q.Source, Line: q.Line})
			}
			continue
		}
		if idx, ok := importedIdx[kk]; ok {
			plan.Stats[q.Source.Category].Skips["导入内部重复"]++
			if betterExplanation(q.Explanation, plan.Inserts[idx].Explanation) {
				plan.Inserts[idx] = *q
			}
			continue
		}
		importedIdx[kk] = len(plan.Inserts)
		plan.Inserts = append(plan.Inserts, *q)
		plan.Stats[q.Source.Category].Valid++
	}

	// 存量内部重复：保留解析最优（同分取小 id），其余写入阶段按引用处置。
	for _, byHash := range existingIdx {
		for _, rows := range byHash {
			if len(rows) < 2 {
				continue
			}
			keep := rows[0]
			for _, e := range rows[1:] {
				if betterExplanation(e.Explanation, keep.Explanation) ||
					(len([]rune(e.Explanation)) == len([]rune(keep.Explanation)) && e.ID < keep.ID) {
					keep = e
				}
			}
			for _, e := range rows {
				if e.ID != keep.ID {
					plan.ExistingDD = append(plan.ExistingDD, ExistingDedupAction{Keep: keep, Remove: e})
				}
			}
		}
	}
	sort.Slice(plan.ExistingDD, func(i, j int) bool { return plan.ExistingDD[i].Remove.ID < plan.ExistingDD[j].Remove.ID })

	// 同一存量题可能命中多条重复导入，仅保留解析最优的一条更新。
	bestEnrich := map[int]int{} // 存量题 id → plan.Enriches 下标
	for _, en := range plan.Enriches {
		if idx, ok := bestEnrich[en.Existing.ID]; ok {
			if betterExplanation(en.NewExpl, plan.Enriches[idx].NewExpl) {
				plan.Enriches[idx] = en
			}
			continue
		}
		bestEnrich[en.Existing.ID] = len(plan.Enriches)
		plan.Enriches = append(plan.Enriches, en)
	}
	return plan
}

// betterExplanation 解析质量比较：空/占位/模板话术不算有内容，长者优先。
func betterExplanation(cand, cur string) bool {
	cand, cur = strings.TrimSpace(cand), strings.TrimSpace(cur)
	if cand == "" || isMissingAnswer(cand) || isTemplateExplanation(cand) {
		return false
	}
	if cur == "" || isMissingAnswer(cur) || isTemplateExplanation(cur) {
		return true
	}
	return len([]rune(cand)) > len([]rune(cur))
}

// questionReferenced 检查存量题是否被练习记录/错题本/模拟考引用。
func questionReferenced(ctx context.Context, tx dbtx, id int) (bool, error) {
	var ref bool
	err := tx.QueryRow(ctx, `
		SELECT EXISTS(SELECT 1 FROM question_practice_record WHERE question_id = $1)
		    OR EXISTS(SELECT 1 FROM wrong_question WHERE question_id = $1)
		    OR EXISTS(
		        SELECT 1 FROM mock_exam
		        WHERE jsonb_typeof(question_ids) = 'array'
		          AND EXISTS(
		              SELECT 1
		              FROM jsonb_array_elements_text(question_ids) AS e(value)
		              WHERE e.value ~ '^\d+$' AND e.value::int = $1
		          )
		    )`, id).Scan(&ref)
	return ref, err
}

// QuestionApplyResult 写入统计。
type QuestionApplyResult struct {
	Inserted        int
	Tagged          int
	Enriched        int
	RemovedExisting int
	KeptReferenced  int
}

// dbtx 事务内外的 SQL 执行面（*pgxpool.Pool 与 pgx.Tx 均满足）。
type dbtx interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// ApplyQuestionPlan 写入阶段（单事务，all-or-nothing）：插入新题、就地更新存量解析、
// 处置存量内部重复、挂真题标签。
func ApplyQuestionPlan(ctx context.Context, pool *pgxpool.Pool, plan *QuestionPlan, credIDs map[string]int) (QuestionApplyResult, error) {
	var res QuestionApplyResult
	tx, err := pool.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	tagID, err := ensureRealExamTag(ctx, tx)
	if err != nil {
		return res, err
	}
	for i := range plan.Inserts {
		q := &plan.Inserts[i]
		credID, ok := credIDs[credentialFor(q.Source)]
		if !ok {
			return res, fmt.Errorf("证件 code %s 不存在", credentialFor(q.Source))
		}
		var options any
		if q.Options != nil {
			b, err := json.Marshal(q.Options)
			if err != nil {
				return res, fmt.Errorf("序列化选项失败: %w", err)
			}
			options = b
		}
		var newID int
		err := tx.QueryRow(ctx, `
			INSERT INTO question (type, content, options, answer, explanation, score, status, credential_id, created_by, created_by_type)
			VALUES ($1, $2, $3, $4, NULLIF($5, ''), $6, 'published', $7, NULL, 'admin')
			RETURNING id`,
			q.Type, q.Content, options, q.Answer, strings.TrimSpace(q.Explanation), importScore[q.Type], credID).Scan(&newID)
		if err != nil {
			return res, fmt.Errorf("插入题目失败（%s#%d）: %w", q.Source.Name, q.Line, err)
		}
		res.Inserted++
		if q.Source.RealExam {
			if _, err := tx.Exec(ctx, `
				INSERT INTO question_tag_relation (question_id, tag_id)
				VALUES ($1, $2) ON CONFLICT DO NOTHING`, newID, tagID); err != nil {
				return res, fmt.Errorf("挂真题标签失败: %w", err)
			}
			res.Tagged++
		}
	}
	for _, en := range plan.Enriches {
		ct, err := tx.Exec(ctx, `
			UPDATE question SET explanation = $1, updated_at = now() WHERE id = $2`,
			strings.TrimSpace(en.NewExpl), en.Existing.ID)
		if err != nil {
			return res, fmt.Errorf("更新存量题 %d 解析失败: %w", en.Existing.ID, err)
		}
		res.Enriched += int(ct.RowsAffected())
	}
	for i := range plan.ExistingDD {
		dd := &plan.ExistingDD[i]
		ref, err := questionReferenced(ctx, tx, dd.Remove.ID)
		if err != nil {
			return res, fmt.Errorf("检查题目 %d 引用失败: %w", dd.Remove.ID, err)
		}
		if ref {
			dd.Referenced = true
			res.KeptReferenced++
			continue
		}
		ct, err := tx.Exec(ctx, `DELETE FROM question WHERE id = $1`, dd.Remove.ID)
		if err != nil {
			return res, fmt.Errorf("删除重复存量题 %d 失败: %w", dd.Remove.ID, err)
		}
		res.RemovedExisting += int(ct.RowsAffected())
	}
	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("提交事务失败: %w", err)
	}
	return res, nil
}

// ensureRealExamTag 幂等创建真题标签并返回 id。
func ensureRealExamTag(ctx context.Context, tx dbtx) (int, error) {
	var id int
	err := tx.QueryRow(ctx, `
		INSERT INTO question_tag (code, name, description, sort_order, status)
		VALUES ($1, '真题', '来源于历年真题/考场原卷的题目（导入管线自动标记）', 8, 1)
		ON CONFLICT (code) DO UPDATE SET updated_at = now()
		RETURNING id`, realExamTagCode).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("创建真题标签失败: %w", err)
	}
	return id, nil
}

// ResolveCredentialIDs 读取全部证件 code → id 映射。
func ResolveCredentialIDs(ctx context.Context, pool *pgxpool.Pool) (map[string]int, error) {
	rows, err := pool.Query(ctx, `SELECT code, id FROM credential`)
	if err != nil {
		return nil, fmt.Errorf("读取证件失败: %w", err)
	}
	defer rows.Close()
	out := map[string]int{}
	for rows.Next() {
		var code string
		var id int
		if err := rows.Scan(&code, &id); err != nil {
			return nil, err
		}
		out[code] = id
	}
	return out, rows.Err()
}
