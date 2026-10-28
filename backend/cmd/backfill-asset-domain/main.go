// Command backfill-asset-domain 把库里写死的资源 URL 从旧域改写到资源域（票 #1742）。
//
// 背景：对象存储的公开 URL 此前全部落在站点域（`https://www.gccsmile.com/<key>`）。
// 官网迁 EdgeOne 后站点域退为「301 + API」，资源改挂独立资源域（`cdn.<domain>`），
// 后端公开 URL 基址（R2_PUBLIC_DOMAIN）随之切换 —— 但库里已存的绝对 URL 不会自己变，
// 本命令就是那次一次性回填。
//
// 口径：
//   - 只换「scheme + host」前缀，路径 / 中文 / 空格原样保留（现网确实有带空格的文件名，
//     不要顺手 URL 编码 —— 那会把能用的链接改成另一种形态）；
//   - 只改登记过的列（清单在 defaultTargets），不全库瞎扫；
//   - 默认 dry-run，只报数不落库；-write 才改；
//   - 整轮回填在**一个事务**里：中途失败即回滚，不会留下「改了一半」的库；
//   - 幂等：改写后不再含旧前缀，第二遍命中 0 行；
//   - 反向回退 = **对调 -from 与 -to**（只改 -to 会撞上「前后缀相同」的拒绝）。
//
// 用例与覆盖锁见 main_test.go（清单必须盖住 model 包里每个 URL 形态的「表.列」，
// 且列的 text/jsonb 形态要与 model 的 gorm tag 对得上）。
package main

