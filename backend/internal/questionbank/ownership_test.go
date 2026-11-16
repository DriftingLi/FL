// 题库写隔离（#2598 / 规格块 C）的判据测试。
//
// seam：服务层公开方法（规格 Testing Decisions「阶段二（数据级谓词，seam＝service 层公开方法）」）。
// 这里锁四件事：
//   - 三条写路径对他人题目全拒、对本人放行（C1）；
//   - created_by 为 NULL 的存量题：讲师拒、管理员放行（C2）；
//   - 非本人写的错误是 ErrQuestionNotOwned（→ 403，不是 404 伪装，C3）；
//   - 「我创建的」筛选只收行集、不改写判定（C2 的另一半）。
package questionbank

import (
	"errors"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

const (
	ownerTutor   = 701 // 题目的作者
	otherTutor   = 702 // 另一位讲师
	adminActorID = 703 // 管理员账号 id（类型判别在前，id 取值不影响判定）
)

// seedOwnedQuestion 造一道带明确作者的题（testutil.SeedQuestion 不填 created_by，
// 那正好是 C2 的 NULL 形态，被 TestOwnership_NullCreator 单独覆盖）。
func seedOwnedQuestion(t *testing.T, db *gorm.DB, createdBy *int) *model.Question {
	t.Helper()
	q := &model.Question{
		Type:          "single_choice",
		Content:       "归属判据题",
		Answer:        "A",
		Status:        "published",
		CreatedBy:     createdBy,
		CreatedByType: "tutor",
		CreatedAt:     testutil.Now(),
		UpdatedAt:     testutil.Now(),
	}
	if err := db.Create(q).Error; err != nil {
		t.Fatalf("插入题目失败: %v", err)
	}
	return q
}

func ptrInt(v int) *int { return &v }

// TestOwnership_ThreeWritePaths 三条写路径逐条：本人放行、他人 403 哨兵（C1/C3）。
//
// 表驱动而不是三个独立用例：三条路径的判据是同一条谓词，分三段写会让「哪条漏挂了」
// 在新增第四条路径时看不出来——这里一张表就是那份清单。
func TestOwnership_ThreeWritePaths(t *testing.T) {
	cases := []struct {
		name string
		// call 用给定身份调一条写路径
		call func(svc *Service, id int, actorID int, actorType string) error
	}{
		{"更新题目", func(svc *Service, id, actorID int, actorType string) error {
			content := "改后的题干"
			_, err := svc.UpdateQuestion(id, QuestionUpdateInput{Content: &content}, actorID, actorType)
			return err
		}},
		{"提交审核", func(svc *Service, id, actorID int, actorType string) error {
			_, err := svc.SubmitQuestion(id, actorID, actorType)
			return err
		}},
		{"删除题目", func(svc *Service, id, actorID int, actorType string) error {
			return svc.DeleteQuestion(id, actorID, actorType)
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// 本人：放行
			svc, db := newQuestionBankSvc(t)
			q := seedOwnedQuestion(t, db, ptrInt(ownerTutor))
			if q.Status != "draft" {
				if err := db.Model(&model.Question{}).Where("id = ?", q.ID).Update("status", "draft").Error; err != nil {
					t.Fatalf("置 draft 失败: %v", err)
				}
			}
			if err := c.call(svc, q.ID, ownerTutor, "tutor"); err != nil {
				t.Fatalf("本人应放行, got %v", err)
			}

			// 他人：403 哨兵
			svc2, db2 := newQuestionBankSvc(t)
			q2 := seedOwnedQuestion(t, db2, ptrInt(ownerTutor))
			if err := db2.Model(&model.Question{}).Where("id = ?", q2.ID).Update("status", "draft").Error; err != nil {
				t.Fatalf("置 draft 失败: %v", err)
			}
			err := c.call(svc2, q2.ID, otherTutor, "tutor")
			if !errors.Is(err, ErrQuestionNotOwned) {
				t.Fatalf("他人应拒 ErrQuestionNotOwned, got %v", err)
			}

			// 管理员：直通（即便题目是别人的）
			svc3, db3 := newQuestionBankSvc(t)
			q3 := seedOwnedQuestion(t, db3, ptrInt(ownerTutor))
			if err := db3.Model(&model.Question{}).Where("id = ?", q3.ID).Update("status", "draft").Error; err != nil {
				t.Fatalf("置 draft 失败: %v", err)
			}
			if err := c.call(svc3, q3.ID, adminActorID, "admin"); err != nil {
				t.Fatalf("管理员应直通, got %v", err)
			}
		})
	}
}

// TestOwnership_NullCreator created_by 为 NULL 的存量题：讲师一律拒、管理员放行（C2）。
//
// 空值不是「人人有份」而是「作者未知」——判给任何一位讲师都等于凭空授权。
func TestOwnership_NullCreator(t *testing.T) {
	cases := []struct {
		name string
		call func(svc *Service, id int, actorID int, actorType string) error
	}{
		{"更新题目", func(svc *Service, id, actorID int, actorType string) error {
			content := "改后的题干"
			_, err := svc.UpdateQuestion(id, QuestionUpdateInput{Content: &content}, actorID, actorType)
			return err
		}},
		{"提交审核", func(svc *Service, id, actorID int, actorType string) error {
			_, err := svc.SubmitQuestion(id, actorID, actorType)
			return err
		}},
		{"删除题目", func(svc *Service, id, actorID int, actorType string) error {
			return svc.DeleteQuestion(id, actorID, actorType)
		}},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, db := newQuestionBankSvc(t)
			q := seedOwnedQuestion(t, db, nil)
			if err := db.Model(&model.Question{}).Where("id = ?", q.ID).Update("status", "draft").Error; err != nil {
				t.Fatalf("置 draft 失败: %v", err)
			}
			// 任何讲师 id 都不该拿到它，包括 0（旧实现里「created_by 未设」会退化成 0）
			for _, id := range []int{ownerTutor, otherTutor, 0} {
				if err := c.call(svc, q.ID, id, "tutor"); !errors.Is(err, ErrQuestionNotOwned) {
					t.Fatalf("讲师 %d 对空创建者题应拒, got %v", id, err)
				}
			}
			// 管理员放行
			if err := c.call(svc, q.ID, adminActorID, "admin"); err != nil {
				t.Fatalf("管理员应放行空创建者题, got %v", err)
			}
		})
	}
}

