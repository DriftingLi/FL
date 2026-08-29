// 题库文件解析：两种排版格式的适配器。
//
// 格式 A（frontmatter 抓取管线产物）：`## 第N题` 块，字段行 `**题型：**`、`**题目：**`、
// `A. xxx`、`**答案：**`、`**解析：**`（答案/解析值可能同行、也可能在下一个非空行）。
// 格式 B（来源ID 管线产物）：`## 第N部分 判断题（共 X 题）` 定题型，条目 `N. 题干` +
// `- A. xxx` 选项 + `**答案：** 正确/A`（部分来源判断题答案为 A/B 字母且无选项，
// 方向不可判别，按跳过处理并在报告中计数）。
package main

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	reQHeader   = regexp.MustCompile(`^##\s*第\d+题`)
	rePartHead  = regexp.MustCompile(`^##\s*第\d+部分\s*([^（(]*)`)
	reTypeLine  = regexp.MustCompile(`^\*\*题型[：:]\s*(.+?)\s*\*\*\s*$`)
	reStemMark  = regexp.MustCompile(`^\*\*题目[：:]\s*\*\*\s*$`)
	reAnswerMk1 = regexp.MustCompile(`^\s*\*\*答案[：:]\s*\*\*\s*(.*)$`)         // **答案：** [值]
	reAnswerMk2 = regexp.MustCompile(`^\s*\*\*答案[：:]\s*(.+?)\s*\*\*\s*$`)     // **答案：值**
	reAnswerTxt = regexp.MustCompile(`^\s*正确答案[：:]\s*(.*)$`)                  // 正确答案：值
	reExplBold  = regexp.MustCompile(`^\s*\*\*解析[：:]\s*(.*?)\s*\*\*\s*$`)     // **解析：[值]**
	reOption    = regexp.MustCompile(`^\s*-{0,2}\s*([A-Za-z])[\.、．]\s*(.*)$`) // A. / - A. / A、
	reInlineExp = regexp.MustCompile(`【解析】`)
	reNumItem   = regexp.MustCompile(`^(\d+)[\.、．]\s*(.*)$`)
)

// ParsedQuestion 解析并通过校验前的中间题目。
type ParsedQuestion struct {
	Type        string // single_choice / multi_choice / true_false
	Content     string
	Options     map[string]string // 判断题为 nil（前端渲染固定对/错模板）
	Answer      string            // 对/错 或字母（串）
	Explanation string
	Source      *SourceFile
	Line        int // 在文件中的 1-based 行号（报告定位用）
}

// SkipNote 被跳过的题目（原因 + 预览），报告中按原因聚合。
type SkipNote struct {
	Source  *SourceFile
	Line    int
	Reason  string
	Preview string
}

const (
	reasonNoAnswer   = "无答案（含\"原资料未提供\"）"
	reasonUnknownTy  = "未知题型（简答/案例等不导入）"
	reasonTFNoOpts   = "判断题字母答案且无选项（方向不可判别）"
	reasonBadOption  = "选项缺失/含空文本"
	reasonAnsNoOpt   = "答案字母不在选项中"
	reasonMultiOne   = "多选题答案不足 2 项"
	reasonEmptyStem  = "题干为空"
	reasonMultiLetts = "单选题答案多于 1 项"
)

// ParseFile 解析单个题库文件，返回题目与跳过明细。
func ParseFile(sf *SourceFile) ([]ParsedQuestion, []SkipNote, error) {
	body, err := readFileBody(sf)
	if err != nil {
		return nil, nil, fmt.Errorf("读取 %s 失败: %w", sf.Path, err)
	}
	return ParseFileWithBody(sf, body)
}

