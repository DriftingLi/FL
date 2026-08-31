// 题干归一化与答案映射：题库导入的共享纯函数。
package main

import (
	"fmt"
	"hash/fnv"
	"sort"
	"strings"
	"unicode"
)

// normalizeStem 题干归一化：仅保留字母/数字（含中文），忽略空白与全部标点，
// 用于跨来源去重比对（全半角、（ ）、空格差异不影响命中）。
func normalizeStem(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// stemHash 归一化题干的 FNV-1a 哈希（hex 字符串）。
func stemHash(s string) string {
	h := fnv.New64a()
	_, _ = h.Write([]byte(normalizeStem(s)))
	return fmt.Sprintf("%016x", h.Sum64())
}

// isTemplateExplanation 存量题的模板话术解析（baseline 批量生成的占位解析）。
func isTemplateExplanation(s string) bool {
	return strings.Contains(s, "请结合教材相应章节理解掌握")
}

// normalizeTFAnswer 判断题答案归一化为「对/错」。
// letter 答案（A/B）无选项时映射方向不可知，返回 false 由调用方跳过。
func normalizeTFAnswer(v string, opts map[string]string) (string, bool) {
	switch strings.TrimSpace(v) {
	case "正确", "对", "√", "T", "t", "Y", "y":
		return "对", true
	case "错误", "错", "×", "x", "X", "F", "f":
		return "错", true
	}
	// 字母答案：若选项文本可判别方向（如 A.正确 B.错误），按选项文本映射。
	if letter := strings.ToUpper(strings.TrimSpace(v)); len(letter) == 1 && opts != nil {
		text, ok := opts[letter]
		if !ok {
			return "", false
		}
		switch strings.TrimSpace(stripAnswerMarks(text)) {
		case "正确", "对":
			return "对", true
		case "错误", "错":
			return "错", true
		}
	}
	return "", false
}

// normalizeChoiceAnswer 选择题答案归一化：单选返回单个大写字母，多选返回排序后
// 逗号分隔字母串（判分 normalizeAnswerList 按逗号拆分，必须存成 "A,C" 而非 "AC"）。
// 所有字母必须存在于选项中。
func normalizeChoiceAnswer(v string, opts map[string]string, multi bool) (string, bool) {
	letters := splitAnswerLetters(v)
	if len(letters) == 0 {
		return "", false
	}
	for _, l := range letters {
		if _, ok := opts[l]; !ok {
			return "", false
		}
	}
	if multi {
		if len(letters) < 2 {
			return "", false
		}
		return strings.Join(letters, ","), true
	}
	if len(letters) != 1 {
		return "", false
	}
	return letters[0], true
}

// splitAnswerLetters 从 "AB"/"A,B"/"A、B"/"A B" 等写法提取大写字母并排序去重。
func splitAnswerLetters(v string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, 4)
	for _, r := range strings.ToUpper(v) {
		if r >= 'A' && r <= 'Z' {
			l := string(r)
			if !seen[l] {
				seen[l] = true
				out = append(out, l)
			}
		}
	}
	sort.Strings(out)
	return out
}

// stripAnswerMarks 去掉选项文本里的正确性标记残留（来源站把正确项标成 "(正确)" 等）。
func stripAnswerMarks(s string) string {
	for _, m := range []string{"(正确)", "（正确）", "【正确】", "[正确]", "(对)", "（对）", "✓", "√"} {
		s = strings.ReplaceAll(s, m, "")
	}
	return s
}
