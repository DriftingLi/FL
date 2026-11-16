// 本文件：课程认领的域具名谓词与认领读写（#2596 / 规格 B2–B7）。
//
// 为什么是域谓词而不是能力键：能力键回答「这个人能不能用讲师工作区」，回答不了
// 「这个人能不能改**这门课**的章节」。后者是数据级不变式，只能由域层表达（ADR-0047
// 已定能力与数据级不变式正交）。同理它也不是「归属隔离」——认领是多对多，
// 两位讲师共认同一门课、各写各的章节，谁也不独占。
//
// 与题库域写隔离（块 C）互不引用：认领不参与题目归属判定，题目归属也不看认领。
// 这条是规格里点名最容易被「顺手统一」掉的地方。
package course

import (
	"errors"
	"time"

	"gorm.io/gorm"

	"forklift-training/internal/dberr"
	"forklift-training/internal/model"
	"forklift-training/internal/timefmt"
)

var (
	// ErrCourseClaimRequired 未认领该课程（讲师面写操作的拒绝哨兵）。
	ErrCourseClaimRequired = errors.New("请先认领本课程")
	// ErrCourseAlreadyClaimed 本人已认领该课程（重复认领，映射 409）。
	ErrCourseAlreadyClaimed = errors.New("你已认领本课程")
	// ErrCourseNotClaimed 退出认领时本人并未认领该课程（幂等边界，映射 404 档）。
	ErrCourseNotClaimed = errors.New("你还没有认领本课程")
)

// RequireCourseClaim 域谓词：要求讲师对课程 courseID 持有认领。未认领返回
// ErrCourseClaimRequired，由 handler 映射 403。
//
// **管理员不经本谓词**：阶段一已定两个写键不授管理员，管理端的章节写走 course.manage
// 那条独立路径（handler_admin.go），故这里不需要 admin 旁路分支——多一个分支就多一处
// 「管理员顺手绕过」的口子。
func RequireCourseClaim(db *gorm.DB, tutorID, courseID int) error {
	ok, err := HasCourseClaim(db, tutorID, courseID)
	if err != nil {
		return err
	}
	if !ok {
		return ErrCourseClaimRequired
	}
	return nil
}

// RequireChapterClaim 域谓词：按**章节**要编辑权——内部由章节查课程再查认领行。
// 讲师面写路径拿到的都是章节 id（附件行也挂在章节上），故它是主要入口。
//
// 章节不存在时返回 course.ErrChapterNotFound（「真不存在」唯一载体），不把「查不动」
// 答成「不存在」（ADR-0064 决策 1）。
func RequireChapterClaim(db *gorm.DB, tutorID, chapterID int) error {
	courseID, err := courseIDOfChapter(db, chapterID)
	if err != nil {
		return err
	}
	return RequireCourseClaim(db, tutorID, courseID)
}

// courseIDOfChapter 由章节 id 取课程 id。
func courseIDOfChapter(db *gorm.DB, chapterID int) (int, error) {
	var chapter model.Chapter
	if err := db.Select("chapter_id", "course_id").First(&chapter, chapterID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, ErrChapterNotFound
		}
		return 0, err // 查不动不得被读成「不存在」（ADR-0064 决策 1）
	}
	return chapter.CourseID, nil
}

// HasCourseClaim 讲师是否认领了该课程。
func HasCourseClaim(db *gorm.DB, tutorID, courseID int) (bool, error) {
	var n int64
	if err := db.Model(&model.TutorCourseClaim{}).
		Where("tutor_id = ? AND course_id = ?", tutorID, courseID).
		Count(&n).Error; err != nil {
		return false, err
	}
	return n > 0, nil
}

// ClaimCourse 讲师自助认领（B3/B5）：已认领返回 ErrCourseAlreadyClaimed 而不是静默成功——
// 前端要把「你已经在里面了」和「刚成功」分开显示。并发重复认领由唯一索引兜底，
// 冲突同样翻译成 ErrCourseAlreadyClaimed（否则并发时一条路径 409、另一条 500）。
func ClaimCourse(db *gorm.DB, tutorID, courseID int) error {
	var c model.Course
	if err := db.Select("course_id").First(&c, courseID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return model.ErrCourseNotFound
		}
		return err
	}
	row := model.TutorCourseClaim{TutorID: tutorID, CourseID: courseID, Source: model.ClaimSourceSelf}
	if err := db.Create(&row).Error; err != nil {
		if dberr.IsDuplicateError(err) {
			return ErrCourseAlreadyClaimed
		}
		return err
	}
	return nil
}

