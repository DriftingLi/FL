// 导入报告生成（Markdown）。
package main

import (
	"fmt"
	"sort"
	"strings"
)

type reportBuilder struct {
	dir       string
	mode      string
	write     bool
	b         strings.Builder
	mergeDone bool
}

func newReport(dir, mode string, write bool) *reportBuilder {
	return &reportBuilder{dir: dir, mode: mode, write: write}
}

func (r *reportBuilder) String() string { return r.b.String() }

func (r *reportBuilder) h2(t string) { fmt.Fprintf(&r.b, "\n## %s\n\n", t) }
func (r *reportBuilder) h3(t string) { fmt.Fprintf(&r.b, "\n### %s\n\n", t) }

func (r *reportBuilder) header() {
	runMode := "干跑"
	if r.write {
		runMode = "写入"
	}
	fmt.Fprintf(&r.b, "# 培训参考资料导入报告\n\n- 资料目录：%s\n- 运行模式：%s（%s）\n", r.dir, r.mode, runMode)
}

// survey 盘点分类结果。
func (r *reportBuilder) survey(files []*SourceFile, counts map[string]int) {
	r.header()
	r.h2("文件盘点分类")
	byCat := map[string][]*SourceFile{}
	for _, f := range files {
		byCat[f.Category] = append(byCat[f.Category], f)
	}
	cats := make([]string, 0, len(byCat))
	for c := range byCat {
		cats = append(cats, c)
	}
	sort.Strings(cats)
	fmt.Fprintln(&r.b, "| 分类 | 题库文件 | 知识文章 | 跳过 |")
	fmt.Fprintln(&r.b, "|---|---|---|---|")
	var totQ, totA, totO int
	for _, c := range cats {
		nq, na, no := counts[c+"|"+kindQuestion], counts[c+"|"+kindArticle], counts[c+"|"+kindOther]
		totQ, totA, totO = totQ+nq, totA+na, totO+no
		fmt.Fprintf(&r.b, "| %s | %d | %d | %d |\n", c, nq, na, no)
	}
	fmt.Fprintf(&r.b, "| **合计** | %d | %d | %d |\n", totQ, totA, totO)

	// 跳过文件明细（按分类+文件名排序）。
	var others []*SourceFile
	for _, f := range files {
		if f.Kind == kindOther {
			others = append(others, f)
		}
	}
	if len(others) > 0 {
		r.h3("跳过文件明细")
		for _, f := range others {
			fmt.Fprintf(&r.b, "- `%s/%s` — %s\n", f.Category, f.Name, f.Reason)
		}
	}

	// 真题源清单。
	var realExams []*SourceFile
	for _, f := range files {
		if f.RealExam && (f.Kind == kindQuestion) {
			realExams = append(realExams, f)
		}
	}
	r.h2("真题源识别")
	if len(realExams) == 0 {
		fmt.Fprintln(&r.b, "未识别到真题源文件。")
		return
	}
	fmt.Fprintf(&r.b, "按 frontmatter `material_type=真题` 或文件名含「真题/历年」识别，共 %d 个文件：\n\n", len(realExams))
	for _, f := range realExams {
		fmt.Fprintf(&r.b, "- `%s/%s`\n", f.Category, f.Name)
	}
}

// questions 题库解析统计（文件侧，不含 DB 比对）。qFiles 为全部题库类文件。
func (r *reportBuilder) questions(parsed []ParsedQuestion, skips []SkipNote, qFiles []*SourceFile) {
	r.h2("题库解析统计")
	if len(parsed) == 0 && len(skips) == 0 {
		fmt.Fprintln(&r.b, "未解析到题目。")
		return
	}
	typeStat := map[string]int{}
	for _, q := range parsed {
		typeStat[q.Type]++
	}
	fmt.Fprintf(&r.b, "解析出题目 %d 道（校验前）；按题型：单选 %d、多选 %d、判断 %d；校验跳过 %d。\n",
		len(parsed), typeStat["single_choice"], typeStat["multi_choice"], typeStat["true_false"], len(skips))

	// 跳过原因聚合 + 每类样例。
	byReason := map[string][]SkipNote{}
	for _, s := range skips {
		byReason[s.Reason] = append(byReason[s.Reason], s)
	}
	reasons := make([]string, 0, len(byReason))
	for k := range byReason {
		reasons = append(reasons, k)
	}
	sort.Strings(reasons)
	if len(reasons) > 0 {
		r.h3("校验跳过明细（按原因）")
		for _, reason := range reasons {
			notes := byReason[reason]
			fmt.Fprintf(&r.b, "- **%s**：%d 道\n", reason, len(notes))
			limit := 3
			if len(notes) < limit {
				limit = len(notes)
			}
			for _, n := range notes[:limit] {
				fmt.Fprintf(&r.b, "  - `%s`#%d：%s\n", n.Source.Name, n.Line, n.Preview)
			}
		}
	}

	// 解析产出为 0 的题库文件（既无题目也无跳过记录 → 排版待排查）。
	seen := map[string]bool{}
	for _, q := range parsed {
		seen[q.Source.Path] = true
	}
	for _, s := range skips {
		if s.Source != nil {
			seen[s.Source.Path] = true
		}
	}
	var zeroFiles []string
	for _, sf := range qFiles {
		if !seen[sf.Path] {
			zeroFiles = append(zeroFiles, sf.Path)
		}
	}
	sort.Strings(zeroFiles)
	if len(zeroFiles) == 0 {
		fmt.Fprintln(&r.b, "无（所有题库文件均有解析产出或跳过记录）。")
	} else {
		for _, p := range zeroFiles {
			fmt.Fprintf(&r.b, "- `%s`\n", p)
		}
	}
}

