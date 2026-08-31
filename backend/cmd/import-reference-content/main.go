// 培训参考资料导入工具。
//
// 把 docs/reference/叉车培训资料/Markdown/ 下的外部资料导入系统：
//   - 题库文件 → question 表（按证件分区，published，真题源自动挂「真题」标签）；
//   - 基础知识文章 → 按大纲知识点建 2 门 N1 课程（章节 = 同主题文章合并 Markdown）。
//
// 用法（backend/ 下执行）：
//
//	DATABASE_URL=... go run ./cmd/import-reference-content -mode survey
//	DATABASE_URL=... go run ./cmd/import-reference-content -mode all            # 干跑
//	DATABASE_URL=... go run ./cmd/import-reference-content -mode all -write     # 实际写入
//
// 幂等：题目重复按归一化题干哈希跳过；课程/章节同名跳过。
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"go.uber.org/zap"

	applogger "forklift-training/internal/logger"
	vconfig "forklift-training/internal/valuation/config"
)

func main() {
	var (
		dirFlag   = flag.String("dir", "", "资料 Markdown 根目录（默认自动探测 docs/reference/叉车培训资料/Markdown）")
		mode      = flag.String("mode", "survey", "运行模式：survey | questions | courses | all")
		write     = flag.Bool("write", false, "实际写入数据库（默认干跑）")
		reportPth = flag.String("report", "", "报告输出文件路径（默认仅 stdout）")
		dsnFlag   = flag.String("dsn", "", "PostgreSQL DSN（默认读 DATABASE_URL）")
		minPaperQ = flag.Int("min-paper-questions", 3, "真题卷最少题数，低于该值的碎片预览文件不建卷")
	)
	flag.Parse()

	logger, err := applogger.New(applogger.Config{Level: "info", Format: "console"})
	if err != nil {
		fmt.Fprintln(os.Stderr, "初始化日志失败:", err)
		os.Exit(1)
	}
	defer func() { _ = logger.Sync() }()

	_ = godotenv.Load()
	dir := *dirFlag
	if dir == "" {
		dir, err = resolveMaterialDir()
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
			os.Exit(1)
		}
	}
	dsn := os.Getenv("DATABASE_URL")
	if *dsnFlag != "" {
		dsn = *dsnFlag
	}
	// survey 模式纯文件侧，不需要数据库；其余模式干跑也需要连库做存量比对。
	if *mode != "survey" && dsn == "" {
		fmt.Fprintln(os.Stderr, "mode 为 questions/courses/all 时需要 DATABASE_URL（或 -dsn）")
		os.Exit(1)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
	defer cancel()

	var pool *pgxpool.Pool
	if dsn != "" && *mode != "survey" {
		pool, err = vconfig.NewPostgresPool(ctx, dsn, 4, 2, 1800)
		if err != nil {
			logger.Error("连接数据库失败", zap.Error(err))
			os.Exit(1)
		}
		defer pool.Close()
	}

	// 1. 文件发现与分类（只读）。
	files, counts, err := discoverFiles(dir)
	if err != nil {
		logger.Error(err.Error())
		os.Exit(1)
	}

	// 2. 题库解析（只读）。
	var parsed []ParsedQuestion
	var skips []SkipNote
	var qFiles []*SourceFile
	for _, sf := range files {
		if sf.Kind != kindQuestion {
			continue
		}
		qFiles = append(qFiles, sf)
		qs, sk, err := ParseFile(sf)
		if err != nil {
			logger.Warn("解析失败", zap.String("file", sf.Name), zap.Error(err))
			skips = append(skips, SkipNote{Source: sf, Line: 0, Reason: "解析异常: " + err.Error()})
			continue
		}
		parsed = append(parsed, qs...)
		skips = append(skips, sk...)
	}

	// 3. 课程分组（只读）。
	articles := loadArticles(files)
	coursePlans := buildCoursePlans(articles)

	rep := newReport(dir, *mode, *write)
	rep.survey(files, counts)
	rep.questions(parsed, skips, qFiles)
	rep.courses(coursePlans, articles)

	// 4. DB 阶段：干跑出比对计划，-write 落库。
	if pool != nil && (*mode == "questions" || *mode == "all") {
		credIDs, err := ResolveCredentialIDs(ctx, pool)
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
		existing, err := LoadExistingQuestions(ctx, pool)
		if err != nil {
			logger.Error(err.Error())
			os.Exit(1)
		}
		plan := BuildQuestionPlan(parsed, skips, existing, credIDs)
		rep.merge(plan, len(existing), credIDs)
		rep.papersPlan(plan, *minPaperQ)
		if *write {
			res, err := ApplyQuestionPlan(ctx, pool, plan, credIDs)
			if err != nil {
				logger.Error("题库写入失败", zap.Error(err))
				os.Exit(1)
			}
			rep.questionApply(res)
			paperRes, err := ApplyPapers(ctx, pool, res.FileQuestions, credIDs, *minPaperQ)
			if err != nil {
				logger.Error("真题卷写入失败", zap.Error(err))
				os.Exit(1)
			}
			rep.paperApply(paperRes)
		}
	}
	if pool != nil && *write && (*mode == "courses" || *mode == "all") {
		res, err := ApplyCoursePlans(ctx, pool, coursePlans)
		if err != nil {
			logger.Error("课程写入失败", zap.Error(err))
			os.Exit(1)
		}
		rep.courseApply(res)
	}

	out := rep.String()
	fmt.Println(out)
	if *reportPth != "" {
		if err := os.WriteFile(*reportPth, []byte(out), 0o644); err != nil {
			logger.Error("写入报告文件失败", zap.Error(err))
			os.Exit(1)
		}
		fmt.Printf("报告已写入 %s\n", *reportPth)
	}
	if *write {
		fmt.Println("⚠️ 本次为写入模式（-write），数据库已变更。")
	} else {
		fmt.Println("本次为干跑（未写库）。确认无误后加 -write 执行。")
	}
}

// resolveMaterialDir 从常见工作目录探测资料根目录。
func resolveMaterialDir() (string, error) {
	candidates := []string{
		"../docs/reference/叉车培训资料/Markdown",
		"docs/reference/叉车培训资料/Markdown",
	}
	for _, c := range candidates {
		if st, err := os.Stat(c); err == nil && st.IsDir() {
			return filepath.Abs(c)
		}
	}
	return "", fmt.Errorf("未找到资料目录，请用 -dir 指定 docs/reference/叉车培训资料/Markdown")
}