// ParseFileWithBody 解析题库正文（测试与 ParseFile 共用入口）。
func ParseFileWithBody(sf *SourceFile, body string) ([]ParsedQuestion, []SkipNote, error) {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	lines := strings.Split(body, "\n")
	if idx := firstMatchIndex(lines, reQHeader); idx >= 0 {
		return parseFormatA(sf, lines, idx)
	}
	if idx := firstMatchIndex(lines, rePartHead); idx >= 0 {
		return parseFormatB(sf, lines, idx)
	}
	return nil, []SkipNote{{Source: sf, Line: 1, Reason: "未识别的排版（无题号/部分标题）", Preview: ""}}, nil
}

func firstMatchIndex(lines []string, re *regexp.Regexp) int {
	for i, l := range lines {
		if re.MatchString(l) {
			return i
		}
	}
	return -1
}

// parseFormatA 按 `## 第N题` 块切分解析。
func parseFormatA(sf *SourceFile, lines []string, start int) ([]ParsedQuestion, []SkipNote, error) {
	var (
		qs   []ParsedQuestion
		skip []SkipNote
	)
	for i := start; i < len(lines); {
		if !reQHeader.MatchString(lines[i]) {
			i++
			continue
		}
		j := i + 1
		for j < len(lines) && !reQHeader.MatchString(lines[j]) {
			j++
		}
		q, note := parseFormatABlock(sf, lines[i:j], i+1)
		if q != nil {
			qs = append(qs, *q)
		} else if note != nil {
			skip = append(skip, *note)
		}
		i = j
	}
	return qs, skip, nil
}

// parseFormatABlock 解析单个题目块；块不构成可用题目时返回 SkipNote。
func parseFormatABlock(sf *SourceFile, block []string, lineNo int) (*ParsedQuestion, *SkipNote) {
	var (
		q         ParsedQuestion
		opts      = map[string]string{}
		optOrder  []string
		stem      []string
		explParts []string
		inStem    bool
		inExpl    bool
		awaitAns  bool
		awaitExpl bool
	)
	q.Source, q.Line = sf, lineNo
	for _, raw := range block[1:] {
		line := strings.TrimRight(raw, " \t")
		trimmed := strings.TrimSpace(line)
		if trimmed == "---" || strings.HasPrefix(trimmed, "#") {
			break
		}
		if trimmed == "" {
			continue
		}
		switch {
		case reTypeLine.MatchString(trimmed):
			m := reTypeLine.FindStringSubmatch(trimmed)
			q.Type = mapTypeText(m[1])
			inStem, inExpl, awaitExpl = false, false, false
			continue
		case reStemMark.MatchString(trimmed):
			inStem, inExpl, awaitExpl = true, false, false
			continue
		}
		if m := reAnswerMk1.FindStringSubmatch(trimmed); m != nil {
			inStem, inExpl, awaitExpl = false, false, false
			if v := strings.TrimSpace(m[1]); v != "" {
				q.Answer = v
				awaitAns = false
			} else {
				awaitAns = true
			}
			continue
		}
		if strings.Contains(trimmed, "**答案") {
			if m := reAnswerMk2.FindStringSubmatch(trimmed); m != nil {
				q.Answer = m[1]
				awaitAns = false
				continue
			}
		}
		if m := reAnswerTxt.FindStringSubmatch(trimmed); m != nil {
			q.Answer = strings.TrimSpace(m[1])
			awaitAns = false
			continue
		}
		if strings.Contains(trimmed, "**解析") {
			if m := reExplBold.FindStringSubmatch(trimmed); m != nil {
				awaitExpl = false
				inExpl = true
				if v := strings.TrimSpace(m[1]); v != "" && !isMissingAnswer(v) {
					explParts = append(explParts, v)
				}
				continue
			}
		}
		if o, text, ok := matchOption(trimmed); ok {
			inStem, inExpl = false, false
			awaitAns = false
			text = strings.TrimSpace(stripAnswerMarks(text))
			if _, dup := opts[o]; !dup {
				optOrder = append(optOrder, o)
			}
			opts[o] = text
			continue
		}
		switch {
		case awaitAns:
			q.Answer = trimmed
			awaitAns = false
		case awaitExpl:
			explParts = append(explParts, trimmed)
			inExpl = true
		case inStem:
			stem = append(stem, trimmed)
		case inExpl:
			explParts = append(explParts, trimmed)
		}
	}
	q.Content = strings.Join(stem, "\n")
	q.Explanation = strings.Join(explParts, "\n")
	q.Options = buildOptions(optOrder, opts)
	return finishQuestion(&q)
}

