// 课程建设管线：基础知识文章按官方大纲知识点分组，建 2 门 N1 课程。
package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

// n1CredentialCode 课程挂载的证件；specialty/level/template 取种子 code。
const (
	n1CredentialCode = "forklift_n1"
	operationCode    = "operation"
	certLevelCode    = "certification"
	certTemplateCode = "FORKLIFT_OPERATION_CERT"
)

// Article 一篇知识文章。
type Article struct {
	Src   *SourceFile
	Title string
	Body  string
	KP    string // frontmatter knowledge_point（大纲章节号开头）
}

// ChapterPlan 章节 = 一组同主题文章的合并 Markdown。
type ChapterPlan struct {
	Course   string
	Title    string
	Articles []Article
	Minutes  int
}

// CoursePlan 课程 = 若干章节。
type CoursePlan struct {
	Name        string
	Description string
	Chapters    []ChapterPlan
}

// 大纲章节号 → 课程 A 章节标题（第一章：场（厂）内机动车辆基础）。
var courseASections = []struct{ Prefix, Title string }{
	{"一（一）", "场车工作原理、结构特点与发展趋势"},
	{"一（二）", "场车分类"},
	{"一（三）", "场车性能与技术参数"},
}

// 二（一）内燃机文章的主题分章关键词（按序首个命中生效）。
var engineBuckets = []struct{ Title, Keywords string }{
	{"使用保养与注意事项", "保养,维护,使用,注意,磨合,寿命,延长"},
	{"故障与维修", "维修,故障,修理,检修,排查,排除,渗漏"},
	{"润滑与冷却系", "润滑,机油,冷却,水箱,散热,水温,油底壳"},
	{"燃油供给系", "燃油,供油,喷油,油泵,化油器,燃料,柴油,汽油,高压油"},
	{"配气与进排气", "配气,气门,进气,排气,增压,涡轮,空滤,空气滤"},
	{"机体与曲柄连杆机构", "活塞,曲柄,连杆,气缸,缸盖,缸体,飞轮,机体,轴瓦"},
	{"点火与电气", "点火,火花塞,蓄电池,起动机,发电机,电气,电路,电瓶"},
	{"总体构造与工作原理", "构造,组成,结构,原理,总体,系统,特点,组成结构"},
	{"性能指标", "性能,参数,指标,功率,扭矩,耗油,热效率"},
}

// loadArticles 从分类结果中提取知识文章（剥 frontmatter 与首个一级标题）。
func loadArticles(files []*SourceFile) []Article {
	var out []Article
	for _, sf := range files {
		if sf.Kind != kindArticle {
			continue
		}
		body, err := readFileBody(sf)
		if err != nil {
			continue
		}
		body = strings.ReplaceAll(body, "\r\n", "\n")
		title := sf.FM["title"]
		var kept []string
		for _, l := range strings.Split(body, "\n") {
			if strings.HasPrefix(l, "# ") {
				if title == "" {
					title = strings.TrimSpace(strings.TrimPrefix(l, "# "))
				}
				continue // 一级标题升级为章节内小节标题，正文剔除
			}
			kept = append(kept, l)
		}
		out = append(out, Article{
			Src:   sf,
			Title: strings.TrimSpace(title),
			Body:  strings.TrimSpace(strings.Join(kept, "\n")),
			KP:    sf.FM["knowledge_point"],
		})
	}
	return out
}

