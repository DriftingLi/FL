// Package scope 证件分区 module（ADR-0056 §2 / ADR-0047 §4 / ADR-0051）：
// 「当前证件」（hrwai_users.current_credential_id）在**读面**的三族具名谓词。
//
// 词表出处：CONTEXT.md「当前证件」。分区的三族语义不可混用——语义由**名字**承担，
// 不由同一个「可选证件」参数兼表两义。三族的 nil（未选证件）语义**相反**：
//
//   - RecordPartitionOf —— 记录冻结分区：行为事实（练习记录、模考记录）在产生那一刻
//     记下的证件（ADR-0051 写入时冻结），读面按记录上的分区列过滤。
//     nil = 不分区、**看全部**（未选证件的学员看到自己的全部历史）。
//   - PartitionBucket —— NULL 桶：按证件分桶存放的历史分区（practice_progress 顺序练习进度）。
//     nil = 只取 credential_id IS NULL 那一桶，**不是**看全部（未选证件的进度是独立一桶）。
//   - EntityOwnedBy —— 归属分区：读**被检索对象自身**的证件列（课程挂载、题库池、搜索分区、
//     收藏目标、真题卷）。nil = 不分区、**看全部**。
//
// 双形态同源：gorm 链式形态（主形态，三谓词）+ SQL 片段形态（raw SQL 装配点，如 catalog
// 标签计数的 LEFT JOIN + FILTER 计数）。片段形态由对应谓词委托调用，保证「raw 重写与谓词
// 脱钩」这一漂移窗口关闭（照 question_pool_scope.go 的双形态先例）；片段形态**导出**
// 是搬包的要求——raw 装配点在域包里（favorite_service.go、training_catalog_service.go），
// 私有片段它够不着。公开面因此是 3 谓词 + 3 片段，但拼接点仍只在本文件的 4 行。
//
// 落点（#1445 P2 波 0b）：本文件原为 internal/service/credential_scope.go。域包离开
// internal/core 后仍要用这三族谓词，留在 core 就是「域包 → core」的反向依赖
// （P2 判据：出边为零才搬得动），故整文件搬进叶子包 internal/scope。静态扫描锁
// （internal/core/credential_scope_guard_test.go）的白名单随它改指本文件——「不许在
// 调用点手写 credential_id 谓词」这条规矩的射程与实现处一起走，不因搬包而放宽。
//
// _Avoid_：
//   - 把三族合并成一个「可选证件」参数——两种相反的 nil 语义会同形不同义（ADR-0056 §2 的由来）；
//   - 把「未选证件」一律理解成「看全部」——NULL 桶不是；
//   - 在调用点就地手写 credential_id 谓词——静态扫描锁住（白名单 = 本文件的谓词实现处）。
//
// 边界（ADR-0056 §2）：**不合并** ADR-0050 §1 否掉的「学员可见性」那一族——题库池
// （question_pool_scope.go）与课程挂载（course_mount_scope.go）是另一种不变式，各自保留；
// 本 module 只承载它们内部那一格证件分区。
package scope

import "gorm.io/gorm"

// RecordPartitionOf 记录冻结分区谓词：按记录上的分区列过滤（ADR-0051 写入时冻结）。
// credentialColumn 传记录表的列名（带表名前缀，如 "question_practice_record.credential_id"）。
// credentialID 为 nil 时不过滤——未选证件读作「不分区、看全部」。
func RecordPartitionOf(q *gorm.DB, credentialColumn string, credentialID *int) *gorm.DB {
	clause, args := RecordPartitionClause(credentialColumn, credentialID)
	if clause == "" {
		return q
	}
	return q.Where(clause, args...)
}

// PartitionBucket 分桶谓词（NULL 桶）：按证件分桶存放的历史分区（顺序练习进度）。
// credentialID 为 nil 时只取「未选定证件」那一桶（credential_id IS NULL）——PG 不支持
// IS 参数占位符，故两个分支在片段形态内分派。
//
// 注意与 RecordPartitionOf / EntityOwnedBy 的 nil 语义**相反**：这里 nil 不是「看全部」。
func PartitionBucket(q *gorm.DB, credentialColumn string, credentialID *int) *gorm.DB {
	clause, args := PartitionBucketClause(credentialColumn, credentialID)
	return q.Where(clause, args...)
}

// EntityOwnedBy 归属分区谓词：读被检索对象自身的证件列（课程挂载、题库池、搜索分区、
// 收藏目标、真题卷）。credentialID 为 nil 时不过滤——未选证件读作「不分区、看全部」。
func EntityOwnedBy(q *gorm.DB, credentialColumn string, credentialID *int) *gorm.DB {
	clause, args := EntityOwnedByClause(credentialColumn, credentialID)
	if clause == "" {
		return q
	}
	return q.Where(clause, args...)
}

// RecordPartitionClause 记录冻结分区的 SQL 片段形态：nil → 空串（调用方据此跳过整个片段，
// 即「不分区、看全部」）。raw 装配点必须先判空串再追加，空片段不许进 SQL。
func RecordPartitionClause(credentialColumn string, credentialID *int) (string, []any) {
	if credentialID == nil {
		return "", nil
	}
	return credentialColumn + " = ?", []any{*credentialID}
}

// PartitionBucketClause NULL 桶的 SQL 片段形态：nil → credential_id IS NULL（恒非空片段）。
func PartitionBucketClause(credentialColumn string, credentialID *int) (string, []any) {
	if credentialID == nil {
		return credentialColumn + " IS NULL", nil
	}
	return credentialColumn + " = ?", []any{*credentialID}
}

// EntityOwnedByClause 归属分区的 SQL 片段形态：nil → 空串（调用方据此跳过整个片段，
// 即「不分区、看全部」）。raw SQL 装配点必须先判空串再追加。
func EntityOwnedByClause(credentialColumn string, credentialID *int) (string, []any) {
	if credentialID == nil {
		return "", nil
	}
	return credentialColumn + " = ?", []any{*credentialID}
}
