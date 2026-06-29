// Package api 实现 HTTP handlers。
package api

import (
	"strconv"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"forklift-training/internal/config"
	"forklift-training/internal/middleware"
	"forklift-training/internal/service"
	"forklift-training/pkg/response"
)

// RegisterAdminRoutes 注册 /api/admin 蓝图（管理员后台）。
// 对应 Python app/api/admin.py。
func RegisterAdminRoutes(rg *gin.RouterGroup, cfg *config.Config, db *gorm.DB) {
	adminSvc := service.NewAdminService(db)
	courseSvc := service.NewAdminCourseService(db)
	authSvc := service.NewAuthService(db, cfg.JWTSecretKey, cfg.JWTExpiry())

	g := rg.Group("/admin", middleware.JWTAuth(cfg), middleware.RoleRequired("admin"))

	// ===== 课程管理 =====

	// GET /api/admin/courses  课程列表
	g.GET("/courses", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		keyword := c.Query("keyword")
		category := c.Query("category")
		response.Success(c, courseSvc.GetCourses(page, pageSize, keyword, category))
	})

	// POST /api/admin/course  创建课程
	g.POST("/course", func(c *gin.Context) {
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := courseSvc.CreateCourse(data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "课程创建成功", result)
	})

	// GET /api/admin/course/:course_id  课程详情
	g.GET("/course/:course_id", func(c *gin.Context) {
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		result, err := courseSvc.GetCourseDetail(courseID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.Success(c, result)
	})

	// PUT /api/admin/course/:course_id  更新课程
	g.PUT("/course/:course_id", func(c *gin.Context) {
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := courseSvc.UpdateCourse(courseID, data)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "课程更新成功", result)
	})

	// DELETE /api/admin/course/:course_id  删除课程
	g.DELETE("/course/:course_id", func(c *gin.Context) {
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		result, err := courseSvc.DeleteCourse(courseID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "课程删除成功", result)
	})

	// POST /api/admin/course/:course_id/chapter  创建章节
	g.POST("/course/:course_id/chapter", func(c *gin.Context) {
		courseID, err := strconv.Atoi(c.Param("course_id"))
		if err != nil {
			response.BadRequest(c, "课程ID无效")
			return
		}
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := courseSvc.CreateChapter(courseID, data)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "章节创建成功", result)
	})

	// PUT /api/admin/chapter/:chapter_id  更新章节
	g.PUT("/chapter/:chapter_id", func(c *gin.Context) {
		chapterID, err := strconv.Atoi(c.Param("chapter_id"))
		if err != nil {
			response.BadRequest(c, "章节ID无效")
			return
		}
		var data map[string]interface{}
		if err := c.ShouldBindJSON(&data); err != nil {
			response.BadRequest(c, "请求数据无效")
			return
		}
		result, err := courseSvc.UpdateChapter(chapterID, data)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "章节更新成功", result)
	})

	// DELETE /api/admin/chapter/:chapter_id  删除章节
	g.DELETE("/chapter/:chapter_id", func(c *gin.Context) {
		chapterID, err := strconv.Atoi(c.Param("chapter_id"))
		if err != nil {
			response.BadRequest(c, "章节ID无效")
			return
		}
		result, err := courseSvc.DeleteChapter(chapterID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "章节删除成功", result)
	})

	// POST /api/admin/course/generate-content  异步生成课程内容（暂未实现，返回 503）
	g.POST("/course/generate-content", func(c *gin.Context) {
		var req struct {
			CourseID   int   `json:"course_id"`
			ChapterIDs []int `json:"chapter_ids"`
		}
		if err := c.ShouldBindJSON(&req); err != nil || req.CourseID == 0 {
			response.BadRequest(c, "请选择课程")
			return
		}
		response.ServerError(c, "异步内容生成功能尚未迁移至 Go 版本")
	})

	// GET /api/admin/course/generate-content/:task_id  异步任务状态
	g.GET("/course/generate-content/:task_id", func(c *gin.Context) {
		response.NotFound(c, "异步内容生成功能尚未迁移至 Go 版本")
	})

	// ===== 学员管理 =====

	// GET /api/admin/students  学员列表
	g.GET("/students", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		keyword := c.Query("keyword")
		response.Success(c, adminSvc.GetStudents(page, pageSize, keyword))
	})

	// POST /api/admin/student  添加学员
	g.POST("/student", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Name     string `json:"name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if req.Username == "" || req.Password == "" || req.Name == "" {
			response.BadRequest(c, "用户名、密码和姓名不能为空")
			return
		}
		result, err := authSvc.StudentRegister(req.Username, req.Password, req.Name)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "学员添加成功", result)
	})

	// DELETE /api/admin/student/:student_id  删除学员
	g.DELETE("/student/:student_id", func(c *gin.Context) {
		studentID, err := strconv.Atoi(c.Param("student_id"))
		if err != nil {
			response.BadRequest(c, "学员ID无效")
			return
		}
		result, err := adminSvc.DeleteStudent(studentID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "学员删除成功", result)
	})

	// ===== 导师管理 =====

	// GET /api/admin/tutors  导师列表
	g.GET("/tutors", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		keyword := c.Query("keyword")
		response.Success(c, adminSvc.GetTutors(page, pageSize, keyword))
	})

	// POST /api/admin/tutor  添加导师
	g.POST("/tutor", func(c *gin.Context) {
		var req struct {
			Username string `json:"username"`
			Password string `json:"password"`
			Name     string `json:"name"`
		}
		if err := c.ShouldBindJSON(&req); err != nil {
			response.BadRequest(c, "请求参数错误")
			return
		}
		if req.Username == "" || req.Password == "" || req.Name == "" {
			response.BadRequest(c, "用户名、密码和姓名不能为空")
			return
		}
		result, err := authSvc.TutorRegister(req.Username, req.Password, req.Name)
		if err != nil {
			response.BadRequest(c, err.Error())
			return
		}
		response.Created(c, "导师添加成功", result)
	})

	// DELETE /api/admin/tutor/:tutor_id  删除导师
	g.DELETE("/tutor/:tutor_id", func(c *gin.Context) {
		tutorID, err := strconv.Atoi(c.Param("tutor_id"))
		if err != nil {
			response.BadRequest(c, "导师ID无效")
			return
		}
		result, err := adminSvc.DeleteTutor(tutorID)
		if err != nil {
			response.NotFound(c, err.Error())
			return
		}
		response.SuccessWithMsg(c, "导师删除成功", result)
	})

	// ===== 统计与实操记录 =====

	// GET /api/admin/statistics  统计看板
	g.GET("/statistics", func(c *gin.Context) {
		response.Success(c, adminSvc.GetStatistics())
	})

	// GET /api/admin/practice-records  实操记录列表
	g.GET("/practice-records", func(c *gin.Context) {
		page := atoiDefault(c.Query("page"), 1)
		pageSize := atoiDefault(c.Query("page_size"), 10)
		practiceType := c.Query("practice_type")
		var studentID *int
		if s := c.Query("student_id"); s != "" {
			if id, err := strconv.Atoi(s); err == nil {
				studentID = &id
			}
		}
		response.Success(c, adminSvc.GetAdminPracticeRecords(page, pageSize, practiceType, studentID))
	})
}
