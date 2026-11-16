// 本文件：管理端课程代分配（#2597 / 规格 D4）——讲师管理页「分配课程」抽屉的后端。
//
// 为什么落在这里而不是 internal/course：这是**讲师管理**动作（「给这位讲师分配哪些课」），
// 入口是讲师管理页、能力键是 tutor.manage。课程域提供认领事实与谓词，管理域提供
// 「按讲师维度批量改认领」这个用例——把用例塞进课程域会让课程域反过来知道讲师管理页。
//
// 谓词与 INSERT 都委托 internal/course（ClaimCourse / ReleaseCourseClaim）：
// 认领是课程域的事实，管理端只是另一个调用方（与讲师自助写同一张表、同一份冲突语义）。
package admin

import (
	"context"
	"errors"
	"time"

	"forklift-training/internal/course"
	"forklift-training/internal/model"
	"forklift-training/internal/timefmt"
)

// TutorCourseAssignInput 代分配 / 移除的入参（批量）。
//
// 为什么是批量而不是单条：抽屉是「多选加入 / 多选移除」，一条请求对应一次用户动作。
// 拆成 N 条单条请求会让「第 3 条失败」变成一个半完成状态，前端还得自己回滚前两条。
type TutorCourseAssignInput struct {
	CourseIDs []int `json:"course_ids"`
}

// TutorCourseAssignResult 代分配 / 移除结果。
//
// Applied / Skipped 两列而不是只回「成功」：管理员需要知道「我选的 5 门里，
// 有 2 门本来就已经分配了」。这不是错误（幂等成功），但也不是「刚刚做了 5 件事」。
type TutorCourseAssignResult struct {
	Applied int `json:"applied"` // 本次真正新建或改动的认领行数
	Skipped int `json:"skipped"` // 已经是目标状态、无需改动的行数
}

// AssignCourses 管理端把一批课程分配给讲师（D4）。
//
// 语义要点（与 #2597 票内评论的拍板一致）：
//   - 来源记 'admin'（B1：与讲师自助认领可区分，供审计）；
//   - **重复分配幂等**：已在认领关系中不重复写行，计入 Skipped；
//   - **讲师自助认领过的课，管理员代分配时把 source 升级为 admin 并计入 Applied**——
//     那是把一条自助认领转成正式分配，与「讲师自己重复点按钮」不是同一件事。
//
// 讲师不存在时返回 model.ErrTutorNotFound（404）：不能给一个不存在的讲师建认领行
// （表上有外键，硬插会 500；那会让「讲师 id 打错了」看起来像服务故障）。
func (s *Service) AssignCourses(ctx context.Context, tutorID int, in TutorCourseAssignInput) (*TutorCourseAssignResult, error) {
	if err := s.requireTutorExists(ctx, tutorID); err != nil {
		return nil, err
	}
	result := &TutorCourseAssignResult{}
	for _, courseID := range in.CourseIDs {
		// 分配前先看现状：
		//   - 已存在且来源已是 admin → 真正的「无事可做」，**一行都不写**（幂等的字面含义）；
		//   - 已存在但来源是 self → 交给 AssignCourse 升级来源（计入 Applied）；
		//   - 不存在 → AssignCourse 新建（计入 Applied）。
		exists, source, err := s.courseClaimState(ctx, tutorID, courseID)
		if err != nil {
			return nil, err
		}
		if exists && source == model.ClaimSourceAdmin {
			result.Skipped++
			continue
		}
		if err := course.AssignCourse(s.db.WithContext(ctx), tutorID, courseID); err != nil {
			if errors.Is(err, model.ErrCourseNotFound) {
				// 单门课不存在不整批失败：抽屉里列的是当前课程，被并发删掉一门
				// 不该让其余 4 门的分配一起回滚。计入 Skipped 并继续。
				result.Skipped++
				continue
			}
			return nil, err
		}
		result.Applied++
	}
	return result, nil
}

// RemoveCourses 管理端从讲师身上移除一批课程认领（D4）。
//
// **不回溯**（B6/D4）：只断今后的编辑权。章节作者归属照旧、在审内容照常审完——
// 那些是 ReleaseCourseClaim 的既有语义，本函数不额外做什么，也不该做什么。
//
// 本来就没认领的课计入 Skipped 而不是报错：管理员在抽屉里勾选的可能是他以为已分配、
// 实际没有的课，这种「目标状态已达成」不是失败。
func (s *Service) RemoveCourses(ctx context.Context, tutorID int, in TutorCourseAssignInput) (*TutorCourseAssignResult, error) {
	if err := s.requireTutorExists(ctx, tutorID); err != nil {
		return nil, err
	}
	result := &TutorCourseAssignResult{}
	for _, courseID := range in.CourseIDs {
		err := course.ReleaseCourseClaim(s.db.WithContext(ctx), tutorID, courseID)
		if err == nil {
			result.Applied++
			continue
		}
		if errors.Is(err, course.ErrCourseNotClaimed) {
			result.Skipped++
			continue
		}
		return nil, err
	}
	return result, nil
}