// parseFormatB 单遍扫描：部分标题定题型，数字条目累积题干/选项/答案。
func parseFormatB(sf *SourceFile, lines []string, start int) ([]ParsedQuestion, []SkipNote, error) {
	var (
		qs       []ParsedQuestion
		skip     []SkipNote
		cur      *ParsedQuestion
		curOpts  = map[string]string{}
		curOrder []string
		curType  string
		awaitAns bool
		flush    = func() {
			if cur != nil {
				cur.Options = buildOptions(curOrder, curOpts)
				q, note := finishQuestion(cur)
				if q != nil {
					qs = append(qs, *q)
				} else if note != nil {
					skip = append(skip, *note)
				}
			}
			cur, curOpts, curOrder, awaitAns = nil, map[string]string{}, nil, false
		}
	)
	for i := start; i < len(lines); i++ {
		trimmed := strings.TrimSpace(lines[i])
		if trimmed == "" || trimmed == "---" {
			continue
		}
		if m := rePartHead.FindStringSubmatch(trimmed); m != nil && strings.HasPrefix(trimmed, "##") {
			flush()
			curType = mapTypeText(strings.TrimSpace(m[1]))
			continue
		}
		if reQHeader.MatchString(trimmed) {
			flush()
			continue
		}
		if m := reNumItem.FindStringSubmatch(trimmed); m != nil {
			flush()
			cur = &ParsedQuestion{Type: curType, Source: sf, Line: i + 1, Content: strings.TrimSpace(m[2])}
			continue
		}
		if cur == nil {
			continue
		}
		if m := reAnswerMk1.FindStringSubmatch(trimmed); m != nil {
			if v := strings.TrimSpace(m[1]); v != "" {
				cur.Answer = v
			} else {
				awaitAns = true
			}
			continue
		}
		if strings.Contains(trimmed, "**答案") {
			if m := reAnswerMk2.FindStringSubmatch(trimmed); m != nil {
				cur.Answer = m[1]
				awaitAns = false
				continue
			}
		}
		if m := reAnswerTxt.FindStringSubmatch(trimmed); m != nil {
			cur.Answer = strings.TrimSpace(m[1])
			awaitAns = false
			continue
		}
		if o, text, ok := matchOption(trimmed); ok {
			awaitAns = false
			text = strings.TrimSpace(stripAnswerMarks(text))
			if _, dup := curOpts[o]; !dup {
				curOrder = append(curOrder, o)
			}
			curOpts[o] = text
			continue
		}
		if awaitAns {
			cur.Answer = trimmed
			awaitAns = false
			continue
		}
		cur.Content += "\n" + trimmed
	}
	flush()
	return qs, skip, nil
}

// matchOption 匹配选项行，返回 (字母, 选项文本)。
func matchOption(line string) (string, string, bool) {
	m := reOption.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return strings.ToUpper(m[1]), m[2], true
}

// buildOptions 按出现顺序构建选项表；空文本选项视为脏数据（返回 nil 由校验拒绝）。
func buildOptions(order []string, opts map[string]string) map[string]string {
	if len(order) == 0 {
		return nil
	}
	out := make(map[string]string, len(order))
	for _, k := range order {
		out[k] = opts[k]
	}
	return out
}

