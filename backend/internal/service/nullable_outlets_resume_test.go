// 简历 / 联系方式域的 nullable 行为例（批①-B 第①段）。
//
// 为什么单独成文件：判据 4 的证据跟着出口走（分域一张表 + init 并进汇总，
// 机制见 nullable_declaration_test.go 的 nullableOutletTables）。
//
// 本段的八格都落在同一个事实上：`service.JSONArray.MarshalJSON` 在 `j == nil` 时字面发出
// `null`，而**非** nil 的 JSONArray 只是把列里那串字节原样透传——列里存着 4 字节的 JSON 字面量
// `null` 时，透传出来的就是 `null`。批①-A 把这条记在 nonnil_outlets_people_test.go 的文件头，
// 本文件把它跑成出口。
//
// 三格脱敏卡与四格明文卡的分界不在「是不是 JSONArray」，而在投影有没有重建（读
// resume_projection.go 的那段注释是改判前置条件）：
//   - JobCardDTO 四格 = toJobCardDTO 的原样透传 ⇒ 本文件四键。
//   - RecruitResumeCard.resume_certifications 走 maskCertificationsJSON（解码后 make(0,n) 重编）
//     ⇒ 任何列内容下都不是 null，已在 nonnil_outlets_people_test.go 举证，不在本文件。
//   - RecruitResumeCard.expected_regions / .resume_experiences 只有一道 `len(x)==0 → "[]"` 守卫
//     ⇒ 4 字节脏值从守卫缝里漏过去 ⇒ 本文件两键。
//   - ContactPlainDTO 两格：该类型在生产里**只是 swagger 形状声明**（GET /recruit/resumes/{id}/contact
//     的字节由 api/contact.go 那段 gin.H 直出，取的就是 toJobCardDTO 的同一份值），
//     所以出口按 handler 那一行的字段清单复现包装，值仍来自真实读库（同一口径的先例见
//     nonnil_outlets_people_test.go 末尾那三格「handler 一行包出来」的信封）。
//
// **这条证据的条件性要说明白**（不说明就是拿测试给自己壮胆）：job_cards 四列是
// `JSONB NOT NULL DEFAULT '[]'`（migrations/000008），NOT NULL 挡得住 SQL NULL、挡不住
// `'null'::jsonb`；而 JobCardService.applyInput 自该文件第一笔提交（598add7c）起就把客户端
// 发的 `null` 归一成 `[]`。⇒ 脏值不是当前任何写路径产生的，而是运维/导入/历史行能留下的形状。
// 本文件因此证的是「列里存 JSON null 时这一格确实发出 null」，即那句 `nullable` 不是虚标；
// 它**不**证「今天生产库里就有这样的行」。改成 `nonnil` 需要的是相反的证明（任何列内容都到不了
// null），而那由投影给出——这四格恰恰没有。
package service

import (
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"

	"forklift-training/internal/testutil"
)

var nullableOutletsResume = map[string]func(t *testing.T) any{
	// ===== 明文卡（toJobCardDTO 原样透传的四格）=====
	"service.JobCardDTO.expected_regions":      outletJobCardJSONNullColumn,
	"service.JobCardDTO.photos":                outletJobCardJSONNullColumn,
	"service.JobCardDTO.resume_certifications": outletJobCardJSONNullColumn,
	"service.JobCardDTO.resume_experiences":    outletJobCardJSONNullColumn,

	// ===== 脱敏卡（len==0 守卫漏掉 4 字节脏值的那两格）=====
	"service.RecruitResumeCard.expected_regions":   outletRecruitCardJSONNullColumn,
	"service.RecruitResumeCard.resume_experiences": outletRecruitCardJSONNullColumn,

	// ===== 明文联系方式面（形状声明，值同上）=====
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
	owner := seedForumUser(t, db, "脏列卡主")
	seedBlankJobCard(t, db, owner.ID, visibility)
	res := db.Exec("UPDATE job_cards SET expected_regions = 'null', photos = 'null', "+
		"resume_experiences = 'null', resume_certifications = 'null' WHERE user_id = ?", owner.ID)
	if res.Error != nil {
		t.Fatalf("写脏列失败: %v", res.Error)
	}
	if res.RowsAffected != 1 {
		t.Fatalf("脏列没写进去（影响 %d 行）—— 这一档列内容量不到，证据不成立", res.RowsAffected)
	}
	// 回读一次确认列里真的是 4 字节 `null`：不是 `[]`、也不是 SQL NULL（那会被 desensitize 的守卫接住）。
	var card model.JobCard
	if err := db.First(&card, "user_id = ?", owner.ID).Error; err != nil {
		t.Fatalf("回读简历卡失败: %v", err)
	}
	if string(card.Photos) != "null" {
		t.Fatalf("photos 列 = %q, 期望 4 字节 JSON 字面量 null", string(card.Photos))
	}
	return owner.ID
}

// outletJobCardJSONNullColumn 明文卡出口：GET /job-card 那一格的实际读路径（JobCardService.Get）。
func outletJobCardJSONNullColumn(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	ownerID := seedJobCardWithJSONNullColumns(t, db, "open")
	dto, err := NewJobCardService(db, nil, zap.NewNop()).Get(ownerID)
	if err != nil {
		t.Fatalf("取明文简历卡失败: %v", err)
	}
	return dto
}

// outletRecruitCardJSONNullColumn 脱敏卡出口：企业浏览读到的那张 L2 卡（同一份脏列）。
func outletRecruitCardJSONNullColumn(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	ownerID := seedJobCardWithJSONNullColumns(t, db, "open")
	card, err := NewRecruitService(db, zap.NewNop()).Get(ownerID)
	if err != nil {
		t.Fatalf("取脱敏简历卡失败: %v", err)
	}
	return card
}

// outletContactPlainJSONNullColumn 明文联系方式面：service.GetContact 的取值就是
// First(card) + toJobCardDTO（与明文卡同一份投影），handler 再按六个键直出。
// 这里复现的是那六个键的形状声明，Photos / ResumeCertifications 两格取真实读库结果。
func outletContactPlainJSONNullColumn(t *testing.T) any {
	t.Helper()
	db := testutil.NewMemoryDB(t)
	ownerID := seedJobCardWithJSONNullColumns(t, db, "open")
	dto, err := NewJobCardService(db, nil, zap.NewNop()).Get(ownerID)
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