// TestOwnership_PublishedReviewInvariantNotRegressed 讲师改自己已发布题的内容仍回 pending（C2 后半）。
//
// 本票只加归属判定，不该动既有的审核不变式——它是「先审后发」的执行面，回归了没人会立刻发现
// （题仍是 published，内容却已换）。
func TestOwnership_PublishedReviewInvariantNotRegressed(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	q := seedOwnedQuestion(t, db, ptrInt(ownerTutor)) // published
	newContent := "改过的题干"
	result, err := svc.UpdateQuestion(q.ID, QuestionUpdateInput{Content: &newContent}, ownerTutor, "tutor")
	if err != nil {
		t.Fatalf("本人改自己的题应放行: %v", err)
	}
	if result.Status != "pending" {
		t.Fatalf("讲师改已发布题内容应回 pending, got %q", result.Status)
	}
}

// TestOwnership_FilterIsRowSetOnly 「我创建的」筛选只收行集，不改写判定（C2 的另一半 + C4）。
func TestOwnership_FilterIsRowSetOnly(t *testing.T) {
	svc, db := newQuestionBankSvc(t)
	mine := seedOwnedQuestion(t, db, ptrInt(ownerTutor))
	seedOwnedQuestion(t, db, ptrInt(otherTutor))
	orphan := seedOwnedQuestion(t, db, nil) // 空创建者：不属于任何人的「我创建的」

	// created_by = 本人：只出自己那道
	got, err := svc.ListQuestions(1, 20, "", "", "", nil, NewQuestionEditScope(nil), "", ptrInt(ownerTutor))
	if err != nil {
		t.Fatalf("带筛选的列表失败: %v", err)
	}
	ids := make([]int, 0, len(got.Questions))
	for _, q := range got.Questions {
		ids = append(ids, q.ID)
	}
	if len(ids) != 1 || ids[0] != mine.ID {
		t.Fatalf("created_by=%d 应只返回 %d, got %v（空创建者题不该被算成任何人的）", ownerTutor, mine.ID, ids)
	}

	// 不带筛选：全量（C4 读面不变，含空创建者题）
	all, err := svc.ListQuestions(1, 20, "", "", "", nil, NewQuestionEditScope(nil), "", nil)
	if err != nil {
		t.Fatalf("无筛选的列表失败: %v", err)
	}
	if all.Total != 3 {
		t.Fatalf("读面应看全量 3 道, got %d", all.Total)
	}
	_ = orphan
}

