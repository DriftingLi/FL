// Package dberr 谓词单测：PG / SQLite 双方言唯一冲突文案（原 internal/service/forum_counter_test.go，
// P2 波 0a 随 IsDuplicateError 一起归位本包）。
package dberr

import (
	"errors"
	"testing"
)

// --- IsDuplicateError：PG / SQLite 双言文案 ---

func TestIsDuplicateError_PostgresDialect(t *testing.T) {
	err := errors.New(`ERROR: duplicate key value violates unique constraint "uq_forum_topic_like" (SQLSTATE 23505)`)
	if !IsDuplicateError(err) {
		t.Fatal("PG duplicate key 文案应判为唯一冲突")
	}
}

func TestIsDuplicateError_SQLiteDialect(t *testing.T) {
	err := errors.New("UNIQUE constraint failed: forum_checkin.user_id, forum_checkin.check_date")
	if !IsDuplicateError(err) {
		t.Fatal("SQLite UNIQUE constraint 文案应判为唯一冲突")
	}
}

func TestIsDuplicateError_ConstraintNamePrefixes(t *testing.T) {
	for _, msg := range []string{
		"constraint uq_forum_reply_like violated",
		"pk_forum_checkin 冲突",
	} {
		if !IsDuplicateError(errors.New(msg)) {
			t.Fatalf("约束名前缀文案应判为唯一冲突: %s", msg)
		}
	}
}

func TestIsDuplicateError_Negative(t *testing.T) {
	for _, err := range []error{
		nil,
		errors.New("record not found"),
		errors.New("connection refused"),
	} {
		if IsDuplicateError(err) {
			t.Fatalf("非唯一冲突错误不应命中: %v", err)
		}
	}
}
