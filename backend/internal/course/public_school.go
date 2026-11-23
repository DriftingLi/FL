// 本文件：独立公开网校的公开读取口径（#2753 实施地图节点 B）。
//
// 与 public_detail.go 的分工：那边是「学员站内置的公开课程详情」（`GET /api/courses/:id`，
// 属于既有学员 Web 的面），这边是**独立公开网校专用**的读面（`/api/public-school/*`）。
// 两者判据同源、DTO 不同 —— 独立网校需要分类带公开课计数（用于隐藏空项）与按关键词
// 先筛后分页，而既有公开详情不需要这些，硬塞进去会让旧端点跟着变形状。
//
// 判据（#2753 节点 A 的建议口径，用户已拍板「按建议口径」）：
// 公开侧只认**已启用且映射到有效证件的一级培训分类**。
//   - training_category.status = 1
//   - training_category.legacy_credential_id IS NOT NULL
//   - 对应 credential 行存在且 status = 1
//
// 为什么不是「已启用分类」就够：日常管理仍允许创建**无证件映射**的独立分类（迁移 69 只做
// 旧证件的一对一初始化，不限制日后新建），本机预览库里「叉车维修」「安全操作」两个分类就是
// 这种形态 —— 它们挂着已发布课程，但对不上任何证件。公开网校按证件口径找课，这类分类
// 不该出现在公开目录里，否则「按证件找课」这一承诺当场失真。
//
// 为什么空项隐藏：已启用且映射有效的 8 个证件里，本机预览库只有 1 个下有公开课（其余 7 个
// 是空项）。把它们全部渲染成筛选项，访客点进去必然是空态 —— 那是把「没有课程」伪装成
// 「有这一类」。空项在服务端隐藏，前端不必自己过滤（判据只有一处）。
//
// **边界**：本文件的口径只施加在公开网校读面。既有学员端的练习 / 真题 / 投稿 / 历史分类
// 规则继续独立（`scope.RequireActiveCategory` 等不动），账号的 current_credential_id 也不
// 参与找课 —— 「公开找课」与「账号的当前证件」是两件事。
package course

import (
	"context"
	"strings"

	"gorm.io/gorm"

	"forklift-training/internal/model"
	"forklift-training/pkg/paging"
	"forklift-training/pkg/response"
)

// PublicSchoolCategoryDTO 公开网校的分类筛选项（带公开课计数，空项已在服务端隐藏）。
type PublicSchoolCategoryDTO struct {
	ID          int    `json:"id"`
	Name        string `json:"name"`
	CourseCount int64  `json:"course_count"`
}

// PublicSchoolCategoryListDTO 公开网校分类列表响应 {"categories": [...]}。
type PublicSchoolCategoryListDTO struct {
	Categories []PublicSchoolCategoryDTO `json:"categories" nullability:"nonnil"`
}

// PublicSchoolCourseSummaryDTO 公开网校目录行：只含公开承诺的字段（无章节正文/无私有数据）。
type PublicSchoolCourseSummaryDTO struct {
	CourseID int    `json:"course_id"`
	Name     string `json:"name"`
	// CourseType 课程主类型（#2760）：video / illustrated。目录行必须出这一格——
	// 访客在「全部课程」里也要看得出这门课是视频还是图文，否则分流入口之外就没有分类线索。
	CourseType  string                  `json:"course_type"`
	Description string                  `json:"description"`
	CoverImage  string                  `json:"cover_image"`
	Category    PublicCourseCategoryDTO `json:"category" gorm:"embedded;embeddedPrefix:category_"`
	PointsPrice *int                    `json:"points_price,omitempty" extensions:"x-optional"`
}

// PublicSchoolCoursePageDTO 公开网校目录分页信封（方言 pages，与既有公开目录一致）。
type PublicSchoolCoursePageDTO struct {
	Courses []PublicSchoolCourseSummaryDTO `json:"courses" nullability:"nonnil"`
	Page    int                            `json:"page"`
	Pages   int                            `json:"pages"`
	Total   int64                          `json:"total"`
}

// PublicSchoolCourseDetailDTO 公开网校详情：公开字段 + 已发布章节目录。
// PointsPrice 缺省表示免费（列约束是 IS NULL OR > 0，免费的落点是 NULL）。
type PublicSchoolCourseDetailDTO struct {
	CourseID int    `json:"course_id"`
	Name     string `json:"name"`
	// CourseType 课程主类型（#2760）：详情页要能显示「这是一门视频课」。
	CourseType   string                   `json:"course_type"`
	Description  string                   `json:"description"`
	CoverImage   string                   `json:"cover_image"`
	Category     PublicCourseCategoryDTO  `json:"category"`
	PointsPrice  *int                     `json:"points_price,omitempty" extensions:"x-optional"`
	ChapterCount int                      `json:"chapter_count"`
	Chapters     []PublicCourseChapterDTO `json:"chapters" nullability:"nonnil"`
}