import (
	"errors"
	"flag"
	"fmt"
	"net/url"
	"os"
	"regexp"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// 默认前缀。两处 host 与 frontend/nginx-host.conf 的域名口径同源：
// 旧域是那张模板里 www.<DOMAIN> 的实例，资源域是 cdn.<DOMAIN> 的实例。
// 换域 / 预发时用 -from / -to 覆盖，不要改这里的默认值。
const (
	defaultFrom = "https://www.gccsmile.com/"
	defaultTo   = "https://cdn.gccsmile.com/"
)

// columnKind 列的存储形态：决定改写时怎么取「文本面」、改完怎么还原。
type columnKind int

const (
	// kindText 普通文本列（含 JSON 数组字符串，如 ai_chat_messages.images）。
	kindText columnKind = iota
	// kindJSONB JSONB 列：必须以文本面替换再转回，否则 replace() 无类型可匹配。
	kindJSONB
)

func (k columnKind) String() string {
	if k == kindJSONB {
		return "jsonb"
	}
	return "text"
}

// column 一列：列名 + 存储形态。
type column struct {
	Name string
	Kind columnKind
}

// target 一张表上的若干列。
type target struct {
	Table   string
	Columns []column
}

// defaultTargets 回填清单（显式枚举，不全库扫列）。
//
// 覆盖范围 = 「对象存储里会被写成绝对 URL」的全部列 + 可能内嵌图片链接的 Markdown 正文：
// 内容精选（封面 / 正文 / 摘要）、课程封面、章节（正文 / 附件 / 幻灯片）、章节文件、
// 证书模板、题库（题图 / 题干）、学员头像、投稿文件、招聘简历卡（PDF / 工作照 / 证书）、
// 论坛主题与回复（正文 / 图集）、AI 助手消息图片、学员反馈图片（#2070）。
//
// 新增「URL 形态」的列会被 main_test.go 的覆盖锁判红（按 表.列 判，且核对 text/jsonb 形态）：
// 要么加进来，要么在豁免表里写明理由。
var defaultTargets = []target{
	{Table: "featured_content", Columns: []column{
		{Name: "cover_image", Kind: kindText},
		{Name: "content", Kind: kindText},
		{Name: "summary", Kind: kindText},
	}},
	{Table: "course", Columns: []column{
		{Name: "cover_image", Kind: kindText},
	}},
	{Table: "chapter", Columns: []column{
		{Name: "content", Kind: kindText},
		{Name: "file_url", Kind: kindText},
		{Name: "slide_urls", Kind: kindText},
	}},
	{Table: "chapter_file", Columns: []column{
		{Name: "file_url", Kind: kindText},
	}},
	{Table: "certificate_template", Columns: []column{
		{Name: "template_url", Kind: kindText},
	}},
	{Table: "question", Columns: []column{
		{Name: "image_url", Kind: kindText},
		{Name: "content", Kind: kindText},
	}},
	{Table: "hrwai_users", Columns: []column{
		{Name: "avatar_url", Kind: kindText},
	}},
	{Table: "user_contribution_file", Columns: []column{
		{Name: "file_url", Kind: kindText},
	}},
	{Table: "job_cards", Columns: []column{
		{Name: "resume_file_url", Kind: kindText},
		{Name: "photos", Kind: kindJSONB},
		{Name: "resume_certifications", Kind: kindJSONB},
	}},
	{Table: "forum_topics", Columns: []column{
		{Name: "content", Kind: kindText},
		{Name: "images", Kind: kindJSONB},
	}},
	{Table: "forum_replies", Columns: []column{
		{Name: "content", Kind: kindText},
		{Name: "images", Kind: kindJSONB},
	}},
	{Table: "ai_chat_messages", Columns: []column{
		{Name: "images", Kind: kindText},
	}},
	// 反馈图片（#2070 / ADR-0089）：与论坛图片同前缀 images/forum/，换存储域名时同样要回填。
	{Table: "feedback", Columns: []column{
		{Name: "images", Kind: kindJSONB},
	}},
}

// columnStat 单列的统计：命中行数（含旧前缀的行）与改写行数（dry-run 恒为 0）。
type columnStat struct {
	Table   string
	Column  string
	Kind    columnKind
	Matched int64
	Changed int64
}

// identPattern 表名 / 列名的允许形态（清单是常量，这里只是把拼错挡在 SQL 之外）。
var identPattern = regexp.MustCompile(`^[a-z_][a-z0-9_]*$`)

func checkIdent(s string) error {
	if !identPattern.MatchString(s) {
		return fmt.Errorf("非法标识符 %q（只允许小写字母 / 数字 / 下划线）", s)
	}
	return nil
}

// run 按清单逐列替换前缀。write=false 时只统计命中行数（dry-run）。
//
// 用 position(? in …) 而不是 LIKE：旧前缀里将来万一出现 % 或 _ 也不会被当成通配符。
// write=true 时整轮进一个事务：第 k 列失败会整体回滚，返回的 stats 仍带前 k-1 列的读数，
// 供调用方打印「跑到哪一列才失败」（回滚意味着这些行数没有落库，别读成「已改」）。
func run(db *gorm.DB, targets []target, from, to string, write bool) ([]columnStat, error) {
	if from == to {
		return nil, errors.New("from 与 to 相同，拒绝空改写")
	}
	var out []columnStat
	exec := func(tx *gorm.DB) error {
		for _, tg := range targets {
			if err := checkIdent(tg.Table); err != nil {
				return err
			}
			for _, c := range tg.Columns {
				if err := checkIdent(c.Name); err != nil {
					return err
				}
				// JSONB 列以文本面替换再转回：replace() 不接受 jsonb 入参。
				expr, castBack := c.Name, ""
				if c.Kind == kindJSONB {
					expr, castBack = c.Name+"::text", "::jsonb"
				}
				st := columnStat{Table: tg.Table, Column: c.Name, Kind: c.Kind}
				if write {
					q := fmt.Sprintf("UPDATE %s SET %s = replace(%s, ?, ?)%s WHERE position(? in %s) > 0",
						tg.Table, c.Name, expr, castBack, expr)
					res := tx.Exec(q, from, to, from)
					if res.Error != nil {
						return fmt.Errorf("%s.%s 改写失败: %w", tg.Table, c.Name, res.Error)
					}
					st.Matched = res.RowsAffected
					st.Changed = res.RowsAffected
				} else {
					q := fmt.Sprintf("SELECT count(*) FROM %s WHERE position(? in %s) > 0", tg.Table, expr)
					if err := tx.Raw(q, from).Scan(&st.Matched).Error; err != nil {
						return fmt.Errorf("%s.%s 预检失败: %w", tg.Table, c.Name, err)
					}
				}
				out = append(out, st)
			}
		}
		return nil
	}

	var err error
	if write {
		err = db.Transaction(exec)
	} else {
		err = exec(db)
	}
	return out, err
}

func main() {
	dsn := flag.String("dsn", os.Getenv("DATABASE_URL"), "Postgres 连接串（默认读 DATABASE_URL）")
	from := flag.String("from", defaultFrom, "旧前缀（scheme + host + 尾斜杠；反向回退时与 -to 对调）")
	to := flag.String("to", defaultTo, "新前缀（资源域；反向回退时与 -from 对调）")
	write := flag.Bool("write", false, "落库（缺省 dry-run，只报数）")
	flag.Parse()

	if err := realMain(*dsn, *from, *to, *write); err != nil {
		fmt.Fprintf(os.Stderr, "回填失败: %v\n", err)
		os.Exit(1)
	}
}

func realMain(dsn, from, to string, write bool) error {
	if dsn == "" {
		return errors.New("缺少连接串：设置 DATABASE_URL 或传 -dsn")
	}
	if err := validatePrefix(from, to); err != nil {
		return err
	}
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return fmt.Errorf("连接 Postgres 失败: %w", err)
	}

	mode := "dry-run（只报数，不落库）"
	if write {
		mode = "写入（整轮一个事务）"
	}
	fmt.Printf("=== 资源 URL 回填\n    %s\n → %s\n=== 模式：%s\n", from, to, mode)

	stats, err := run(db, defaultTargets, from, to, write)
	printStats(stats)
	if err != nil {
		if write {
			fmt.Fprintf(os.Stderr, "已回滚：上面的读数只表示失败前跑到的那一列，未落库。\n")
		}
		return err
	}
	if write {
		fmt.Printf("\n已落库；重跑本命令应命中 0 行（幂等）。\n")
	} else {
		fmt.Printf("\n以上为「将受影响」的行数；确认无误后加 -write 落库（先备份库）。\n")
	}
	return nil
}

// printStats 打表（失败路径也要打：先看清跑到哪一列，再看错误）。
func printStats(stats []columnStat) {
	if len(stats) == 0 {
		return
	}
	fmt.Printf("%-28s %-24s %-6s %8s %8s\n", "TABLE", "COLUMN", "KIND", "MATCHED", "CHANGED")
	var totalMatched, totalChanged int64
	for _, s := range stats {
		fmt.Printf("%-28s %-24s %-6s %8d %8d\n", s.Table, s.Column, s.Kind, s.Matched, s.Changed)
		totalMatched += s.Matched
		totalChanged += s.Changed
	}
	fmt.Printf("%-28s %-24s %-6s %8d %8d\n", "合计", "", "", totalMatched, totalChanged)
}

// validatePrefix 前后缀必须是绝对 http(s) URL：scheme + host 是这次要替换的部分，
// 不是「http 开头的一串字面量」（HasPrefix("http") 会放行 "httpfoo"）。
func validatePrefix(from, to string) error {
	for _, p := range []string{from, to} {
		u, err := url.Parse(p)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			return fmt.Errorf("前后缀必须是绝对 http(s) URL（实得 %q）", p)
		}
	}
	return nil
}