// finishQuestion 归一化答案并做入库前校验；通过返回题目，否则返回跳过原因。
func finishQuestion(q *ParsedQuestion) (*ParsedQuestion, *SkipNote) {
	fail := func(reason string) (*ParsedQuestion, *SkipNote) {
		return nil, &SkipNote{Source: q.Source, Line: q.Line, Reason: reason, Preview: preview(q.Content)}
	}
	if q.Type == "" {
		return fail(reasonUnknownTy)
	}
	q.Content = strings.TrimSpace(q.Content)
	if q.Content == "" {
		return fail(reasonEmptyStem)
	}
	q.Content, q.Explanation = splitInlineExplanation(q.Content, q.Explanation)
	switch q.Type {
	case "true_false":
		ans, ok := normalizeTFAnswer(q.Answer, q.Options)
		if !ok {
			if isMissingAnswer(q.Answer) {
				return fail(reasonNoAnswer)
			}
			return fail(reasonTFNoOpts)
		}
		q.Answer, q.Options = ans, nil
	case "single_choice":
		if len(q.Options) < 2 {
			return fail(reasonBadOption)
		}
		for _, v := range q.Options {
			if v == "" {
				return fail(reasonBadOption)
			}
		}
		if isMissingAnswer(q.Answer) {
			return fail(reasonNoAnswer)
		}
		ans, ok := normalizeChoiceAnswer(q.Answer, q.Options, false)
		if !ok {
			if letterOnly(q.Answer) {
				return fail(reasonAnsNoOpt)
			}
			return fail(reasonMultiLetts)
		}
		q.Answer = ans
	case "multi_choice":
		if len(q.Options) < 2 {
			return fail(reasonBadOption)
		}
		for _, v := range q.Options {
			if v == "" {
				return fail(reasonBadOption)
			}
		}
		if isMissingAnswer(q.Answer) {
			return fail(reasonNoAnswer)
		}
		ans, ok := normalizeChoiceAnswer(q.Answer, q.Options, true)
		if !ok {
			if len(splitAnswerLetters(q.Answer)) < 2 {
				return fail(reasonMultiOne)
			}
			return fail(reasonAnsNoOpt)
		}
		q.Answer = ans
	default:
		return fail(reasonUnknownTy)
	}
	return q, nil
}

// splitInlineExplanation 把题干内联的「【解析】…」拆到解析列。
func splitInlineExplanation(content, explanation string) (string, string) {
	if !reInlineExp.MatchString(content) {
		return content, explanation
	}
	parts := reInlineExp.Split(content, 2)
	stem := strings.TrimSpace(parts[0])
	inline := strings.TrimSpace(parts[1])
	if explanation == "" && inline != "" {
		explanation = inline
	}
	return stem, explanation
}

// mapTypeText 题型中文 → 系统题型 code；简答/案例等返回空（不导入）。
func mapTypeText(t string) string {
	t = strings.TrimSpace(t)
	switch {
	case strings.Contains(t, "多选"):
		return "multi_choice"
	case strings.Contains(t, "单选"):
		return "single_choice"
	case strings.Contains(t, "判断"):
		return "true_false"
	default:
		return ""
	}
}

// isMissingAnswer 答案列缺失（空或"原资料未提供"等占位）。
func isMissingAnswer(v string) bool {
	v = strings.TrimSpace(v)
	return v == "" || strings.Contains(v, "未提供") || strings.Contains(v, "未包含")
}

// letterOnly 答案是否为纯字母串（区分"字母不在选项中"与"答案格式异常"）。
func letterOnly(v string) bool {
	if v == "" {
		return false
	}
	for _, r := range strings.ToUpper(v) {
		if (r < 'A' || r > 'Z') && r != ',' && r != ' ' {
			return false
		}
	}
	return true
}

func preview(s string) string {
	s = strings.TrimSpace(strings.ReplaceAll(s, "\n", " "))
	if len(s) > 40 {
		r := []rune(s)
		if len(r) > 40 {
			s = string(r[:40]) + "…"
		}
	}
	return s
}
