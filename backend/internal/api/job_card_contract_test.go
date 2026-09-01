package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/security"
	"forklift-training/internal/service"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestJobCardContract(t *testing.T) {
	gin.SetMode(gin.TestMode)
	db := testutil.NewMemoryDB(t)
	spec := model.Position{Code: "maintenance", Name: "维修", SortOrder: 1, Status: 1}
	if err := db.Create(&spec).Error; err != nil {
		t.Fatalf("创建 specialty 失败: %v", err)
	}
	cred := model.Credential{Code: "forklift_n1", Name: "叉车司机N1", Category: "special_operation", Status: 1}
	if err := db.Create(&cred).Error; err != nil {
		t.Fatalf("创建 credential 失败: %v", err)
	}
	author := model.HrwaiUser{Account: "resume_author", Phone: "13800000201", Username: "简历作者", Status: 1, CreatedAt: testutil.Now()}
	if err := db.Create(&author).Error; err != nil {
		t.Fatalf("创建作者失败: %v", err)
	}
	cfg := &config.Config{JWTSecretKey: "contract-test-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
	r := gin.New()
	apiGroup := r.Group("/api")
	deps := newContractDeps(t, db, cfg)
	st := storage.NewLocalStorage(t.TempDir())
	fileSvc := service.NewFileStore("", st, zap.NewNop())
	jobSvc := service.NewJobCardService(db, fileSvc, zap.NewNop())
	deps.FileSvc = fileSvc
	deps.JobCardSvc = jobSvc
	RegisterJobCardRoutes(apiGroup, deps.RouterDeps(), deps.JobCardSvc, deps.FileSvc)
	token, err := security.NewSession(cfg.JWTSecretKey, time.Hour, security.CookieConfig{}).Issue(int(author.ID), author.Account, "hrwai_user")
	if err != nil {
		t.Fatalf("签发 token 失败: %v", err)
	}
	do := func(method, path string, body any) *httptest.ResponseRecorder {
		var req *http.Request
		if body != nil {
			b, _ := json.Marshal(body)
			req, _ = http.NewRequest(method, path, bytes.NewReader(b))
			req.Header.Set("Content-Type", "application/json")
		} else {
			req, _ = http.NewRequest(method, path, nil)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		return w
	}
	if rec := do(http.MethodGet, "/api/resume", nil); rec.Code != http.StatusNotFound {
		t.Fatalf("未创建简历 GET 应 404, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	payload := map[string]any{"real_name": "张三", "contact_phone": "13900001111", "wechat": "zhangsan_wx", "region": "江苏苏州", "expected_position_id": spec.PositionID, "expected_regions": []string{"江苏苏州", "浙江杭州"}, "salary_min": 8000, "salary_max": 12000, "experience_years": 5, "self_intro": "5 年叉车维修经验", "resume_experiences": []map[string]any{{"company": "A公司", "role": "维修工", "start_month": "2020-01", "end_month": "2023-01", "desc": "维修叉车"}}, "resume_certifications": []map[string]any{{"credential_id": cred.ID, "cert_no": "CERT123", "expire_date": "2028-01-01", "image_urls": []string{}}}, "photos": []string{"https://example.com/a.jpg"}, "resume_file_url": ""}
	rec := do(http.MethodPut, "/api/resume", payload)
	if rec.Code != http.StatusOK {
		t.Fatalf("PUT 创建简历应 200, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	var putResp struct {
		Code int `json:"code"`
		Data struct {
			ContactPhone       string `json:"contact_phone"`
			Visibility         string `json:"visibility"`
			ExpectedPositionID *int   `json:"expected_position_id"`
			RealName           string `json:"real_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &putResp); err != nil {
		t.Fatalf("解析 PUT 响应失败: %v", err)
	}
	if putResp.Data.ContactPhone != "13900001111" {
		t.Fatalf("contact_phone 回填错误: %q", putResp.Data.ContactPhone)
	}
	if putResp.Data.Visibility != "hidden" {
		t.Fatalf("visibility 默认应 hidden, 实际 %q", putResp.Data.Visibility)
	}
	if putResp.Data.ExpectedPositionID == nil || *putResp.Data.ExpectedPositionID != spec.PositionID {
		t.Fatalf("expected_position_id 回填错误: %v", putResp.Data.ExpectedPositionID)
	}
	rec = do(http.MethodGet, "/api/resume", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("GET 已创建简历应 200, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	var getResp struct {
		Code int `json:"code"`
		Data struct {
			ContactPhone         string          `json:"contact_phone"`
			Visibility           string          `json:"visibility"`
			ResumeExperiences    json.RawMessage `json:"resume_experiences"`
			ResumeCertifications json.RawMessage `json:"resume_certifications"`
			Photos               json.RawMessage `json:"photos"`
			ExpectedRegions      json.RawMessage `json:"expected_regions"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("解析 GET 响应失败: %v", err)
	}
	if getResp.Data.ContactPhone != "13900001111" {
		t.Fatalf("GET contact_phone 不一致: %q", getResp.Data.ContactPhone)
	}
	if getResp.Data.Visibility != "hidden" {
		t.Fatalf("GET visibility 应 hidden, 实际 %q", getResp.Data.Visibility)
	}
	var exps []map[string]any
	if err := json.Unmarshal(getResp.Data.ResumeExperiences, &exps); err != nil || len(exps) != 1 {
		t.Fatalf("resume_experiences 往返失败: %s err=%v", string(getResp.Data.ResumeExperiences), err)
	}
	var certs []map[string]any
	if err := json.Unmarshal(getResp.Data.ResumeCertifications, &certs); err != nil || len(certs) != 1 {
		t.Fatalf("resume_certifications 往返失败: %s", string(getResp.Data.ResumeCertifications))
	}
	rec = do(http.MethodPut, "/api/resume/visibility", map[string]any{"visibility": "open"})
	if rec.Code != http.StatusOK {
		t.Fatalf("切换 visibility open 应 200, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	var visResp struct {
		Code int `json:"code"`
		Data struct {
			Visibility string `json:"visibility"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &visResp); err != nil {
		t.Fatalf("解析 visibility 响应失败: %v", err)
	}
	if visResp.Data.Visibility != "open" {
		t.Fatalf("visibility 切换后应 open, 实际 %q", visResp.Data.Visibility)
	}
	rec = do(http.MethodGet, "/api/resume", nil)
	if err := json.Unmarshal(rec.Body.Bytes(), &getResp); err != nil {
		t.Fatalf("GET after visibility 失败: %v", err)
	}
	if getResp.Data.Visibility != "open" {
		t.Fatalf("GET after visibility open 应 open, 实际 %q", getResp.Data.Visibility)
	}
	rec = do(http.MethodPut, "/api/resume/visibility", map[string]any{"visibility": "hidden"})
	if rec.Code != http.StatusOK {
		t.Fatalf("切换 visibility hidden 应 200, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	rec = do(http.MethodPut, "/api/resume", map[string]any{"expected_position_id": 99999})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 specialty_id 应 400, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	rec = do(http.MethodPut, "/api/resume", map[string]any{"resume_certifications": []map[string]any{{"credential_id": 99999, "cert_no": "X"}}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非法 credential_id 应 400, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	rec = do(http.MethodPut, "/api/resume", map[string]any{"resume_file_url": "https://example.com/resume.jpg"})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("非 PDF 的 resume_file_url 应 400, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	rec = do(http.MethodPut, "/api/resume", map[string]any{"resume_file_url": "https://example.com/resume.pdf"})
	if rec.Code != http.StatusOK {
		t.Fatalf("PDF resume_file_url 应 200, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	{
		body := &bytes.Buffer{}
		w := multipart.NewWriter(body)
		fw, _ := w.CreateFormFile("file", "resume.jpg")
		_, _ = fw.Write([]byte("fake image"))
		w.Close()
		req, _ := http.NewRequest(http.MethodPost, "/api/resume/pdf", body)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("上传非 PDF 应 400, 实际 %d body=%s", rr.Code, rr.Body.String())
		}
	}
	{
		body := &bytes.Buffer{}
		w := multipart.NewWriter(body)
		fw, _ := w.CreateFormFile("file", "resume.pdf")
		_, _ = fw.Write([]byte("%PDF-1.4 fake pdf"))
		w.Close()
		req, _ := http.NewRequest(http.MethodPost, "/api/resume/pdf", body)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("上传 PDF 应 200, 实际 %d body=%s", rr.Code, rr.Body.String())
		}
		var resp struct {
			Code int `json:"code"`
			Data struct {
				URL string `json:"url"`
			} `json:"data"`
		}
		if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
			t.Fatalf("解析 PDF 上传响应失败: %v", err)
		}
		if resp.Data.URL == "" {
			t.Fatalf("PDF 上传应返回 url")
		}
	}
	{
		body := &bytes.Buffer{}
		w := multipart.NewWriter(body)
		fw, _ := w.CreateFormFile("file", "cert.pdf")
		_, _ = fw.Write([]byte("%PDF fake"))
		w.Close()
		req, _ := http.NewRequest(http.MethodPost, "/api/resume/image", body)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusBadRequest {
			t.Fatalf("image 端点上传 pdf 应 400, 实际 %d body=%s", rr.Code, rr.Body.String())
		}
	}
	{
		body := &bytes.Buffer{}
		w := multipart.NewWriter(body)
		fw, _ := w.CreateFormFile("file", "photo.jpg")
		_, _ = fw.Write([]byte("fake jpg"))
		w.Close()
		req, _ := http.NewRequest(http.MethodPost, "/api/resume/image", body)
		req.Header.Set("Content-Type", w.FormDataContentType())
		req.Header.Set("Authorization", "Bearer "+token)
		rr := httptest.NewRecorder()
		r.ServeHTTP(rr, req)
		if rr.Code != http.StatusOK {
			t.Fatalf("上传图片应 200, 实际 %d body=%s", rr.Code, rr.Body.String())
		}
	}
	rec = do(http.MethodPut, "/api/resume", map[string]any{"photos": []string{"a.jpg", "b.jpg", "c.jpg", "d.jpg", "e.jpg", "f.jpg", "g.jpg"}})
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("photos 超 6 张应 400, 实际 %d body=%s", rec.Code, rec.Body.String())
	}
	if err := deps.AuthSvc.DeleteAccount(int(author.ID)); err != nil {
		t.Fatalf("删除账号失败: %v", err)
	}
	var cnt int64
	if err := db.Model(&model.JobCard{}).Where("user_id = ?", author.ID).Count(&cnt).Error; err != nil {
		t.Fatalf("查询 job_cards 失败: %v", err)
	}
	if cnt != 0 {
		t.Fatalf("级联删除失败：job_cards 仍有 %d 行，期望 0", cnt)
	}
	fmt.Println("简历卡契约通过：读写往返、字典校验、PDF 白名单、visibility 默认与切换、级联删除")
}
