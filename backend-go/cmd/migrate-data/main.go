// Package main 是 PostgreSQL→PostgreSQL 一次性数据迁移 CLI 入口。
// 将 Python 版生产 PostgreSQL 库的数据迁移至 Go 版 PostgreSQL schema。
//
// 用法:
//
//	go run ./cmd/migrate-data -source "postgres://user:pass@host:5432/forklift_training" -target "postgres://..."
//
// 前置条件: 目标 PostgreSQL 已执行 migrate up 创建 schema。
// 迁移顺序遵循外键依赖，分批 commit（每 1000 行），含行数校验与序列重置。
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	_ "github.com/lib/pq"
)

// tableSpec 定义一张表的迁移规格。
type tableSpec struct {
	Name    string   // 表名（源与目标 PG 一致）
	Columns []string // 显式列名列表（按 Go 版 DDL 顺序）
}

// migrationOrder 按外键依赖顺序排列的全部 19 张表。
// 父表在前，子表在后；自引用表允许 NULL 外键可提前。
var migrationOrder = []tableSpec{
	{Name: "student", Columns: []string{"student_id", "username", "password", "name", "status", "level", "level_updated_at", "created_at"}},
	{Name: "admin", Columns: []string{"admin_id", "username", "password", "name", "created_at"}},
	{Name: "tutor", Columns: []string{"tutor_id", "username", "password", "name", "status", "created_at"}},
	{Name: "course", Columns: []string{"course_id", "name", "category", "description", "cover_image", "duration", "status", "created_at"}},
	{Name: "chapter", Columns: []string{"chapter_id", "course_id", "title", "content", "content_url", "content_type", "file_url", "description", "duration", "order_num", "created_at"}},
	{Name: "chapter_file", Columns: []string{"file_id", "chapter_id", "file_url", "file_name", "content_type", "file_size", "created_at"}},
	{Name: "knowledge_point", Columns: []string{"id", "name", "level", "parent_id", "description", "created_at"}},
	{Name: "question", Columns: []string{"id", "type", "level", "content", "options", "answer", "explanation", "image_url", "reference_answer", "scoring_criteria", "score", "knowledge_point_id", "status", "created_by", "created_by_type", "created_at", "updated_at"}},
	{Name: "exam_session", Columns: []string{"id", "name", "level", "start_time", "end_time", "duration", "status", "created_by", "question_config", "total_score", "pass_score", "created_at", "updated_at"}},
	{Name: "study_record", Columns: []string{"record_id", "student_id", "course_id", "chapter_id", "study_duration", "progress", "study_date"}},
	{Name: "exam_record", Columns: []string{"exam_id", "student_id", "course_id", "score", "answers", "exam_date"}},
	{Name: "exam_participant", Columns: []string{"id", "exam_session_id", "student_id", "status", "start_time", "submit_time", "remaining_time", "score", "objective_score", "subjective_score", "is_passed", "answers_snapshot", "question_ids", "created_at"}},
	{Name: "exam_answer", Columns: []string{"id", "exam_participant_id", "question_id", "user_answer", "is_correct", "score", "grader_id", "graded_at", "grading_comment", "ai_score", "ai_comment", "ai_graded_at"}},
	{Name: "question_practice_record", Columns: []string{"id", "student_id", "question_id", "level", "is_correct", "practice_type", "user_answer", "created_at"}},
	{Name: "practice_record", Columns: []string{"record_id", "student_id", "practice_type", "duration", "score", "operations", "status", "difficulty", "scenario_id", "time_limit", "correct_parts", "wrong_attempts", "created_at"}},
	{Name: "wrong_question", Columns: []string{"id", "student_id", "question_id", "wrong_count", "last_wrong_at", "is_removed", "created_at"}},
	{Name: "mock_exam", Columns: []string{"id", "student_id", "level", "question_ids", "answers", "start_time", "submit_time", "remaining_time", "duration", "score", "status", "result", "created_at"}},
	{Name: "ai_generation_log", Columns: []string{"log_id", "user_id", "user_type", "generation_type", "input_params", "output_result", "status", "created_at"}},
	{Name: "async_task", Columns: []string{"id", "task_type", "status", "payload", "result", "error", "created_at", "updated_at"}},
}

// identityColumns 每张表使用 GENERATED ALWAYS AS IDENTITY 的主键列，迁移后需重置序列。
var identityColumns = map[string]string{
	"student":                  "student_id",
	"admin":                    "admin_id",
	"tutor":                    "tutor_id",
	"course":                   "course_id",
	"chapter":                  "chapter_id",
	"chapter_file":             "file_id",
	"knowledge_point":          "id",
	"question":                 "id",
	"exam_session":             "id",
	"study_record":             "record_id",
	"exam_record":              "exam_id",
	"exam_participant":         "id",
	"exam_answer":              "id",
	"question_practice_record": "id",
	"practice_record":          "record_id",
	"wrong_question":           "id",
	"mock_exam":                "id",
	"ai_generation_log":        "log_id",
	"async_task":               "id",
}