// buildCoursePlans 文章分组：第一章三节 → 课程 A；二（一）按关键词 → 课程 B。
func buildCoursePlans(articles []Article) []CoursePlan {
	var (
		secA     = make([][]Article, len(courseASections))
		eng      = make(map[string][]Article)
		leftover []Article
	)
	for _, a := range articles {
		matched := false
		for i, sec := range courseASections {
			if strings.HasPrefix(a.KP, sec.Prefix) {
				secA[i] = append(secA[i], a)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		if strings.HasPrefix(a.KP, "二（一）") {
			bucket := engineBucketFor(a)
			eng[bucket] = append(eng[bucket], a)
			matched = true
		}
		if !matched {
			leftover = append(leftover, a)
		}
	}

	courseA := CoursePlan{
		Name:        "场（厂）内机动车辆基础",
		Description: "场车的工作原理、结构特点、分类与性能技术参数，对标 N1 考试大纲第一章。",
	}
	for i, sec := range courseASections {
		if len(secA[i]) == 0 {
			continue
		}
		courseA.Chapters = append(courseA.Chapters, ChapterPlan{
			Course: courseA.Name, Title: sec.Title, Articles: secA[i],
			Minutes: estimateMinutes(secA[i]),
		})
	}

	courseB := CoursePlan{
		Name:        "内燃叉车动力装置（内燃机）",
		Description: "内燃机（汽油机、柴油机）的构造、工作原理、性能指标与保养维修，对标 N1 考试大纲第二章。",
	}
	// 按主题桶定义顺序出章（综合知识殿后）。
	for _, b := range engineBuckets {
		arts := eng[b.Title]
		if len(arts) == 0 {
			continue
		}
		courseB.Chapters = append(courseB.Chapters, ChapterPlan{
			Course: courseB.Name, Title: "内燃机·" + b.Title, Articles: arts,
			Minutes: estimateMinutes(arts),
		})
	}
	if misc := eng["综合知识"]; len(misc) > 0 {
		courseB.Chapters = append(courseB.Chapters, ChapterPlan{
			Course: courseB.Name, Title: "内燃机·综合知识", Articles: misc,
			Minutes: estimateMinutes(misc),
		})
	}

	// 未归组文章按数量并入相应课程末章，避免内容静默丢弃。
	if len(leftover) > 0 {
		courseA.Chapters = append(courseA.Chapters, ChapterPlan{
			Course: courseA.Name, Title: "综合知识",
			Articles: leftover, Minutes: estimateMinutes(leftover),
		})
	}
	return []CoursePlan{courseA, courseB}
}

// engineBucketFor 按标题关键词给内燃机文章分桶。
func engineBucketFor(a Article) string {
	hay := a.Src.Name + "\n" + a.Title
	for _, b := range engineBuckets {
		for _, kw := range strings.Split(b.Keywords, ",") {
			if strings.Contains(hay, kw) {
				return b.Title
			}
		}
	}
	return "综合知识"
}

// estimateMinutes 按 400 字/分钟估算章节学习时长（下限 5 分钟）。
func estimateMinutes(arts []Article) int {
	words := 0
	for _, a := range arts {
		words += len([]rune(a.Body))
	}
	m := words / 400
	if m < 5 {
		m = 5
	}
	return m
}

// CourseApplyResult 课程写入统计。
type CourseApplyResult struct {
	CreatedCourses  int
	SkippedCourses  int
	CreatedChapters int
	ArticleCount    int
}

// ApplyCoursePlans 写入课程与章节（幂等：同名课程/同题章节跳过）。
func ApplyCoursePlans(ctx context.Context, pool *pgxpool.Pool, plans []CoursePlan) (CourseApplyResult, error) {
	var res CourseApplyResult
	credentialID, err := lookupID(ctx, pool, "credential", "code", n1CredentialCode)
	if err != nil {
		return res, err
	}
	specialtyID, err := lookupID(ctx, pool, "specialty", "code", operationCode)
	if err != nil {
		return res, err
	}
	levelID, err := lookupID(ctx, pool, "course_level", "code", certLevelCode)
	if err != nil {
		return res, err
	}
	templateID, err := lookupID(ctx, pool, "certificate_template", "code", certTemplateCode)
	if err != nil {
		return res, err
	}

	for _, p := range plans {
		var courseID int
		err := pool.QueryRow(ctx,
			`SELECT course_id FROM course WHERE name = $1 AND credential_id = $2`, p.Name, credentialID).Scan(&courseID)
		switch {
		case err == nil:
			res.SkippedCourses++
		case errors.Is(err, pgx.ErrNoRows):
			var total int
			for _, ch := range p.Chapters {
				total += ch.Minutes
			}
			err := pool.QueryRow(ctx, `
				INSERT INTO course (name, description, duration, specialty_id, level_id, theory_hours, practice_hours,
				                    certificate_template_id, credential_id, is_hot, is_featured, sort_order, status)
				VALUES ($1, $2, $3, $4, $5, $6, 0, $7, $8, FALSE, FALSE, 0, 1)
				RETURNING course_id`,
				p.Name, p.Description, total, specialtyID, levelID, (total+59)/60, templateID, credentialID).Scan(&courseID)
			if err != nil {
				return res, fmt.Errorf("创建课程 %s 失败: %w", p.Name, err)
			}
			res.CreatedCourses++
		default:
			return res, fmt.Errorf("查询课程 %s 失败: %w", p.Name, err)
		}

		for i, ch := range p.Chapters {
			var chapterID int
			err := pool.QueryRow(ctx,
				`SELECT chapter_id FROM chapter WHERE course_id = $1 AND title = $2`, courseID, ch.Title).Scan(&chapterID)
			switch {
			case err == nil:
				continue // 已存在，幂等跳过
			case errors.Is(err, pgx.ErrNoRows):
				content := mergeChapterContent(ch)
				if _, err := pool.Exec(ctx, `
					INSERT INTO chapter (course_id, title, content, content_type, duration, order_num)
					VALUES ($1, $2, $3, 'document', $4, $5)`,
					courseID, ch.Title, content, ch.Minutes, i+1); err != nil {
					return res, fmt.Errorf("创建章节 %s 失败: %w", ch.Title, err)
				}
				res.CreatedChapters++
				res.ArticleCount += len(ch.Articles)
			default:
				return res, fmt.Errorf("查询章节 %s 失败: %w", ch.Title, err)
			}
		}
	}
	return res, nil
}

// mergeChapterContent 同组文章合并为章节 Markdown：文章标题作二级小节。
func mergeChapterContent(ch ChapterPlan) string {
	var b strings.Builder
	for _, a := range ch.Articles {
		b.WriteString("## ")
		b.WriteString(a.Title)
		b.WriteString("\n\n")
		b.WriteString(a.Body)
		b.WriteString("\n\n")
	}
	return strings.TrimSpace(b.String())
}

// lookupID 按 code 查引用表 id（各表主键列名不同）。
func lookupID(ctx context.Context, pool *pgxpool.Pool, table, col, code string) (int, error) {
	pk := map[string]string{"credential": "id", "specialty": "specialty_id", "course_level": "level_id", "certificate_template": "id"}[table]
	var id int
	err := pool.QueryRow(ctx, fmt.Sprintf(`SELECT %s FROM %s WHERE %s = $1`, pk, table, col), code).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("查询 %s.%s=%s 失败: %w", table, col, code, err)
	}
	return id, nil
}
