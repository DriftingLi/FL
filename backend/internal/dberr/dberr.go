// Package dberr 数据库错误的共享谓词叶子包。
//
// 为什么不留在 internal/service：唯一约束冲突的判定散落在打卡 / 积分 / 投稿 / 论坛等幂等写入点，
// 而域包一旦 import internal/service，只要 service 里还有一处引用该域，就成 import cycle
// （见 docs/agents/domain-package-migration.md §10）。谓词无状态、只吃 error，天然是叶子。
package dberr

import "strings"

// IsDuplicateError 判断数据库错误是否为唯一约束冲突——幂等写入点共享谓词，
// 收敛原多处字符串匹配复制；小写归一后兼容 PG（duplicate key / uq_ 约束名）
// 与 SQLite（UNIQUE constraint failed）双方言。
func IsDuplicateError(err error) bool {
	if err == nil {
		return false
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "duplicate") ||
		strings.Contains(msg, "unique") ||
		strings.Contains(msg, "uq_") ||
		strings.Contains(msg, "pk_")
}