const batchSize = 1000

func main() {
	source := flag.String("source", "", "源 PostgreSQL DSN，如 postgres://forklift:pass@host:5432/forklift_training?sslmode=disable")
	target := flag.String("target", os.Getenv("DATABASE_URL"), "目标 PostgreSQL DSN（默认读取 DATABASE_URL）")
	dryRun := flag.Bool("dry-run", false, "仅校验行数，不写入数据")
	flag.Parse()

	if *source == "" {
		log.Fatal("请通过 -source 指定源 PostgreSQL DSN")
	}
	if *target == "" {
		log.Fatal("请通过 -target 或 DATABASE_URL 指定目标 PostgreSQL DSN")
	}

	fmt.Println("===== PostgreSQL → PostgreSQL 数据迁移工具 =====")
	fmt.Printf("源:   %s\n", maskDSN(*source))
	fmt.Printf("目标: %s\n", maskDSN(*target))
	if *dryRun {
		fmt.Println("模式: dry-run（仅校验行数）")
	}
	fmt.Println()

	srcDB, err := sql.Open("postgres", *source)
	if err != nil {
		log.Fatalf("打开源 PostgreSQL 连接失败: %v", err)
	}
	defer srcDB.Close()
	srcDB.SetMaxOpenConns(4)

	tgtDB, err := sql.Open("postgres", *target)
	if err != nil {
		log.Fatalf("打开目标 PostgreSQL 连接失败: %v", err)
	}
	defer tgtDB.Close()
	tgtDB.SetMaxOpenConns(4)

	if err := srcDB.Ping(); err != nil {
		log.Fatalf("源 PostgreSQL ping 失败: %v", err)
	}
	if err := tgtDB.Ping(); err != nil {
		log.Fatalf("目标 PostgreSQL ping 失败: %v", err)
	}
	fmt.Println("✓ 双端 PostgreSQL 连接成功")
	fmt.Println()

	start := time.Now()
	var totalMigrated, totalSkipped int64

	for _, t := range migrationOrder {
		srcCount, err := countRows(srcDB, t.Name)
		if err != nil {
			log.Fatalf("统计 %s 源行数失败: %v", t.Name, err)
		}
		tgtCount, err := countRows(tgtDB, t.Name)
		if err != nil {
			log.Fatalf("统计 %s 目标行数失败: %v", t.Name, err)
		}

		if tgtCount > 0 {
			fmt.Printf("⏭  %-28s 源=%-8d 目标=%-8d  跳过（目标非空）\n", t.Name, srcCount, tgtCount)
			totalSkipped += srcCount
			continue
		}

		if *dryRun {
			fmt.Printf("🔍 %-28s 源=%-8d  dry-run\n", t.Name, srcCount)
			continue
		}

		migrated, err := migrateTable(srcDB, tgtDB, t)
		if err != nil {
			log.Fatalf("迁移 %s 失败: %v", t.Name, err)
		}
		totalMigrated += migrated
		fmt.Printf("✓  %-28s %-8d 行  (%.1fs)\n", t.Name, migrated, time.Since(start).Seconds())
	}

	if !*dryRun && totalMigrated > 0 {
		fmt.Println()
		fmt.Println("重置 identity 序列...")
		if err := resetSequences(tgtDB); err != nil {
			log.Printf("⚠ 序列重置失败（不影响数据，但新插入可能主键冲突）: %v", err)
		} else {
			fmt.Println("✓ 序列重置完成")
		}
	}

	fmt.Println()
	fmt.Printf("===== 迁移完成 =====\n")
	fmt.Printf("总迁移行数: %d\n", totalMigrated)
	if totalSkipped > 0 {
		fmt.Printf("跳过行数:   %d（目标表非空）\n", totalSkipped)
	}
	fmt.Printf("耗时:       %.1fs\n", time.Since(start).Seconds())

	// 最终行数校验
	fmt.Println()
	fmt.Println("===== 行数校验 =====")
	allMatch := true
	for _, t := range migrationOrder {
		srcCount, _ := countRows(srcDB, t.Name)
		tgtCount, _ := countRows(tgtDB, t.Name)
		status := "✓"
		if srcCount != tgtCount {
			status = "✗"
			allMatch = false
		}
		fmt.Printf("  %s %-28s 源=%-8d 目标=%-8d\n", status, t.Name, srcCount, tgtCount)
	}
	if !allMatch {
		os.Exit(1)
	}
}

