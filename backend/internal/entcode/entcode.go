// Package entcode 目录实体唯一编码的叶子包（ADR-0070 第三种破法）。
//
// 为什么单独成包而不是放进 internal/core：培训域与帮助中心域都要在创建时生成编码，
// 而 internal/core 的包内测试 import 了 internal/questionbank（它 import internal/training）
// ⇒ training → core 这条边会在 test 构建里成环。本包只依赖标准库，两域都能安全引用。
package entcode

import (
	"crypto/rand"
	"encoding/hex"
)

// Generate 生成唯一编码：<prefix>_<16 位 hex>。
//
// 管理端不再让管理员手填编码（培训目录六实体与帮助中心分类），创建时由后端生成。
// 字符集 [a-z0-9_] 同时满足 faq 分类标识的 ^[a-z0-9_]{2,64}$ 与各表 code 列的长度上限。
func Generate(prefix string) (string, error) {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return prefix + "_" + hex.EncodeToString(b), nil
}
