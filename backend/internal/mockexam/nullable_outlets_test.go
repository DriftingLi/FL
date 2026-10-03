// 模拟考域的 nullable 行为例（域包版本；由 internal/service/nullable_outlets_exam_test.go 随域搬来，波 4a）。
//
// 机制见本文件下半段的 nullableOutletTables（分域表 + init 并进汇总，runner 逐条执行，
// 不是给 AST 查名字的花名册）。**域包不得 import internal/core 的测试文件**（ADR-0070 决策 9：
// 证据表可住域包），所以这里自带一份形状相同的 runner，照抄
// internal/core/nullable_declaration_test.go。键格式 = <包名>.<类型>.<json键>，
// 登记处是 internal/apitypes/nullability_lock_test.go 的 nullableEvidenceSources（它按**目录**收表）。
//
// 这一格为什么 nullable（原判词，随键一起搬来）：
//   - MockExamSubmitDTO.details ⇒ **同一条声明经两个出口**：Submit 那侧是 `make(0,n)` 恒非 null，
//     GetResult 那侧在未交卷（result 列为空）时留零值 DTO ⇒ details 是 nil。
//     而 MockExamResultDTO 内嵌本 DTO、共用这一条声明，所以「恒非 null」在那条 GET 上是谎话。
package mockexam

import (
	"strings"
	"testing"

	"go.uber.org/zap"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// nullableOutlets 键 = 包名.类型名.json键（与判据 4 的账同一格式），值 = 走真实出口取到的结果。
var nullableOutlets = map[string]func(t *testing.T) any{
	"mockexam.MockExamSubmitDTO.details": outletMockExamResultUnsent,
}

// nullableOutletTables 是域包这张 nullable 表的汇总点；runner 逐条执行它（理由同留驻侧
// internal/core/nullable_declaration_test.go：一张只有 AST 在看的表就是花名册）。
var nullableOutletTables []map[string]func(t *testing.T) any

func init() {
	nullableOutletTables = append(nullableOutletTables, nullableOutlets)
}

// allNullableOutlets 展开所有分表。同名键出现在两张表里即判红：一条事实两处举证，改一处另一处还在过。
func allNullableOutlets(t *testing.T) map[string]func(t *testing.T) any {
	t.Helper()
	out := map[string]func(t *testing.T) any{}
	owner := map[string]int{}
	for i, tbl := range nullableOutletTables {
		for k, v := range tbl {
			if prev, dup := owner[k]; dup {
				t.Fatalf("%s 被第 %d 张与第 %d 张表各自举证——一条事实一个证据，留一处", k, prev, i)
			}
			owner[k] = i
			out[k] = v
		}
	}
	return out
}

// TestNullableDeclaredOutletsEmitNull 每条登记过的出口都必须真的发出 null。
//
// 键名反推 json 键：表键最后一段就是要看的键；对不上（字段被改名、被 omitempty 掉、
// 或整个类型不在射程里）一律判红，不留「找不到就当通过」的分支。
func TestNullableDeclaredOutletsEmitNull(t *testing.T) {
	outlets := allNullableOutlets(t)
	if len(outlets) == 0 {
		t.Fatal("证据表是空的：判据 4 会因此空转，这里必须同步红")
	}
	for key, build := range outlets {
		t.Run(key, func(t *testing.T) {
			jsonKey := key[strings.LastIndex(key, ".")+1:]
			got := testutil.MarshalKey(t, build(t), jsonKey)
			if got != "null" {
				t.Fatalf("声明 nullable 的 %s 实际发出 %s——这句表态没有出口证明，"+
					"要么找一条真发 null 的出口，要么按实测改判 nonnil", key, got)
			}
		})
	}
}

// outletMockExamResultUnsent 已开考未交卷的模考：result 列没写过 ⇒ GetResult 里那个
// `var result MockExamSubmitDTO` 保持零值，内嵌进响应后 details 发 null（HTTP 仍是 200）。
func outletMockExamResultUnsent(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	student := testutil.SeedStudent(t, db, "模考学员", "x")
	exam := model.MockExam{
		ID: 9301, StudentID: student.ID, Status: "in_progress",
		QuestionIDs: model.JSONB([]byte("[1,2]")),
		CreatedAt:   testutil.Now(),
	}
	if err := db.Create(&exam).Error; err != nil {
		t.Fatalf("播未交卷模考失败: %v", err)
	}
	res, err := NewService(db, nil, zap.NewNop()).GetResult(exam.ID, student.ID)
	if err != nil {
		t.Fatalf("取未交卷模考结果失败: %v", err)
	}
	return res
}