// requireTutorExists 讲师必须真实存在（代分配的前置，也是 404 的唯一来源）。
func (s *Service) requireTutorExists(ctx context.Context, tutorID int) error {
	var cnt int64
	if err := s.db.WithContext(ctx).Model(&model.Tutor{}).Where("tutor_id = ?", tutorID).Count(&cnt).Error; err != nil {
		return err
	}
	if cnt == 0 {
		return model.ErrTutorNotFound
	}
	return nil
}

// courseClaimState 查一条认领的现状：(是否存在, 来源)。
//
// 为什么要单独查一次而不是看 AssignCourse 的返回值：AssignCourse 对「新建」与
// 「升级来源」都返回 nil，而管理员要看到的 Applied/Skipped 区分依赖这个事实。
// 一次多查一条按主键的 SELECT 换一个准确的读数，值。
func (s *Service) courseClaimState(ctx context.Context, tutorID, courseID int) (bool, string, error) {
	var rows []model.TutorCourseClaim
	if err := s.db.WithContext(ctx).
		Where("tutor_id = ? AND course_id = ?", tutorID, courseID).
		Limit(1).Find(&rows).Error; err != nil {
		return false, "", err
	}
	if len(rows) == 0 {
		return false, "", nil
	}
	return true, rows[0].Source, nil
}

// TutorCourseAssignmentDTO 讲师已分配课程的一条（抽屉里「已分配」标识的数据源）。
type TutorCourseAssignmentDTO struct {
	CourseID   int    `json:"course_id"`
	CourseName string `json:"course_name"`
	Source     string `json:"source"`     // self | admin：抽屉要显示「他自己认领的」还是「管理员分配的」
	CreatedAt  string `json:"created_at"` // 认领时间，按升序排
}

// TutorCourseAssignmentListDTO 讲师已分配课程信封。
//
// 为什么要 CourseIDs 与 Items 两列：前端勾选框只需要 id 集合（O(1) 判断已分配），
// 但抽屉里要显示来源与时间（「他自己认领的」vs「分配给他的」在审计上是两件事）。
// 只给 id 会让前端再发 N 条请求去补来源；只给 items 会让前端每行做一次 find。
type TutorCourseAssignmentListDTO struct {
	CourseIDs []int                      `json:"course_ids" nullability:"nonnil"`
	Items     []TutorCourseAssignmentDTO `json:"items" nullability:"nonnil"`
}

// ListTutorCourseAssignments 列出某讲师已分配的课程（D4 抽屉的已分配标识）。
//
// 只列认领行、不列「他可能能写的全部课程」：抽屉的语义是「这个人被分到了哪些课」，
// 未分配的课由前端从课程列表里减出来（课程列表本来就要拉一次做候选）。
func (s *Service) ListTutorCourseAssignments(ctx context.Context, tutorID int) (*TutorCourseAssignmentListDTO, error) {
	if err := s.requireTutorExists(ctx, tutorID); err != nil {
		return nil, err
	}
	var rows []struct {
		CourseID   int
		CourseName string
		Source     string
		CreatedAt  time.Time
	}
	if err := s.db.WithContext(ctx).Model(&model.TutorCourseClaim{}).
		Select("tutor_course_claim.course_id, course.name AS course_name, tutor_course_claim.source, tutor_course_claim.created_at").
		Joins("JOIN course ON course.course_id = tutor_course_claim.course_id").
		Where("tutor_course_claim.tutor_id = ?", tutorID).
		Order("tutor_course_claim.created_at ASC, tutor_course_claim.claim_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := &TutorCourseAssignmentListDTO{
		CourseIDs: make([]int, 0, len(rows)),
		Items:     make([]TutorCourseAssignmentDTO, 0, len(rows)),
	}
	for _, r := range rows {
		out.CourseIDs = append(out.CourseIDs, r.CourseID)
		out.Items = append(out.Items, TutorCourseAssignmentDTO{
			CourseID:   r.CourseID,
			CourseName: r.CourseName,
			Source:     r.Source,
			CreatedAt:  timefmt.FormatISO(r.CreatedAt),
		})
	}
	return out, nil
}
