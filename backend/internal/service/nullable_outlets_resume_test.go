// 留驻侧的 nullable 行为例：明文联系方式面两格（批①-B 第①段，P2 波 4e 收窄到只剩这一面）。
//
// 为什么单独成文件：判据 4 的证据跟着出口走（分域一张表 + init 并进汇总，
// 机制见 nullable_declaration_test.go 的 nullableOutletTables）。
//
// 波 4e 的收窄：本文件原有八格，其中 JobCardDTO 四格随简历域搬去 internal/resume/nullable_outlets_test.go、
// RecruitResumeCard 两格随招聘域搬去 internal/recruit/nullable_outlets_test.go；
// 留下的两格属 `contact`——它**不是** 30 个声明域里的一个（domains.go 无 contact 域），
// 联系方式交换闭环留驻 internal/service，故证据也留在这里。
//
// 这两格为什么是 nullable：`ContactPlainDTO` 在生产里**只是 swagger 形状声明**
// （GET /recruit/resumes/{id}/contact 的字节由 api/contact.go 那段 gin.H 直出，取的就是
// resume.ToJobCardDTO 的同一份值），所以出口按 handler 那一行的字段清单复现包装，
// 值仍来自真实读库。底层四列是 `JSONB NOT NULL DEFAULT '[]'`，NOT NULL 挡得住 SQL NULL、
// 挡不住 `'null'::jsonb` —— 本文件证的是「列里存 JSON null 时这一格确实发出 null」。
package service

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/resume"

	"forklift-training/internal/testutil"
)

var nullableOutletsResume = map[string]func(t *testing.T) any{
	// ===== 明文联系方式面（形状声明，值同 resume.ToJobCardDTO）=====
	"service.ContactPlainDTO.photos":                outletContactPlainJSONNullColumn,
	"service.ContactPlainDTO.resume_certifications": outletContactPlainJSONNullColumn,
}

func init() {
	nullableOutletTables = append(nullableOutletTables, nullableOutletsResume)
}

// seedJobCardWithJSONNullColumns 播一张简历卡，再把四个 JSONB 列写成 4 字节的 JSON 字面量
// `null`（`'null'::jsonb` 在 Postgres 与 sqlite 上都是合法值，NOT NULL 不拦它）。
// 刻意走**裸 SQL**：应用内任何写路径都会被 applyInput 归一成 `[]`，用 DTO 入口种脏列
// 就量不到列的真实形状了。
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

// seedBlankJobCard 播一张**四个 JSONB 列都没写过**的简历卡（visibility 由调用方给）。
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

// outletContactPlainJSONNullColumn 明文联系方式面：service.GetContact 的取值就是
// First(card) + resume.ToJobCardDTO（与明文卡同一份投影），handler 再按六个键直出。
// 这里复现的是那六个键的形状声明，Photos / ResumeCertifications 两格取真实读库结果。
func outletContactPlainJSONNullColumn(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	ownerID := seedJobCardWithJSONNullColumns(t, db, "open")
	dto, err := resume.NewService(db, nil, zap.NewNop()).Get(ownerID)
	if err != nil {
		t.Fatalf("取明文联系方式的底层简历卡失败: %v", err)
	}
	return &ContactPlainDTO{
		RealName:             dto.RealName,
		ContactPhone:         dto.ContactPhone,
		Wechat:               dto.Wechat,
		ResumeFileURL:        dto.ResumeFileURL,
		Photos:               dto.Photos,
		ResumeCertifications: dto.ResumeCertifications,
	}
}
