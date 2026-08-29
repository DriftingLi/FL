// 解析与归一化的单元测试（样例取自真实资料文件的典型排版）。
package main

import (
	"strings"
	"testing"
)

func TestNormalizeStem(t *testing.T) {
	a := stemHash("叉车在行驶过程中，可以超车。（ ）")
	b := stemHash("叉车在行驶过程中 可以超车（）")
	c := stemHash("叉车在行驶过程中，不可以超车。")
	if a != b {
		t.Fatalf("相同语义题干应命中同一哈希: %s != %s", a, b)
	}
	if a == c {
		t.Fatal("不同题干不应命中同一哈希")
	}
}

func TestNormalizeTFAnswer(t *testing.T) {
	cases := []struct {
		v     string
		opts  map[string]string
		want  string
		valid bool
	}{
		{"正确", nil, "对", true},
		{"错误", nil, "错", true},
		{"对", nil, "对", true},
		{"A", map[string]string{"A": "正确", "B": "错误"}, "对", true},
		{"B", map[string]string{"A": "正确", "B": "错误"}, "错", true},
		{"A", nil, "", false}, // 无选项的字母答案：方向不可判别
		{"C", map[string]string{"A": "正确", "B": "错误"}, "", false},
	}
	for _, c := range cases {
		got, ok := normalizeTFAnswer(c.v, c.opts)
		if ok != c.valid || (ok && got != c.want) {
			t.Errorf("normalizeTFAnswer(%q) = (%q, %v), want (%q, %v)", c.v, got, ok, c.want, c.valid)
		}
	}
}

func TestNormalizeChoiceAnswer(t *testing.T) {
	opts := map[string]string{"A": "x", "B": "y", "C": "z", "D": "w"}
	if v, ok := normalizeChoiceAnswer("AC", opts, true); !ok || v != "A,C" {
		t.Errorf("多选 AC 应归一化为 A,C，got (%q, %v)", v, ok)
	}
	if v, ok := normalizeChoiceAnswer("b, a", opts, true); !ok || v != "A,B" {
		t.Errorf("多选乱序应排序去重，got (%q, %v)", v, ok)
	}
	if _, ok := normalizeChoiceAnswer("E", opts, false); ok {
		t.Error("不存在的选项字母应拒绝")
	}
	if _, ok := normalizeChoiceAnswer("A", opts, true); ok {
		t.Error("多选答案不足 2 项应拒绝")
	}
	if _, ok := normalizeChoiceAnswer("AB", opts, false); ok {
		t.Error("单选答案多于 1 项应拒绝")
	}
	if v, ok := normalizeChoiceAnswer("d.", opts, false); !ok || v != "D" {
		t.Errorf("单选 d. 应归一化为 D，got (%q, %v)", v, ok)
	}
}