// migrateTable 分批迁移单张表，返回已迁移行数。
func migrateTable(srcDB, tgtDB *sql.DB, t tableSpec) (int64, error) {
	cols := t.Columns
	colList := strings.Join(cols, ", ")
	placeholders := make([]string, len(cols))
	for i := range cols {
		placeholders[i] = fmt.Sprintf("$%d", i+1)
	}
	// OVERRIDING SYSTEM VALUE 允许显式插入 IDENTITY 列的值（保留源 PostgreSQL 原始主键）
	insertSQL := fmt.Sprintf(
		"INSERT INTO %s (%s) OVERRIDING SYSTEM VALUES VALUES (%s)",
		t.Name, colList, strings.Join(placeholders, ", "),
	)

	selectSQL := fmt.Sprintf("SELECT %s FROM %s", colList, t.Name)
	rows, err := srcDB.Query(selectSQL)
	if err != nil {
		return 0, fmt.Errorf("查询源 %s 失败: %w", t.Name, err)
	}
	defer rows.Close()

	txn, err := tgtDB.Begin()
	if err != nil {
		return 0, fmt.Errorf("开启事务失败: %w", err)
	}
	stmt, err := txn.Prepare(insertSQL)
	if err != nil {
		_ = txn.Rollback()
		return 0, fmt.Errorf("预编译插入语句失败: %w", err)
	}
	defer stmt.Close()

	var total, batchCount int64
	for rows.Next() {
		values := make([]interface{}, len(cols))
		valuePtrs := make([]interface{}, len(cols))
		for i := range values {
			valuePtrs[i] = &values[i]
		}
		if err := rows.Scan(valuePtrs...); err != nil {
			_ = txn.Rollback()
			return total, fmt.Errorf("扫描源行失败: %w", err)
		}

		// 转换源 PG 类型为目标 PG 兼容值
		for i, v := range values {
			values[i] = convertValue(v, cols[i])
		}

		if _, err := stmt.Exec(values...); err != nil {
			_ = txn.Rollback()
			return total, fmt.Errorf("插入目标行失败（表 %s）: %w", t.Name, err)
		}
		total++
		batchCount++

		if batchCount >= batchSize {
			if err := txn.Commit(); err != nil {
				return total, fmt.Errorf("提交批次失败: %w", err)
			}
			txn, err = tgtDB.Begin()
			if err != nil {
				return total, fmt.Errorf("重新开启事务失败: %w", err)
			}
			stmt, err = txn.Prepare(insertSQL)
			if err != nil {
				_ = txn.Rollback()
				return total, fmt.Errorf("重新预编译失败: %w", err)
			}
			batchCount = 0
		}
	}
	if err := rows.Err(); err != nil {
		_ = txn.Rollback()
		return total, fmt.Errorf("遍历源行失败: %w", err)
	}

	if batchCount > 0 {
		if err := txn.Commit(); err != nil {
			return total, fmt.Errorf("提交最后批次失败: %w", err)
		}
	}
	return total, nil
}

// convertValue 将源 PostgreSQL 驱动返回的值转换为目标 PG 兼容值。
// PostgreSQL→PostgreSQL 迁移中，源驱动返回的类型已与目标兼容，
// 仅需处理 []byte（JSONB 列）→ string 以确保 JSONB 插入正确。
func convertValue(v interface{}, colName string) interface{} {
	if v == nil {
		return nil
	}
	switch val := v.(type) {
	case []byte:
		// PG JSONB 列通过 lib/pq 返回 []byte，目标 JSONB 接受 string 或 []byte
		return string(val)
	case time.Time:
		// PG TIMESTAMPTZ 直接接受 time.Time
		return val
	case int64, float64, bool, string:
		return val
	default:
		return val
	}
}

// countRows 返回指定表的行数。
func countRows(db *sql.DB, table string) (int64, error) {
	var count int64
	err := db.QueryRow(fmt.Sprintf("SELECT COUNT(*) FROM %s", table)).Scan(&count)
	return count, err
}

// resetSequences 将每张表的 IDENTITY 序列重置为 MAX(主键)+1，确保后续插入不冲突。
func resetSequences(tgtDB *sql.DB) error {
	for table, pkCol := range identityColumns {
		// PG IDENTITY 列对应的序列名格式: <table>_<col>_seq
		seqName := fmt.Sprintf("%s_%s_seq", table, pkCol)
		// setval 到当前最大值，下次 nextval 将返回 max+1
		_, err := tgtDB.Exec(fmt.Sprintf(
			"SELECT setval('%s', COALESCE((SELECT MAX(%s) FROM %s), 1), true)",
			seqName, pkCol, table,
		))
		if err != nil {
			return fmt.Errorf("重置 %s 序列失败: %w", seqName, err)
		}
	}
	return nil
}

// maskDSN 隐藏 DSN 中的密码，仅用于日志输出。
func maskDSN(dsn string) string {
	// 处理 postgres://user:pass@host/db 或 postgresql://user:pass@host/db 格式
	for _, prefix := range []string{"postgres://", "postgresql://"} {
		if strings.HasPrefix(dsn, prefix) {
			atIdx := strings.Index(dsn, "@")
			colonIdx := strings.Index(dsn[len(prefix):], ":")
			if atIdx > 0 && colonIdx >= 0 {
				realColon := len(prefix) + colonIdx
				if realColon < atIdx {
					return dsn[:realColon+1] + "****" + dsn[atIdx:]
				}
			}
			return dsn
		}
	}
	return dsn
}