// publicSchoolListRequest 目录查询参数（关键词与分类都在**服务端**收口：先筛后分页）。
type publicSchoolListRequest struct {
	page       int
	pageSize   int
	categoryID *int
	keyword    string
	// courseType 课程主类型筛选（#2760）：空串 = 全部课程（首页/「全部课程」入口）。
	// 非空时必须落在 ValidCourseTypes 内（handler 侧已校验，见 public-school 的 400）。
	courseType string
}

// publicSchoolCategoryColumns 两个读面共用的分类列投影（别名固定为 training_category）。
const publicSchoolCategoryColumns = "training_category.id AS category_id, training_category.name AS category_name"

// publicSchoolCategoryScope 公开网校的分类资格谓词 —— 本包内**唯一**一处定义，
// 分类选项 / 目录 / 详情三个读面都从这里取，避免「列表能进、直达详情却 404」的分叉。
//
// 表别名固定为 training_category / credential：调用点必须用这两个别名 join，
// 不用别名（如 t / c）是为了让这几条 join 在三个调用点逐字一致。
func publicSchoolCategoryScope(db *gorm.DB) *gorm.DB {
	return db.
		Joins("JOIN training_category ON training_category.id = course.category_id").
		Joins("JOIN credential ON credential.id = training_category.legacy_credential_id").
		Where("training_category.status = ?", 1).
		Where("credential.status = ?", 1)
}

// publicSchoolCourseScope 公开网校的课程资格谓词：已发布 + 分类满足上面的口径。
// 与 `PublishedCategoryCourseScope`（只要求「分类存在」）是**两套口径**：那个是既有学员端
// 读面的判据（ADR-0058），本函数是公开网校的判据；后者更严，且只服务公开网校。
func publicSchoolCourseScope(db *gorm.DB) *gorm.DB {
	return publicSchoolCategoryScope(db.Where("course.status = ?", 1))
}

// escapeLikePattern 转义 LIKE 的通配符：关键词里的 % 与 _ 必须按字面匹配。
// 不转义的话访客输入一个 `%` 就能把整库刷出来（它是搜索框，不是 SQL 控制台）。
func escapeLikePattern(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// publicSchoolKeywordHit 关键词匹配的 SQL 片段：公开名称 ∪ 公开简介，**不分大小写**、
// LIKE 元字符按字面处理。
//
// 为什么是 `LOWER(...) LIKE ? ESCAPE '\'` 而不是 `ILIKE`：本仓测试用内存 SQLite、生产用
// PostgreSQL，而 SQLite 没有 ILIKE（实测直接 500）。`LOWER` 两侧 + 显式 `ESCAPE`
// 是 internal/search 早已采用的跨引擎写法，这里沿用同一形状 —— 换引擎不改判据。
const publicSchoolKeywordHit = "(LOWER(course.name) LIKE ? ESCAPE '\\' OR LOWER(course.description) LIKE ? ESCAPE '\\')"

// ListPublicSchoolCategories 公开网校分类选项：已启用 + 映射有效证件 + **只含公开课 > 0 的分类**。
// 排序沿用分类行的 sort_order / id（迁移 69 从证件初始化，公开显示名与排序直接沿用）。
//
// courseType 非空时（#2760 US-4），计数与「空项隐藏」都按该类型收口：视频课程页的证件下拉
// 只列**有视频课**的证件。这是「少遇到空结果」那条 user story 的落点——若类型页沿用了
// 全部课程的计数，访客会在下拉里看到「叉车司机N1证（6）」然后点进去发现只有 2 门视频课。
func (s *Service) ListPublicSchoolCategories(ctx context.Context, courseType string) (*PublicSchoolCategoryListDTO, error) {
	type row struct {
		ID          int    `gorm:"column:id"`
		Name        string `gorm:"column:name"`
		CourseCount int64  `gorm:"column:course_count"`
	}
	// 计数口径与目录读面共用同一个课程资格谓词（publicSchoolCourseScope）：两处不可能漂移。
	// 先按分类聚合公开课，再 JOIN 回分类表 —— 计数为 0 的分类自然被 INNER JOIN 滤掉（= 空项隐藏）。
	countQuery := publicSchoolCourseScope(s.db.WithContext(ctx).Table("course")).
		Select("course.category_id AS category_id, COUNT(*) AS course_count").
		Group("course.category_id")
	if courseType != "" {
		countQuery = countQuery.Where("course.course_type = ?", courseType)
	}
	var rows []row
	err := s.db.WithContext(ctx).Table("training_category").
		Select("training_category.id AS id, training_category.name AS name, counts.course_count AS course_count").
		Joins("JOIN credential ON credential.id = training_category.legacy_credential_id").
		Joins("JOIN (?) AS counts ON counts.category_id = training_category.id", countQuery).
		Where("training_category.status = ?", 1).
		Where("credential.status = ?", 1).
		Order("training_category.sort_order, training_category.id").
		Find(&rows).Error
	if err != nil {
		return nil, err
	}
	items := make([]PublicSchoolCategoryDTO, 0, len(rows))
	for i := range rows {
		items = append(items, PublicSchoolCategoryDTO{ID: rows[i].ID, Name: rows[i].Name, CourseCount: rows[i].CourseCount})
	}
	return &PublicSchoolCategoryListDTO{Categories: items}, nil
}

// ListPublicSchoolCourses 公开网校目录：分类筛选 + 关键词（公开名称与简介）+ 先筛后分页。
// category_id 指向不合格分类时结果为空集（不 404）—— 目录是发现面，空集是可表达的事实；
// 「不可见课程按 404」只适用于**按 ID 取详情**。同一门课「列表里不可见」与「直达详情 404」
// 由同一个 courseScope 保证一致（节点 B 的显式要求）。
func (s *Service) ListPublicSchoolCourses(ctx context.Context, req publicSchoolListRequest) (*PublicSchoolCoursePageDTO, error) {
	items, total, page, pageSize, err := paging.QueryWithScan[PublicSchoolCourseSummaryDTO](
		s.db.WithContext(ctx), req.page, req.pageSize, 12, 60,
		"course.sort_order, course.created_at DESC, course.course_id DESC",
		func(db *gorm.DB) *gorm.DB {
			query := publicSchoolCourseScope(db.Table("course")).
				Select("course.course_id, course.name, course.course_type, course.description, course.cover_image, course.points_price, " +
					publicSchoolCategoryColumns)
			// 类型筛选在分页前应用（#2760）：与分类、关键词同一层，不分先后。
			if req.courseType != "" {
				query = query.Where("course.course_type = ?", req.courseType)
			}
			if req.categoryID != nil {
				query = query.Where("course.category_id = ?", *req.categoryID)
			}
			if keyword := strings.TrimSpace(req.keyword); keyword != "" {
				// 关键词只匹配**公开**字段（名称与简介）：匹配章节正文会泄漏未公开内容，
				// 也会让「搜得到但读不到」成为常态。
				pattern := "%" + escapeLikePattern(strings.ToLower(keyword)) + "%"
				query = query.Where(publicSchoolKeywordHit, pattern, pattern)
			}
			return query
		},
	)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []PublicSchoolCourseSummaryDTO{}
	}
	return &PublicSchoolCoursePageDTO{Courses: items, Page: page, Pages: response.PageCount(total, pageSize), Total: total}, nil
}