// TestOwnership_AnonymousActorFailsClosed 取不到身份时一律拒（fail closed）。
//
// `middleware.CurrentUserID` 在 ctx 里没有 uid 时返回 0，而 0 是一个**能写进库的合法值**
// （建题时若 uid 取失败就会传 &0）。若谓词不单独挡这一格，「一次鉴权故障」就会被读成
// 「匹配上了 created_by=0 的那道题」——把故障变成越权，是安全判据里最坏的失败方向。
func TestOwnership_AnonymousActorFailsClosed(t *testing.T) {
	zero := 0
	cases := []struct {
		name      string
		createdBy *int
	}{
		{"created_by=0（建题时 uid 取失败留下的行）", &zero},
		{"created_by=NULL", nil},
		{"created_by=他人", ptrInt(otherTutor)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			svc, db := newQuestionBankSvc(t)
			q := seedOwnedQuestion(t, db, c.createdBy)
			if err := db.Model(&model.Question{}).Where("id = ?", q.ID).Update("status", "draft").Error; err != nil {
				t.Fatalf("置 draft 失败: %v", err)
			}
			content := "匿名改"
			// actorID=0 且角色是 tutor：无论题目 created_by 是什么，都必须拒
			if _, err := svc.UpdateQuestion(q.ID, QuestionUpdateInput{Content: &content}, 0, "tutor"); !errors.Is(err, ErrQuestionNotOwned) {
				t.Fatalf("取不到身份时应拒, got %v", err)
			}
			if _, err := svc.SubmitQuestion(q.ID, 0, "tutor"); !errors.Is(err, ErrQuestionNotOwned) {
				t.Fatalf("取不到身份时提审应拒, got %v", err)
			}
			if err := svc.DeleteQuestion(q.ID, 0, "tutor"); !errors.Is(err, ErrQuestionNotOwned) {
				t.Fatalf("取不到身份时删除应拒, got %v", err)
			}
		})
	}
}

// TestOwnership_PredicateIgnoresCourseClaim 两套谓词互不引用（规格硬约束 3）。
//
// 判据：给讲师认领一门课（认领行存在），他仍改不了别人写的题；反之，没认领任何课，
// 他照样能改自己写的题。这条测试的价值全在**负向**——它拦住「顺手把两个谓词统一起来」的改动。
func TestOwnership_PredicateIgnoresCourseClaim(t *testing.T) {
	svc, db := newQuestionBankSvc(t)

	// 讲师 otherTutor 认领了一门课（认领行与本判据无关，只是「有认领」这个事实存在）
	course := testutil.SeedCourse(t, db, "无关课程")
	tutor := testutil.SeedTutor(t, db, "ownership_tutor", "x")
	if err := db.Create(&model.TutorCourseClaim{
		TutorID: tutor.TutorID, CourseID: course.CourseID,
		Source: model.ClaimSourceSelf, CreatedAt: testutil.Now(),
	}).Error; err != nil {
		t.Fatalf("造认领行失败: %v", err)
	}

	// 有认领 ≠ 能改别人的题
	othersQ := seedOwnedQuestion(t, db, ptrInt(otherTutor))
	content := "别人写的题"
	if _, err := svc.UpdateQuestion(othersQ.ID, QuestionUpdateInput{Content: &content}, tutor.TutorID, "tutor"); !errors.Is(err, ErrQuestionNotOwned) {
		t.Fatalf("认领了课程不代表能改别人的题, got %v", err)
	}

	// 没有认领 ≠ 不能改自己的题（这位讲师一行认领都没有）
	ownQ := seedOwnedQuestion(t, db, ptrInt(ownerTutor))
	if _, err := svc.UpdateQuestion(ownQ.ID, QuestionUpdateInput{Content: &content}, ownerTutor, "tutor"); err != nil {
		t.Fatalf("没认领任何课也应能改自己的题, got %v", err)
	}
	_ = zap.NewNop
}