// merge 存量比对（干跑即可产出，需 DB）。
func (r *reportBuilder) merge(plan *QuestionPlan, existingCount int, credIDs map[string]int) {
	r.mergeDone = true
	r.h2("与存量题库比对")
	fmt.Fprintf(&r.b, "存量题总数 %d；导入侧计划新增 %d 道，与存量重复 %d 道，导入内部重复与校验跳过见第三节；存量解析补全 %d 处；存量内部重复组 %d 条。\n",
		existingCount, len(plan.Inserts), sumSkips(plan, "与存量题重复"), len(plan.Enriches), len(plan.ExistingDD))

	r.h3("按证件的新增计划")
	fmt.Fprintln(&r.b, "| 证件 | 新增 |")
	fmt.Fprintln(&r.b, "|---|---|")
	perCred := map[string]int{}
	for _, q := range plan.Inserts {
		perCred[credentialFor(q.Source)]++
	}
	codes := make([]string, 0, len(perCred))
	for c := range perCred {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		fmt.Fprintf(&r.b, "| %s | %d |\n", c, perCred[c])
	}

	if len(plan.Enriches) > 0 {
		r.h3("存量题解析补全明细")
		for _, en := range plan.Enriches {
			fmt.Fprintf(&r.b, "- 存量题 #%d ← `%s`#%d：%s\n", en.Existing.ID, en.Source.Name, en.Line, preview(en.NewExpl))
		}
	}
	if len(plan.ExistingDD) > 0 {
		r.h3("存量内部重复处置")
		for _, dd := range plan.ExistingDD {
			status := "拟删除（无引用）"
			if dd.Referenced {
				status = "保留（有引用）"
			}
			fmt.Fprintf(&r.b, "- 保留 #%d，处置 #%d — %s\n", dd.Keep.ID, dd.Remove.ID, status)
		}
	}
}

func sumSkips(plan *QuestionPlan, reason string) int {
	n := 0
	for _, st := range plan.Stats {
		n += st.Skips[reason]
	}
	return n
}

// papersPlan 真题卷建卷计划（干跑即可产出，需 DB）。
func (r *reportBuilder) papersPlan(plan *QuestionPlan, minQuestions int) {
	r.h2("真题卷建卷计划")
	perFile := map[*SourceFile]int{}
	var files []*SourceFile
	for _, pr := range plan.PaperRefs {
		if perFile[pr.Source] == 0 {
			files = append(files, pr.Source)
		}
		perFile[pr.Source]++
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Path < files[j].Path })
	if len(files) == 0 {
		fmt.Fprintln(&r.b, "无真题源文件命中题目。")
		return
	}
	fmt.Fprintf(&r.b, "真题源文件 %d 个（最少 %d 题建卷）：\n\n", len(files), minQuestions)
	fmt.Fprintln(&r.b, "| 文件 | 卷内题数 | 处置 |")
	fmt.Fprintln(&r.b, "|---|---|---|")
	for _, f := range files {
		n := perFile[f]
		action := "建卷"
		if n < minQuestions {
			action = "跳过（碎片）"
		}
		fmt.Fprintf(&r.b, "| `%s/%s` | %d | %s |\n", f.Category, f.Name, n, action)
	}
}

// paperApply 真题卷写入结果（仅 -write）。
func (r *reportBuilder) paperApply(res PaperApplyResult) {
	r.h2("真题卷写入结果")
	fmt.Fprintf(&r.b, "- 新建卷 %d、更新卷 %d（碎片跳过 %d）；卷题关联 %d 条、题次合计 %d。\n",
		res.Created, res.Updated, res.Skipped, res.Relations, res.TotalCount)
}

// courses 课程分组结果。
func (r *reportBuilder) courses(plans []CoursePlan, articles []Article) {
	r.h2("课程建设计划")
	fmt.Fprintf(&r.b, "知识文章 %d 篇，规划 %d 门课程：\n", len(articles), len(plans))
	for _, p := range plans {
		r.h3(p.Name)
		fmt.Fprintf(&r.b, "%s\n\n", p.Description)
		fmt.Fprintln(&r.b, "| 章节 | 文章数 | 估算时长 |")
		fmt.Fprintln(&r.b, "|---|---|---|")
		for _, ch := range p.Chapters {
			fmt.Fprintf(&r.b, "| %s | %d | %d 分钟 |\n", ch.Title, len(ch.Articles), ch.Minutes)
		}
	}
}

// questionApply 写入结果（仅 -write）。
func (r *reportBuilder) questionApply(res QuestionApplyResult) {
	r.h2("写入结果")
	fmt.Fprintf(&r.b, "- 新增题目：%d（其中真题标签 %d）\n- 存量解析补全：%d\n- 存量重复删除：%d（有引用保留 %d）\n",
		res.Inserted, res.Tagged, res.Enriched, res.RemovedExisting, res.KeptReferenced)
}

// courseApply 课程写入结果（仅 -write）。
func (r *reportBuilder) courseApply(res CourseApplyResult) {
	if !r.mergeDone {
		r.h2("写入结果")
	}
	fmt.Fprintf(&r.b, "- 新建课程：%d（同名跳过 %d）\n- 新建章节：%d（收录文章 %d 篇）\n",
		res.CreatedCourses, res.SkippedCourses, res.CreatedChapters, res.ArticleCount)
}
