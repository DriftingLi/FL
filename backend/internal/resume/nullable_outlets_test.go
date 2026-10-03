// 简历域的 nullable 行为例（ADR-0065 决策 5 判据 4；P2 波 4e 随域包拆出）。
//
// 四格 = `JobCardDTO` 的四个 JSONB 字段，都是 `resume.ToJobCardDTO` 的**原样透传**：
// `JSONArray.MarshalJSON` 在 `j == nil` 时字面发出 `null`，非 nil 时把列里那串字节原样透传
// ⇒ 列里存 4 字节 `null` 时透传出来就是 `null`。
//
// **这条证据的条件性要说明白**（不说明就是拿测试给自己壮胆）：job_cards 四列是
// `JSONB NOT NULL DEFAULT '[]'`（migrations/000008），NOT NULL 挡得住 SQL NULL、挡不住
// `'null'::jsonb`；而 Service.applyInput 自该文件第一笔提交（598add7c）起就把客户端发的 `null`
// 归一成 `[]`。⇒ 脏值不是当前任何写路径产生的，而是运维 / 导入 / 历史行能留下的形状。
// 本文件因此证的是「列里存 JSON null 时这一格确实发出 null」，即那句 `nullable` 不是虚标。
package resume

import (
	"strings"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

// nullableOutletsResume 键 = 包名.类型名.json键，值 = 走真实出口取到的结果。
var nullableOutletsResume = map[string]func(t *testing.T) any{
	"resume.JobCardDTO.expected_regions":      outletJobCardJSONNullColumn,
	"resume.JobCardDTO.photos":                outletJobCardJSONNullColumn,
	"resume.JobCardDTO.resume_certifications": outletJobCardJSONNullColumn,
	"resume.JobCardDTO.resume_experiences":    outletJobCardJSONNullColumn,
}

// nullableOutletTables 是域包这张 nullable 表的汇总点；runner 逐条执行它（不是给 AST 查名字的花名册）。
var nullableOutletTables []map[string]func(t *testing.T) any

func init() {
	nullableOutletTables = append(nullableOutletTables, nullableOutletsResume)
}

// allNullableOutlets 展开所有分表。同名键出现在两张表里即判红：一条事实两处举证。
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
// 键名反推 json 键：表键最后一段就是要看的键；对不上（字段改名 / 被 omitempty / 类型不在射程）一律判红。
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

// seedJobCardWithJSONNullColumns 播一张简历卡，再把四个 JSONB 列写成 4 字节的 JSON 字面量
// `null`（`'null'::jsonb` 在 Postgres 与 sqlite 上都是合法值，NOT NULL 不拦它）。
// 刻意走**裸 SQL**：应用内任何写路径都会被 applyInput 归一成 `[]`，用 DTO 入口种脏列
// 就量不到列的真实形状了。逐字副本（registry 与 internal/core 各一份，域包测试不互相 import）。
func seedJobCardWithJSONNullColumns(t *testing.T, db *gorm.DB, visibility string) int {
	t.Helper()
	owner := testutil.SeedStudent(t, db, "脏列卡主", "hash")
	seedBlankJobCard(t, db, owner.ID, visibility)
	res := db.Exec("UPDATE job_cards SET expected_regions = 'null', photos = 'null', "+
		"resume_experiences = 'null', resume_certifications = 'null' WHERE user_id = ?", owner.ID)
	if res.Error != nil {
		t.Fatalf("写脏列失败: %v", res.Error)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("脏列没写进去（影响 %d 行）—— 这一档列内容量不到，证据不成立", res.RowsAffected)
	}
	var card model.JobCard
	if err := db.First(&card, "user_id = ?", owner.ID).Error; err != nil {
		t.Fatalf("回读简历卡失败: %v", err)
	}
	if string(card.Photos) != "null" {
		t.Fatalf("photos 列 = %q, 期望 4 字节 JSON 字面量 null", string(card.Photos))
	}
	return owner.ID
}

// seedBlankJobCard 播一张四个 JSONB 列都没写过的简历卡（逐字副本，见 nonnil 表的同款说明）。
func seedBlankJobCard(t *testing.T, db *gorm.DB, userID int, visibility string) {
	t.Helper()
	card := model.JobCard{
		UserID: userID, RealName: "张三丰", Visibility: visibility,
		CreatedAt: testutil.Now(), UpdatedAt: testutil.Now(),
	}
	if err := db.Create(&card).Error; err != nil {
		t.Fatalf("播简历卡失败: %v", err)
	}
}

// outletJobCardJSONNullColumn 明文卡出口：GET /api/resume 那一格的实际读路径（Service.Get）。
func outletJobCardJSONNullColumn(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	ownerID := seedJobCardWithJSONNullColumns(t, db, "open")
	dto, err := NewService(db, nil, zap.NewNop()).Get(ownerID)
	if err != nil {
		t.Fatalf("取明文简历卡失败: %v", err)
	}
	return dto
}