// ReleaseCourseClaim 讲师退出认领（B6）：只断**今后**的编辑权——章节作者归属保留
// （ADR-0095 决策二），在审快照照常走完审核。最后一名认领者退出后课程回无主状态，
// 对全体讲师开放认领（本函数只要删掉自己那行就是这个语义，不需要额外状态位）。
//
// 未认领时返回 ErrCourseNotClaimed：删除 0 行不冒充成功。
func ReleaseCourseClaim(db *gorm.DB, tutorID, courseID int) error {
	res := db.Where("tutor_id = ? AND course_id = ?", tutorID, courseID).
		Delete(&model.TutorCourseClaim{})
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrCourseNotClaimed
	}
	return nil
}

// CourseClaimant 课程共认讲师（供章节页展示名单、管理端分配面板复用）。
//
// IsMe 由服务端逐行标出（#2596 D1）：前端若自己拿 user_id 去比对，会多出一条「我是谁」的
// 判定链（登录响应、/auth/me、令牌 uid 三处可能不同步），而这本来是服务端一眼可知的事实。
type CourseClaimant struct {
	TutorID   int    `json:"tutor_id"`
	TutorName string `json:"tutor_name"`
	Source    string `json:"source"`
	CreatedAt string `json:"created_at"`
	IsMe      bool   `json:"is_me"`
}

// CourseClaimantsResult 认领讲师名单信封（tutor_id/name/source/created_at 四列）。
//
// ClaimedByMe 由服务端回答「当前登录讲师在不在名单里」：前端若自己拿 user_id 去比对，
// 会多出一条「我是谁」的判定链（登录响应、/auth/me、令牌 uid 三处都可能不同步），
// 而这本来就是一个服务端一眼可知的事实。
type CourseClaimantsResult struct {
	Claimants   []CourseClaimant `json:"claimants" nullability:"nonnil"`
	ClaimedByMe bool             `json:"claimed_by_me"`
}

// ListCourseClaimants 列出某课程的认领讲师（按认领时间升序——先来的排在前面，
// 与「谁在编」的直觉一致）。
func ListCourseClaimants(db *gorm.DB, courseID int) ([]CourseClaimant, error) {
	var rows []struct {
		TutorID   int
		TutorName string
		Source    string
		CreatedAt time.Time
	}
	if err := db.Model(&model.TutorCourseClaim{}).
		Select("tutor_course_claim.tutor_id, tutor.name AS tutor_name, tutor_course_claim.source, tutor_course_claim.created_at").
		Joins("JOIN tutor ON tutor.tutor_id = tutor_course_claim.tutor_id").
		Where("tutor_course_claim.course_id = ?", courseID).
		Order("tutor_course_claim.created_at ASC, tutor_course_claim.claim_id ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]CourseClaimant, 0, len(rows))
	for _, r := range rows {
		out = append(out, CourseClaimant{
			TutorID:   r.TutorID,
			TutorName: r.TutorName,
			Source:    r.Source,
			CreatedAt: timefmt.FormatISO(r.CreatedAt),
		})
	}
	return out, nil
}

// BatchCourseClaimStats 一次查询一批课程的认领统计：course_id → (认领人数, 我是否在其中)。
// 消除课程列表逐行查的 N+1（规格 D1 明写「一次联表聚合算出，不逐行查」）。
//
// 用 SUM(CASE WHEN ...) 而不是两条查询：人数与「其中有没有我」来自同一批行，
// 分两条查会在两次查询之间被写操作撕裂（同一个人可能算出「3 人在编但我不在」）。
func BatchCourseClaimStats(db *gorm.DB, courseIDs []int, tutorID int) (map[int]int, map[int]bool, error) {
	counts := make(map[int]int, len(courseIDs))
	mine := make(map[int]bool, len(courseIDs))
	if len(courseIDs) == 0 {
		return counts, mine, nil
	}
	var rows []struct {
		CourseID int
		N        int
		Mine     int
	}
	if err := db.Model(&model.TutorCourseClaim{}).
		Select("course_id, COUNT(*) AS n, SUM(CASE WHEN tutor_id = ? THEN 1 ELSE 0 END) AS mine", tutorID).
		Where("course_id IN ?", courseIDs).
		Group("course_id").
		Scan(&rows).Error; err != nil {
		return nil, nil, err
	}
	for _, r := range rows {
		counts[r.CourseID] = r.N
		mine[r.CourseID] = r.Mine > 0
	}
	return counts, mine, nil
}
