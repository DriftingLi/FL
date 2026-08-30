// 真题卷写入：按真题源文件的 文件→题目 映射 upsert real_exam_paper（ADR-0022）。
package main

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PaperApplyResult 真题卷写入统计。
type PaperApplyResult struct {
	Created    int
	Updated    int
	Skipped    int // 低于最少题数阈值跳过的文件
	Relations  int
	TotalCount int
}

// ApplyPapers 按文件→题目映射建卷（单事务，幂等：credential_id + source_ref 定位）。
// questionIDs 为 nil 的真题文件视为该卷本轮无可用题目，跳过。
func ApplyPapers(ctx context.Context, pool *pgxpool.Pool, fileQuestions map[*SourceFile][]int, credIDs map[string]int, minQuestions int) (PaperApplyResult, error) {
	var res PaperApplyResult
	if minQuestions < 1 {
		minQuestions = 1
	}

	// 固定处理顺序（map 遍历无序，报告与幂等性依赖稳定顺序）。
	files := make([]*SourceFile, 0, len(fileQuestions))
	for f := range fileQuestions {
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })

	tx, err := pool.Begin(ctx)
	if err != nil {
		return res, fmt.Errorf("开启事务失败: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	for _, f := range files {
		if !f.RealExam {
			continue
		}
		ids := dedupeInts(fileQuestions[f])
		if len(ids) < minQuestions {
			res.Skipped++
			continue
		}
		if err := upsertPaper(ctx, tx, f, ids, credIDs, &res); err != nil {
			return res, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return res, fmt.Errorf("提交事务失败: %w", err)
	}
	return res, nil
}

// upsertPaper 单卷落库：定位 → 更新或创建 → 重建卷题关联 → 同步 question_count。
func upsertPaper(ctx context.Context, tx pgx.Tx, f *SourceFile, ids []int, credIDs map[string]int, res *PaperApplyResult) error {
	credID, ok := credIDs[credentialFor(f)]
	if !ok {
		return fmt.Errorf("证件 code %s 不存在", credentialFor(f))
	}
	title := truncateRunes(paperTitle(f), 200)
	source := truncateRunes(strings.TrimSpace(f.FM["source"]), 100)
	year := paperYear(f)
	sourceRef := filepath.ToSlash(filepath.Join(f.Category, f.Name))

	var paperID int
	err := tx.QueryRow(ctx,
		`SELECT paper_id FROM real_exam_paper WHERE credential_id = $1 AND source_ref = $2`,
		credID, sourceRef).Scan(&paperID)
	switch {
	case err == nil:
		if _, err := tx.Exec(ctx, `
			UPDATE real_exam_paper
			SET title = $1, year = NULLIF($2, 0), source = NULLIF($3, ''),
			    question_count = $4, updated_at = now()
			WHERE paper_id = $5`,
			title, year, source, len(ids), paperID); err != nil {
			return fmt.Errorf("更新真题卷失败（%s）: %w", title, err)
		}
		if _, err := tx.Exec(ctx, `DELETE FROM real_exam_paper_question WHERE paper_id = $1`, paperID); err != nil {
			return fmt.Errorf("清理真题卷关联失败（%s）: %w", title, err)
		}
		res.Updated++
	case isNoRows(err):
		err := tx.QueryRow(ctx, `
			INSERT INTO real_exam_paper (credential_id, title, year, source, duration_minutes, source_ref, question_count, status)
			VALUES ($1, $2, NULLIF($3, 0), NULLIF($4, ''), 90, $5, $6, 1)
			RETURNING paper_id`,
			credID, title, year, source, sourceRef, len(ids)).Scan(&paperID)
		if err != nil {
			return fmt.Errorf("创建真题卷失败（%s）: %w", title, err)
		}
		res.Created++
	default:
		return fmt.Errorf("查询真题卷失败（%s）: %w", title, err)
	}

	for i, qid := range ids {
		if _, err := tx.Exec(ctx, `
			INSERT INTO real_exam_paper_question (paper_id, question_id, order_num)
			VALUES ($1, $2, $3) ON CONFLICT (paper_id, question_id) DO UPDATE SET order_num = EXCLUDED.order_num`,
			paperID, qid, i+1); err != nil {
			return fmt.Errorf("写入真题卷关联失败（%s#%d）: %w", title, i+1, err)
		}
		res.Relations++
	}
	res.TotalCount += len(ids)
	return nil
}

// paperTitle 卷名：frontmatter title 优先，退回文件名（去扩展名）。
func paperTitle(f *SourceFile) string {
	if t := strings.TrimSpace(f.FM["title"]); t != "" {
		return t
	}
	return strings.TrimSuffix(f.Name, filepath.Ext(f.Name))
}

// paperYear 卷年份：frontmatter year 解析失败返回 0（NULL）。
func paperYear(f *SourceFile) int {
	y, _ := strconv.Atoi(strings.TrimSpace(f.FM["year"]))
	return y
}

// dedupeInts 保序去重（同题在卷内只出现一次）。
func dedupeInts(in []int) []int {
	seen := make(map[int]bool, len(in))
	out := make([]int, 0, len(in))
	for _, v := range in {
		if !seen[v] {
			seen[v] = true
			out = append(out, v)
		}
	}
	return out
}

// truncateRunes 按字符数截断（卷名/来源列有长度上限）。
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// isNoRows pgx 无行错误判定。
func isNoRows(err error) bool {
	return err == pgx.ErrNoRows
}
