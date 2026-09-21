// Package service 实现业务服务层。
// 本文件：权益（entitlement）读面单点（ADR-0062 决策 3，CONTEXT.md「权益」词条）。
// 「这一份是否被这个人兑换过」的唯一查询：积分域的已拥有判定与各域门禁都经此，
// 不得在调用侧再写一遍 user_entitlement 查询。
package service

import (
	"fmt"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// CourseSKU 课程权益 sku（course:<courseID>，ref_id=<courseID>），与 RealPaperSKU 同形。
// 兑换写入与门禁读取同取此处——sku 词汇表出现第二个源时，「写了没人读 / 读了写不中」
// 就是本案 unlock_real_paper 那笔死端的成因。
func CourseSKU(courseID int) string { return fmt.Sprintf("course:%d", courseID) }

// holdsEntitlement 权益判据：按 (主体, sku, ref_id) 查存在性。
// 返回 error：查不动不得被读成「没兑换过」（ADR-0062 票6 同判据）——旧写法咽掉错误，
// 一次 DB 抖动就把已付费的学员挡在自己买过的内容外面。
func holdsEntitlement(db *gorm.DB, userID int, sku, refID string) (bool, error) {
	var cnt int64
	if err := db.Model(&model.UserEntitlement{}).
		Where("user_id = ? AND sku = ? AND ref_id = ?", userID, sku, refID).Count(&cnt).Error; err != nil {
		return false, err
	}
	return cnt > 0, nil
}
