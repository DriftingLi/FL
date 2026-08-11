// Package service 实现业务服务层。
// 本文件：默认昵称生成（叉车人 + 随机编码，保证不重复）。
package service

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/model"
)

// generateDefaultNickname 生成默认昵称：叉车人 + 6 位随机数字，创建前查重，冲突重试。
func generateDefaultNickname(db *gorm.DB) string {
	for i := 0; i < 10; i++ {
		nickname := "叉车人" + randomDigits(6)
		var count int64
		if err := db.Model(&model.HrwaiUser{}).Where("username = ?", nickname).Count(&count).Error; err != nil {
			continue
		}
		if count == 0 {
			return nickname
		}
	}
	// 兜底：毫秒时间戳尾部 6 位，冲突概率极低
	return fmt.Sprintf("叉车人%06d", time.Now().UnixMilli()%1000000)
}

// IsValidAccount 校验登录账号格式（4-20 位字母/数字/下划线）。
func IsValidAccount(account string) bool {
	if len(account) < 4 || len(account) > 20 {
		return false
	}
	for _, r := range account {
		if !(r >= 'a' && r <= 'z') && !(r >= 'A' && r <= 'Z') && !(r >= '0' && r <= '9') && r != '_' {
			return false
		}
	}
	return true
}

// randomDigits 使用密码学安全随机数生成 n 位十进制数字串。
func randomDigits(n int) string {
	const digits = "0123456789"
	b := make([]byte, n)
	limit := big.NewInt(int64(len(digits)))
	for i := range b {
		v, err := rand.Int(rand.Reader, limit)
		if err != nil {
			b[i] = digits[i%len(digits)]
			continue
		}
		b[i] = digits[v.Int64()]
	}
	return string(b)
}