func TestParseFormatA(t *testing.T) {
	sf := &SourceFile{Name: "样例.md", Category: "N1", RealExam: false}
	body := `
## 第1题

**题型：多选题**

**题目：**

《中华人民共和国特种设备安全法》第七条规定，特种设备生产单位应当建立（）和（）责任制度。

A. 安全
B. 岗位管理制度
C. 节能

**答案：**

AC

**解析：原资料未提供**

---

## 第2题

**题型：判断题**

**题目：**

发动机温度过高会影响发动机的正常工作，而过低则不会影响。

A. 正确
B. 错误

**答案：**

A

**解析：原资料未提供**

---

## 第3题

**题型：单选题**

**题目：**

以下哪种会车避让规则是不正确的？【解析】会车规则：小车让大车错误。

- A. 下坡车让上坡车
- B. 小车让大车
- C. 大车让小车

**答案：** B

**解析：** 会车让行三原则先行考虑安全。
`
	qs, skips, err := ParseFileWithBody(sf, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(skips) != 0 {
		t.Fatalf("不应有跳过: %+v", skips)
	}
	if len(qs) != 3 {
		t.Fatalf("应解析出 3 题，got %d", len(qs))
	}
	if qs[0].Type != "multi_choice" || qs[0].Answer != "A,C" {
		t.Errorf("第1题多选归一化失败: %+v", qs[0])
	}
	if qs[1].Type != "true_false" || qs[1].Answer != "对" || qs[1].Options != nil {
		t.Errorf("第2题判断归一化失败: %+v", qs[1])
	}
	if qs[2].Answer != "B" || !strings.Contains(qs[2].Content, "会车避让规则") {
		t.Errorf("第3题解析失败: %+v", qs[2])
	}
	if strings.Contains(qs[2].Content, "【解析】") {
		t.Error("内联【解析】应从题干剥离")
	}
	if qs[2].Explanation == "" {
		t.Error("内联【解析】应进入解析列")
	}
}

func TestParseFormatB(t *testing.T) {
	sf := &SourceFile{Name: "试卷.md", Category: "五级"}
	body := `
# 试卷

> **来源ID：** 人人文库(renrendoc.com)
> **题目数量：** 共 20 题

## 第1部分 判断题（共 10 题）

1. 叉车操作人员在上岗前，必须接受专业培训。（ ）
   **答案：** 正确

2. 叉车在行驶过程中，可以超车。（ ）
   **答案：** 错误

## 第2部分 单选题（共 10 题）

1. 为避免损伤变速箱齿轮，哪项操作必须在车辆彻底停稳后才能执行？
   - A. 增挡
   - B. 减档
   - C. 挂入倒档
   - D. 变换档位
   **答案：** C

2. 哪种装置具备预防传动系统超载的功能？
   - A. 液力偶合器
   - B. 液力变阻器
   **答案：** A
`
	qs, skips, err := ParseFileWithBody(sf, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(skips) != 0 {
		t.Fatalf("不应有跳过: %+v", skips)
	}
	if len(qs) != 4 {
		t.Fatalf("应解析出 4 题，got %d", len(qs))
	}
	if qs[0].Type != "true_false" || qs[0].Answer != "对" {
		t.Errorf("判断题归一化失败: %+v", qs[0])
	}
	if qs[2].Answer != "C" || qs[2].Options["C"] != "挂入倒档" {
		t.Errorf("单选题归一化失败: %+v", qs[2])
	}
}

func TestParseFormatBSkipsLetterTF(t *testing.T) {
	sf := &SourceFile{Name: "题库.md", Category: "N1"}
	body := `
## 第1部分 判断题（共 2 题）

1. 饮酒驾车易发事故
   **答案：** A

2. 内燃机叉车以蓄电池为工作装置动力
   **答案：** B
`
	qs, skips, err := ParseFileWithBody(sf, body)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 0 {
		t.Fatalf("字母答案的判断题（无选项）应全部跳过，got %d 题", len(qs))
	}
	if len(skips) != 2 || skips[0].Reason != reasonTFNoOpts {
		t.Fatalf("跳过原因应为 %s: %+v", reasonTFNoOpts, skips)
	}
}

func TestBuildCoursePlans(t *testing.T) {
	mk := func(name, kp string) Article {
		return Article{
			Src:   &SourceFile{Name: name, Category: "基础知识", FM: map[string]string{"knowledge_point": kp}},
			Title: strings.TrimSuffix(name, ".md"),
			Body:  "正文内容",
			KP:    kp,
		}
	}
	articles := []Article{
		mk("总体组成.md", "一（一）场（厂）内机动车辆的工作原理、结构特点及发展趋势"),
		mk("场车分类.md", "一（二）场（厂）内机动车辆的分类"),
		mk("性能参数.md", "一（三）场（厂）内机动车辆的性能及参数"),
		mk("内燃机的基本构造.md", "二（一）内燃机（汽油机，柴油机）的构造、工作原理、性能指标"),
		mk("柴油叉车发动机保养.md", "二（一）内燃机（汽油机，柴油机）的构造、工作原理、性能指标"),
		mk("未知章节.md", "三（九）其他章节"),
	}
	plans := buildCoursePlans(articles)
	if len(plans) != 2 {
		t.Fatalf("应生成 2 门课程，got %d", len(plans))
	}
	if len(plans[0].Chapters) != 4 {
		t.Fatalf("课程 A 应含 3 个大纲章节 + 1 个未归组兜底章，got %d", len(plans[0].Chapters))
	}
	if got := plans[0].Chapters[0].Title; got != "场车工作原理、结构特点与发展趋势" {
		t.Errorf("章节标题不符: %s", got)
	}
	if len(plans[1].Chapters) < 2 {
		t.Fatalf("课程 B 应按关键词分出多章，got %d", len(plans[1].Chapters))
	}
	if plans[1].Chapters[0].Title != "内燃机·使用保养与注意事项" {
		t.Errorf("内燃机首章应按桶定义顺序，got %s", plans[1].Chapters[0].Title)
	}
}

func TestBetterExplanation(t *testing.T) {
	template := "【答案：正确】本题考察叉车安全操作与维修技术相关知识，请结合教材相应章节理解掌握。（题库判断题第1题）"
	if betterExplanation("", template) {
		t.Error("空解析不应替换任何解析")
	}
	if !betterExplanation("侧翻常见原因：高速转弯、超载偏载。", template) {
		t.Error("实质解析应替换模板话术")
	}
	if betterExplanation(template, "实质解析") {
		t.Error("模板话术不应替换实质解析")
	}
}
