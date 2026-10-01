// Package entitlement 权益（entitlement）读面单点（ADR-0062 决策 3，CONTEXT.md「权益」词条）。
//
// 「这一份是否被这个人兑换过」的唯一查询：积分域的已拥有判定与各域门禁都经此，不得在调用侧
// 再写一遍 user_entitlement 查询；sku 词汇表也在此单点 —— 兑换写入与门禁读取同取一处，
// 「写了没人读 / 读了写不中」就是 unlock_real_paper 那笔死端的成因（该死端现由
// internal/service/shop_sku_registry.go 的对账表封住）。
//
// 落点（#1445 P2 波 1b-0）：本包是 points 域搬包前提出来的共享叶子。points 域、course 域、
// real_exam 读面都要用同一份 sku 词汇与同一条判据，而 points 先搬 ⇒ 载体不能留在任一域包里
// （同 ADR-0070「跨域共享的词汇贴着实体 / 收进叶子包」）。本包只吃 *gorm.DB 与入参，
// 不持服务状态、不 import 任何域包。
package entitlement

import (
	"fmt"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// CourseSKU 课程权益 sku（course:<courseID>，ref_id=<courseID>），与 RealPaperSKU 同形。
func CourseSKU(courseID int) string { return fmt.Sprintf("course:%d", courseID) }

// RealPaperSKU 真题卷权益 sku（real_paper:<paperID>，ref_id=<paperID>，按套粒度）。
// 自 points_service.go 迁入：兑换写入（RedeemRealPaper）与门禁读取（真题读面）必须同取此处。
func RealPaperSKU(paperID int) string { return fmt.Sprintf("real_paper:%d", paperID) }

// Holds 权益判据：按 (主体, sku, ref_id) 查存在性。
// 返回 error：查不动不得被读成「没兑换过」（ADR-0062 票6 同判据）——旧写法咽掉错误，
// 一次 DB 抖动就把已付费的学员挡在自己买过的内容外面。
func Holds(db *gorm.DB, userID int, sku, refID string) (bool, error) {
	var cnt int64
	if err := db.Model(&model.UserEntitlement{}).
		Where("user_id = ? AND sku = ? AND ref_id = ?", userID, sku, refID).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}
