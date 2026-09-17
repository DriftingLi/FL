package service

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"testing"
)

// 本文件 = 「service 包不得再出现裸 credential_id 谓词」的静态扫描锁（ADR-0056 §2 锁之一）。
// 用 Go 测试实现（读包内源码），不新增 CI 步骤、不动 scripts/ 与 .github/workflows。

var (
	// 裸谓词：字符串里直接出现「证件列 + 比较符」（等值 / 不等 / NULL 判定）。
	bareCredentialPredicateRE = regexp.MustCompile(`(?i)credential_id\s*(?:=|!=|<>|IS\s+NOT\s+NULL|IS\s+NULL)`)
	// 列常量拼接：把证件列表达式与运算符字面量拼起来（内联谓词的第二形态，
	// 如 QuestionPoolCredentialColumn + " = ?"）。只允许出现在谓词实现处。
	credentialColumnConcatRE = regexp.MustCompile(`(?i)credentialcolumn\s*\+`)
)

// scanBareCredentialPredicates 扫描一段源码里的内联证件分区谓词，返回「行号: 原文」列表。
// 整行注释跳过（文档里引用谓词形态不算违规）；具名谓词调用（列名作为实参传入）与
// 列名常量定义都不算违规。
func scanBareCredentialPredicates(src string) []string {
	var hits []string
	for i, line := range strings.Split(src, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "//") {
			continue
		}
		if bareCredentialPredicateRE.MatchString(line) || credentialColumnConcatRE.MatchString(line) {
			hits = append(hits, fmt.Sprintf("%d: %s", i+1, strings.TrimSpace(line)))
		}
	}
	return hits
}

// TestServicePackageHasNoBareCredentialPredicate 真源扫描：包内**全部**源文件（含测试）里，
// 证件分区谓词只允许出现在白名单——
//   - credential_scope.go：三族谓词的实现处；
//   - credential_scope_guard_test.go：本扫描器自身的探针样本（回归形态的字面量本身就是样本）。
func TestServicePackageHasNoBareCredentialPredicate(t *testing.T) {
	const implementation = "credential_scope.go"
	allowlist := map[string]bool{implementation: true, "credential_scope_guard_test.go": true}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("读取包目录失败: %v", err)
	}
	var scanned, implementationHits int
	var violations []string
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatalf("读取 %s 失败: %v", name, err)
		}
		hits := scanBareCredentialPredicates(string(src))
		if allowlist[name] {
			if name == implementation {
				implementationHits = len(hits)
			}
			continue
		}
		scanned++
		for _, h := range hits {
			violations = append(violations, name+":"+h)
		}
	}
	// 扫描面非空：包内源文件确实被读到（防「目录读空 → 恒绿」）。
	if scanned < 150 {
		t.Fatalf("扫描面异常：只扫到 %d 个包内源文件（排除白名单）", scanned)
	}
	// 规则活性：白名单文件里必须命中谓词实现，否则规则与实现脱钩、扫描恒绿。
	if implementationHits < 3 {
		t.Fatalf("扫描规则失灵：白名单 %s 只命中 %d 条谓词实现", implementation, implementationHits)
	}
	if len(violations) > 0 {
		t.Fatalf("service 包出现裸证件分区谓词（应改调 credential_scope.go 的具名谓词）：\n%s", strings.Join(violations, "\n"))
	}
}

// TestCredentialPredicateScanProbes 负向探针：把「回归形态」合成进样本，断言规则一定报违规；
// 同时用负样本证明规则不误伤具名调用、列名常量与整行注释。没有这组探针，上面的扫描可能是空转。
func TestCredentialPredicateScanProbes(t *testing.T) {
	cases := []struct {
		name string
		src  string
		want int
	}{
		{"正向探针：内联等值谓词", "q = q.Where(\"credential_id = ?\", *credentialID)", 1},
		{"正向探针：带表名前缀", "q = q.Where(\"question.credential_id = ?\", *credentialID)", 1},
		{"正向探针：NULL 桶分支", "return \"credential_id IS NULL\", nil", 1},
		{"正向探针：非空 NULL 分支", "return \"credential_id IS NOT NULL\", nil", 1},
		{"正向探针：列常量拼接", "query += \" AND \" + QuestionPoolCredentialColumn + \" = ?\"", 1},
		{"负向样本：具名谓词调用", "q = EntityOwnedBy(q, \"question.credential_id\", credentialID)", 0},
		{"负向样本：列名常量定义", "const QuestionPoolCredentialColumn = \"question.credential_id\"", 0},
		{"负向样本：无关谓词", "q = q.Where(\"status = ?\", 1)", 0},
		{"负向样本：整行注释", "// 旧实现：q.Where(\"credential_id = ?\", cred)", 0},
	}
	for _, tc := range cases {
		if got := len(scanBareCredentialPredicates(tc.src)); got != tc.want {
			t.Errorf("%s: 命中 %d 条, want %d (%v)", tc.name, got, tc.want, scanBareCredentialPredicates(tc.src))
		}
	}
}
