// 收藏新增两支的契约（#2067）：职位收藏（含下架口径）与「题目」按题库标签下钻。
//
// 失效口径（票面要求显式写清）：**过滤**，不是标记 —— 职位 closed / forced_offline、或行被删，
// 收藏列表里不出现（与既有五类「目标读不到即不出现」同形状），宁可少一条也不返回坏链接。
package favorite

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/testutil"
)

func TestFavoriteJobAndQuestionTagContract(t *testing.T) {
	t.Parallel()
	testutil.SetTestGinMode()
	db := testutil.NewPostgresDB(t)

	student := testutil.SeedStudent(t, db, "fav_job_stu", "x")
	recruiter := testutil.SeedRecruiter(t, db, "fav_job_rec", "x")

	now := testutil.Now()
	openJob := model.JobPosting{
		RecruiterID: recruiter.ID, Title: "叉车维修技师", Region: "上海",
		Status: "open", PublishedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&openJob).Error; err != nil {
		t.Fatalf("创建职位失败: %v", err)
	}
	closedJob := model.JobPosting{
		RecruiterID: recruiter.ID, Title: "已停止招聘的职位", Region: "杭州",
		Status: "closed", PublishedAt: now, CreatedAt: now, UpdatedAt: now,
	}
	if err := db.Create(&closedJob).Error; err != nil {
		t.Fatalf("创建下架职位失败: %v", err)
	}

	// 题库标签：tagA（sort 1）、tagB（sort 2）；q1 挂 tagA，q2 挂 tagA+tagB。
	tagA := model.QuestionTag{Code: "fav-tag-a", Name: "法规", SortOrder: 1, Status: 1, CreatedAt: now, UpdatedAt: now}
	tagB := model.QuestionTag{Code: "fav-tag-b", Name: "液压", SortOrder: 2, Status: 1, CreatedAt: now, UpdatedAt: now}
	for _, tg := range []*model.QuestionTag{&tagA, &tagB} {
		if err := db.Create(tg).Error; err != nil {
			t.Fatalf("创建标签失败: %v", err)
		}
	}
	q1 := testutil.SeedQuestion(t, db, "single_choice", "收藏题一", "A")
	q2 := testutil.SeedQuestion(t, db, "single_choice", "收藏题二", "B")
	for _, rel := range []model.QuestionTagRelation{
		{QuestionID: q1.ID, TagID: tagA.ID, CreatedAt: now},
		{QuestionID: q2.ID, TagID: tagA.ID, CreatedAt: now},
		{QuestionID: q2.ID, TagID: tagB.ID, CreatedAt: now},
	} {
		if err := db.Create(&rel).Error; err != nil {
			t.Fatalf("建标签关系失败: %v", err)
		}
	}

	cfg := &config.Config{JWTSecretKey: "contract-test-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := newFavoriteContractRouter(t, db, cfg)
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).
		Issue(int(student.ID), student.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	do := func(method, path string, body any) *httptest.ResponseRecorder {
		var buf *bytes.Buffer
		if body != nil {
			raw, _ := json.Marshal(body)
			buf = bytes.NewBuffer(raw)
		} else {
			buf = bytes.NewBuffer(nil)
		}
		req, _ := http.NewRequest(method, path, buf)
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	decodeData := func(w *httptest.ResponseRecorder, out any) {
		t.Helper()
		if w.Code != http.StatusOK && w.Code != http.StatusCreated {
			t.Fatalf("应 2xx, got %d %s", w.Code, w.Body.String())
		}
		var env struct {
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &env); err != nil {
			t.Fatalf("解析信封失败: %v", err)
		}
		if err := json.Unmarshal(env.Data, out); err != nil {
			t.Fatalf("解析 data 失败: %v", err)
		}
	}

	// 职位：收藏成功、字段形状（标题/企业/地区）、重复收藏幂等
	var added FavoriteDTO
	decodeData(do(http.MethodPost, "/api/favorites", map[string]any{"target_type": "job", "target_id": openJob.ID}), &added)
	if added.Title != openJob.Title || added.Company != recruiter.CompanyName || added.Region != "上海" {
		t.Fatalf("职位收藏快照字段不对: %+v", added)
	}
	var again FavoriteDTO
	decodeData(do(http.MethodPost, "/api/favorites", map[string]any{"target_type": "job", "target_id": openJob.ID}), &again)
	if again.FavoriteID != added.FavoriteID {
		t.Fatalf("重复收藏应幂等：first=%d again=%d", added.FavoriteID, again.FavoriteID)
	}

	// 下架职位不可收藏（closed 与不存在同对待）
	if w := do(http.MethodPost, "/api/favorites", map[string]any{"target_type": "job", "target_id": closedJob.ID}); w.Code != http.StatusBadRequest {
		t.Fatalf("下架职位收藏应 400, got %d %s", w.Code, w.Body.String())
	}

	// 列表：职位项在（带企业/地区）
	var jobPage FavoritePageResult
	decodeData(do(http.MethodGet, "/api/favorites?target_type=job", nil), &jobPage)
	if jobPage.Total != 1 || len(jobPage.Favorites) != 1 || jobPage.Favorites[0].Region != "上海" {
		t.Fatalf("职位收藏列表不对: %+v", jobPage)
	}

	// 题目：带标签（按 sort_order 排序）
	decodeData(do(http.MethodPost, "/api/favorites", map[string]any{"target_type": "question", "target_id": q2.ID}), &FavoriteDTO{})
	var qPage FavoritePageResult
	decodeData(do(http.MethodGet, "/api/favorites?target_type=question", nil), &qPage)
	if len(qPage.Favorites) != 1 {
		t.Fatalf("题目收藏应 1 条, got %+v", qPage.Favorites)
	}
	tags := qPage.Favorites[0].Tags
	if len(tags) != 2 || tags[0].TagID != tagA.ID || tags[1].TagID != tagB.ID {
		t.Fatalf("题目收藏应带两个按 sort_order 排好的标签, got %+v", tags)
	}

	// 下钻：按标签过滤（tagB 只命中 q2）
	var tagOnly FavoritePageResult
	decodeData(do(http.MethodGet, "/api/favorites?target_type=question&tag_id="+strconv.Itoa(tagB.ID), nil), &tagOnly)
	if tagOnly.Total != 1 || tagOnly.Favorites[0].TargetID != q2.ID {
		t.Fatalf("按 tagB 下钻应只命中 q2：%+v", tagOnly)
	}

	// tag_id 只对题目有意义：其它类型 400，而不是静默忽略
	if w := do(http.MethodGet, "/api/favorites?target_type=job&tag_id="+strconv.Itoa(tagA.ID), nil); w.Code != http.StatusBadRequest {
		t.Fatalf("非题目类型带 tag_id 应 400, got %d %s", w.Code, w.Body.String())
	}

	// 失效口径 = 过滤：职位一旦 closed，列表里不再出现（收藏行仍在库里）
	if err := db.Model(&model.JobPosting{}).Where("id = ?", openJob.ID).Update("status", "closed").Error; err != nil {
		t.Fatalf("下架失败: %v", err)
	}
	var afterClose FavoritePageResult
	decodeData(do(http.MethodGet, "/api/favorites?target_type=job", nil), &afterClose)
	// 「过滤」的口径落在 items 上：Total 仍是**收藏行数**（既有五类同形状——快照查不到的行照样算进
	// 分页总数，只是不出现在 items 里）。这一条写进票面，免得下游把 Total 当可见条目数。
	if len(afterClose.Favorites) != 0 {
		t.Fatalf("下架后职位收藏不应出现在 items 里, got %+v", afterClose)
	}
}
