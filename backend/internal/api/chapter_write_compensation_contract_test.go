package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/security"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

type chapterWriteRecordingStorage struct {
	storage.Storage
	saved     []string
	deleted   []string
	deleteErr error
}

func (s *chapterWriteRecordingStorage) Save(ctx context.Context, key string, content []byte, contentType string) (string, error) {
	url, err := s.Storage.Save(ctx, key, content, contentType)
	if err == nil {
		s.saved = append(s.saved, url)
	}
	return url, err
}

func (s *chapterWriteRecordingStorage) Delete(ctx context.Context, url string) error {
	s.deleted = append(s.deleted, url)
	if s.deleteErr != nil {
		return s.deleteErr
	}
	return s.Storage.Delete(ctx, url)
}

func postTutorChapterFile(t *testing.T, router http.Handler, token string, chapterID int) *httptest.ResponseRecorder {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile("file", "课件.pdf")
	if err != nil {
		t.Fatalf("构造附件上传失败: %v", err)
	}
	if _, err := part.Write([]byte("pdf")); err != nil {
		t.Fatalf("写附件内容失败: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("结束 multipart 失败: %v", err)
	}
	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/tutor/chapter/%d/upload", chapterID), &body)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	return rec
}

func TestChapterWriteCompensationFailureIsExplicit(t *testing.T) {
	testutil.SetTestGinMode()
	db := testutil.NewMemoryDB(t)
	cfg := &config.Config{JWTSecretKey: "chapter-cleanup-failure-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	store := &chapterWriteRecordingStorage{
		Storage:   storage.NewLocalStorage(t.TempDir()),
		deleteErr: errors.New("injected storage cleanup failure"),
	}
	logCore, logs := observer.New(zap.InfoLevel)
	deps := NewDeps(cfg, db, store, zap.New(logCore), nil)
	deps.Session = security.SessionFromConfigWithBlacklist(cfg, testutil.NewValueBlacklist())
	router := NewRouter(deps)
	tutor := testutil.SeedTutor(t, db, "chapter_cleanup_failure_tutor", "x")
	token := slice6Token(t, cfg, tutor.TutorID, tutor.Username, "tutor")
	courseID, chapterID := seedChapterWriteCourse(t, db)
	rec := doTutorJSON(t, router, token, http.MethodPost, fmt.Sprintf("/api/tutor/course/%d/claim", courseID), "")
	if rec.Code != http.StatusOK {
		t.Fatalf("认领课程应 200，实际 %d: %s", rec.Code, rec.Body.String())
	}
	if err := db.Callback().Update().Before("gorm:update").Register("chapter_write:fail_cleanup_ownership", func(tx *gorm.DB) {
		if tx.Statement.Table == "chapter" {
			injected := errors.New("injected database failure")
			if got := tx.AddError(injected); !errors.Is(got, injected) {
				t.Errorf("AddError 未保留注入错误: got=%v want=%v", got, injected)
			}
		}
	}); err != nil {
		t.Fatalf("注册章节写入失败回调失败: %v", err)
	}

	rec = postTutorImage(t, router, token, fmt.Sprint(chapterID), "")
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("DB 与补偿均失败应返回 500，实际 %d: %s", rec.Code, rec.Body.String())
	}
	if len(store.saved) != 1 || len(store.deleted) != 1 || store.deleted[0] != store.saved[0] {
		t.Fatalf("补偿失败也必须显式尝试删除已落盘对象，saved=%v deleted=%v", store.saved, store.deleted)
	}
	exists, err := store.Exists(context.Background(), store.saved[0])
	if err != nil || !exists {
		t.Fatalf("注入删除失败后对象应仍在，exists=%v err=%v", exists, err)
	}
	found := false
	for _, entry := range logs.All() {
		if entry.Message != "[course] 章节写入落盘补偿失败" {
			continue
		}
		found = true
		detail := fmt.Sprint(entry.ContextMap()["error"])
		if !strings.Contains(detail, "数据库写入失败") || !strings.Contains(detail, "补偿失败") {
			t.Fatalf("补偿失败日志应同时暴露 DB 与清理错误，实际 %q", detail)
		}
	}
	if !found {
		t.Fatalf("补偿失败必须留下显式运行日志，实际日志=%v", logs.All())
	}
}

func TestChapterWriteCompensatesStoredObjectAfterDatabaseFailure(t *testing.T) {
	for _, tc := range []struct {
		name  string
		image bool
	}{
		{name: "图片归属写入失败", image: true},
		{name: "附件行创建失败", image: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			testutil.SetTestGinMode()
			db := testutil.NewMemoryDB(t)
			cfg := &config.Config{JWTSecretKey: "chapter-compensation-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
			store := &chapterWriteRecordingStorage{Storage: storage.NewLocalStorage(t.TempDir())}
			router := NewRouter(newContractDepsWithStorage(t, db, cfg, store))
			tutor := testutil.SeedTutor(t, db, "chapter_compensation_tutor", "x")
			token := slice6Token(t, cfg, tutor.TutorID, tutor.Username, "tutor")
			courseID, chapterID := seedChapterWriteCourse(t, db)

			claim := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/tutor/course/%d/claim", courseID), nil)
			claim.Header.Set("Authorization", "Bearer "+token)
			claimRec := httptest.NewRecorder()
			router.ServeHTTP(claimRec, claim)
			if claimRec.Code != http.StatusOK {
				t.Fatalf("认领课程应 200，实际 %d: %s", claimRec.Code, claimRec.Body.String())
			}

			injected := errors.New("injected chapter write database failure")
			if tc.image {
				if err := db.Callback().Update().Before("gorm:update").Register("chapter_write:fail_image_ownership", func(tx *gorm.DB) {
					if tx.Statement.Table == "chapter" {
						if got := tx.AddError(injected); !errors.Is(got, injected) {
							t.Errorf("AddError 未保留注入错误: got=%v want=%v", got, injected)
						}
					}
				}); err != nil {
					t.Fatalf("注册图片归属失败回调失败: %v", err)
				}
			} else {
				if err := db.Callback().Create().Before("gorm:create").Register("chapter_write:fail_attachment_create", func(tx *gorm.DB) {
					if tx.Statement.Table == "chapter_file" {
						if got := tx.AddError(injected); !errors.Is(got, injected) {
							t.Errorf("AddError 未保留注入错误: got=%v want=%v", got, injected)
						}
					}
				}); err != nil {
					t.Fatalf("注册附件创建失败回调失败: %v", err)
				}
			}

			var rec *httptest.ResponseRecorder
			if tc.image {
				rec = postTutorImage(t, router, token, fmt.Sprint(chapterID), "")
			} else {
				rec = postTutorChapterFile(t, router, token, chapterID)
			}
			if rec.Code != http.StatusInternalServerError {
				t.Fatalf("落盘后 DB 失败应返回 500，实际 %d: %s", rec.Code, rec.Body.String())
			}
			if len(store.saved) != 1 {
				t.Fatalf("测试应先真实落盘一个对象，实际 saved=%v", store.saved)
			}
			if len(store.deleted) != 1 || store.deleted[0] != store.saved[0] {
				t.Fatalf("module 应删除已落盘对象，saved=%v deleted=%v", store.saved, store.deleted)
			}
			exists, err := store.Exists(context.Background(), store.saved[0])
			if err != nil {
				t.Fatalf("检查补偿后的存储对象失败: %v", err)
			}
			if exists {
				t.Fatalf("补偿后存储对象仍存在: %s", store.saved[0])
			}
		})
	}
}