// publicSchoolCourseRow 详情读面的行投影（课程字段 + 分类，一次查询取回）。
type publicSchoolCourseRow struct {
	CourseID     int    `gorm:"column:course_id"`
	Name         string `gorm:"column:name"`
	CourseType   string `gorm:"column:course_type"`
	Description  string `gorm:"column:description"`
	CoverImage   string `gorm:"column:cover_image"`
	PointsPrice  *int   `gorm:"column:points_price"`
	CategoryID   int    `gorm:"column:category_id"`
	CategoryName string `gorm:"column:category_name"`
}

// GetPublicSchoolCourseDetail 公开网校详情：课程公开字段 + 已发布章节目录。
// 未发布 / 分类不合格 / 不存在统一按 ErrCourseNotVisible 返回（三态不可分辨，不泄漏是哪一态）。
func (s *Service) GetPublicSchoolCourseDetail(ctx context.Context, courseID int) (*PublicSchoolCourseDetailDTO, error) {
	db := s.db.WithContext(ctx)
	var item publicSchoolCourseRow
	// 一次查询把课程与分类一起取回：分两次取会出现「课程可见但分类随后被停用」的时序窗口，
	// 那时第二次查询失败会让详情答 500 而不是 404 —— 一次 join 让判据原子。
	query := publicSchoolCourseScope(db.Table("course")).
		Select("course.course_id, course.name, course.course_type, course.description, course.cover_image, course.points_price, "+
			publicSchoolCategoryColumns).
		Where("course.course_id = ?", courseID)
	if err := fetchRow(query, &item, ErrCourseNotVisible); err != nil {
		return nil, err
	}
	chapters := make([]PublicCourseChapterDTO, 0)
	if err := db.Model(&model.Chapter{}).
		Select("chapter.chapter_id, chapter.title, chapter.order_num, published.content_type, chapter.duration").
		Joins("JOIN chapter_content_slot AS published ON published.chapter_id = chapter.chapter_id AND published.slot = ?", model.ChapterReviewPublished).
		Where("chapter.course_id = ?", courseID).
		Order("chapter.order_num, chapter.chapter_id").Find(&chapters).Error; err != nil {
		return nil, err
	}
	return &PublicSchoolCourseDetailDTO{
		CourseID: item.CourseID, Name: item.Name, CourseType: item.CourseType,
		Description: item.Description, CoverImage: item.CoverImage,
		Category:    PublicCourseCategoryDTO{ID: item.CategoryID, Name: item.CategoryName},
		PointsPrice: item.PointsPrice, ChapterCount: len(chapters), Chapters: chapters,
	}, nil
}
