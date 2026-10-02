// 投稿暂存文件四校验的 HTTP 面（#1361 / ADR-0066 决策 5，真实缺陷 #13）。
//
// 服务层锁（internal/contribution/staging_partition_test.go）判的是「哪枚哨兵」；
// 本文件判的是「哨兵有没有落成明确的 4xx、那句话有没有原样出门」。两者缺一都不算做完：
// 只测服务层，域表把越权配成 400、把不存在配成 500 都测不出来；只测 HTTP 面，
// 「同一档里挤了两条不同事实」这种压不成一句的漂移无人认领。
//
// 档位台账（contribution.ErrStatus）本身另由 errstatus_test.go 的快照钉住；本文件是它的行为面。
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"forklift-training/internal/config"
	"forklift-training/internal/model"
	"forklift-training/internal/storage"
)

// stagingCfg 与 newContributionRouter 里同一份 JWT 配置（token 签发方与校验方必须同密钥）。
func stagingCfg() *config.Config {
	return &config.Config{JWTSecretKey: "contract-secret", AuthCookie: config.AuthCookieConfig{Name: "hrwai_token"}}
}

// stageLocalFile 往学员 uid 的本地暂存位里种一个真文件，返回它的 URL（contributions/<uid>/<name>）。
// 走 storage.LocalStorage.Save 而不是 os.WriteFile 或手拼字符串：URL 形态由存储实现自己铸造，
// 测试手拼的那条与真链路差一个字符都不会被发现。
func stageLocalFile(t *testing.T, st *storage.LocalStorage, userID int, name string) string {
	t.Helper()
	url, err := st.Save(context.Background(), fmt.Sprintf("contributions/%d/%s", userID, name),
		[]byte("%PDF-1.4 契约测试夹具"), "application/pdf")
	if err != nil {
		t.Fatalf("种暂存文件失败: %v", err)
	}
	return url
}

// stagingCreateBody 一条投稿表单：files 里放给定 URL。
func stagingCreateBody(credID int, title, fileURL, fileName string) map[string]any {
	return map[string]any{
		"credential_id": credID,
		"title":         title,
		"intro":         "整理自一线维修笔记",
		"files": []map[string]any{{
			"file_url": fileURL, "file_name": fileName, "file_size": 1024, "content_type": "document",
		}},
	}
}

// stagingEnvelope 解出信封的 code / message。
func stagingEnvelope(t *testing.T, body string) (int, string) {
	t.Helper()
	var env struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
	}
	if err := json.Unmarshal([]byte(body), &env); err != nil {
		t.Fatalf("解析响应信封失败: %v body=%s", err, body)
	}
	return env.Code, env.Message
}

// TestCreateStagedFileChecksReturnDistinctFourXXs 判据 1：四校验各回明确的 4xx，且各给各的句子。
//
// 「已被登记」与「类型不在白名单」共用 400 这一档是刻意的（本仓未启用 409，见 contribution.go
// 的落档注释），但两句话必须不同——客户端要能告诉学员「换个文件」还是「换个类型」，
// 这正是「各自返回明确不同的 4xx」在本仓语义下的读法：档位不同 + 句子不同，两条都判。
func TestCreateStagedFileChecksReturnDistinctFourXXs(t *testing.T) {
	t.Parallel()
	r, deps, stu, cred, st := newContributionRouter(t)
	tok := issueContributionToken(t, stagingCfg(), stu)

	// 先立正样本：本人分区里真实存在的 pdf ⇒ 200（四条判据若有一条恒红，这一格会先炸）。
	okURL := stageLocalFile(t, st, stu.ID, "manual.pdf")
	if w := contributionDo(t, r, tok, "POST", "/api/contributions", stagingCreateBody(cred.ID, "正样本", okURL, "manual.pdf")); w.Code != http.StatusOK {
		t.Fatalf("本人暂存位的真实文件应 200，实得 %d body=%s", w.Code, w.Body.String())
	}

	cases := []struct {
		name     string
		fact     string // 四校验里的第几条（①的两个形态是同一条事实的两种形状）
		fileURL  string
		fileName string
		want     int
		wantMsg  string
	}{
		{
			name: "① 前缀是别人的暂存位", fact: "①",
			fileURL: fmt.Sprintf("/static/uploads/contributions/%d/his.pdf", stu.ID+4242),
			want:    http.StatusForbidden, wantMsg: "不属于当前用户",
		},
		{
			name: "① 前缀是改动前的扁平老路径", fact: "①",
			fileURL: "/static/uploads/contributions/legacy.pdf",
			want:    http.StatusForbidden, wantMsg: "不属于当前用户",
		},
		{
			name: "② 类型不在白名单（绕过上传端点直接造 URL）", fact: "②",
			fileURL: stageLocalFile(t, st, stu.ID, "payload.svg"),
			want:    http.StatusBadRequest, wantMsg: "不在投稿白名单",
		},
		{
			name: "③ 同一 URL 已被登记过", fact: "③",
			fileURL: okURL,
			want:    http.StatusBadRequest, wantMsg: "已被其它投稿登记",
		},
		{
			name: "④ 文件在存储侧不存在", fact: "④",
			fileURL: fmt.Sprintf("/static/uploads/contributions/%d/gone.pdf", stu.ID),
			want:    http.StatusNotFound, wantMsg: "暂存文件不存在",
		},
	}

	// 三条锁：
	//  1. 每条事实的句子稳定（①的两种形状必须同句同档）；
	//  2. 四条判据发出四条**不同**的句子（同档的 ②③ 靠句子区分——客户端要能告诉学员
	//     「换个类型」还是「换个文件」）；
	//  3. 403 与 404 各只由一条事实占用（「明确不同的 4xx」的字面判据）。
	//
	// ②③ 共用 400 是刻意的：本仓未启用 409，renderStatus 的单一咽喉里没有那一档，
	// 域表放 409 会被静默渲染成 500（见 contribution.go 的落档注释与 faq.go 的同源先例）。
	factOf := map[string]string{}         // wantMsg → 第几条事实
	statusOf := map[string]int{}          // wantMsg → 档位
	byStatus := map[int]map[string]bool{} // 档位 → 该档下的句子集合
	for _, tc := range cases {
		w := contributionDo(t, r, tok, "POST", "/api/contributions", stagingCreateBody(cred.ID, tc.name, tc.fileURL, tc.name+".pdf"))
		if w.Code != tc.want {
			t.Fatalf("%s：应回 %d，实得 %d body=%s", tc.name, tc.want, w.Code, w.Body.String())
		}
		code, msg := stagingEnvelope(t, w.Body.String())
		if code != tc.want {
			t.Fatalf("%s：信封 code 应与 HTTP 档位一致，实得 %d msg=%q", tc.name, code, msg)
		}
		if !strings.Contains(msg, tc.wantMsg) {
			t.Fatalf("%s：正文必须给出那条事实（含 %q），实得 %q", tc.name, tc.wantMsg, msg)
		}
		if prev, dup := statusOf[tc.wantMsg]; dup && prev != tc.want {
			t.Fatalf("同一条事实 %q 在不同形态下落了不同档位（%d / %d）", tc.wantMsg, prev, tc.want)
		}
		if prev, dup := factOf[tc.wantMsg]; dup && prev != tc.fact {
			t.Fatalf("两条判据发出了同一句话 %q（%s / %s）", tc.wantMsg, prev, tc.fact)
		}
		factOf[tc.wantMsg] = tc.fact
		statusOf[tc.wantMsg] = tc.want
		if byStatus[tc.want] == nil {
			byStatus[tc.want] = map[string]bool{}
		}
		byStatus[tc.want][tc.wantMsg] = true
	}
	if len(factOf) != 4 {
		t.Fatalf("四校验应发出四条不同的句子，实得 %d 条", len(factOf))
	}
	for _, want := range []int{http.StatusForbidden, http.StatusNotFound} {
		if got := len(byStatus[want]); got != 1 {
			t.Fatalf("档位 %d 应恰由一条事实占用，实得 %d 条", want, got)
		}
	}
	if got := len(byStatus[http.StatusBadRequest]); got != 2 {
		t.Fatalf("400 档应由「类型不在白名单」与「已被登记」两条句子共用，实得 %d 条", got)
	}
	if _, ok := byStatus[http.StatusConflict]; ok {
		t.Fatal("本仓未启用 409（renderStatus 里没有那一档，放 409 会被静默渲染成 500）")
	}

	// 被拒的五次创建不得留下任何行（「先落库再报错」的形状在这里判红）。
	var contr, files int64
	if err := deps.DB.Model(&model.UserContribution{}).Where("user_id = ?", stu.ID).Count(&contr).Error; err != nil {
		t.Fatalf("数投稿失败: %v", err)
	}
	if err := deps.DB.Model(&model.UserContributionFile{}).Count(&files).Error; err != nil {
		t.Fatalf("数投稿文件失败: %v", err)
	}
	if contr != 1 || files != 1 {
		t.Fatalf("只应留下正样本那一笔（投稿 1 / 文件 1），实得 投稿 %d / 文件 %d", contr, files)
	}
}

// TestUploadFileLandsInOwnPartition 判据 2 的 HTTP 面：上传落 contributions/<当前用户id>/。
// 服务层已证明 key 的形状；这里证明 handler 真的把**当前 token 里的用户**取出来传了进去
// （UploadFile 的 userID 参数一旦接错来源，只有走真端点才看得见）。
func TestUploadFileLandsInOwnPartition(t *testing.T) {
	t.Parallel()
	r, _, stu, cred, st := newContributionRouter(t)
	tok := issueContributionToken(t, stagingCfg(), stu)

	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("file", "维修手册.pdf")
	if err != nil {
		t.Fatalf("构造 multipart 失败: %v", err)
	}
	if _, err := io.WriteString(part, "%PDF-1.4 上传夹具"); err != nil {
		t.Fatalf("写入夹具失败: %v", err)
	}
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/api/contributions/upload-file", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("上传应 200，实得 %d body=%s", rec.Code, rec.Body.String())
	}
	var resp struct {
		Code int `json:"code"`
		Data struct {
			FileURL  string `json:"file_url"`
			FileName string `json:"file_name"`
		} `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("解析上传响应失败: %v body=%s", err, rec.Body.String())
	}
	wantDir := fmt.Sprintf("/static/uploads/contributions/%d/", stu.ID)
	if !strings.HasPrefix(resp.Data.FileURL, wantDir) {
		t.Fatalf("上传必须落本人分区 %s，实得 %q", wantDir, resp.Data.FileURL)
	}
	// 上传即落盘：Create 的第四校验与上传用的是同一把尺子，两头必须对齐。
	if ok, err := st.Exists(context.Background(), resp.Data.FileURL); err != nil || !ok {
		t.Fatalf("返回的 URL 必须指向真实文件，exists=%v err=%v url=%s", ok, err, resp.Data.FileURL)
	}
	// 拿着它立刻能投出去（闭环：上传 → 创建），且路径里没有第二个人的 id。
	if w := contributionDo(t, r, tok, "POST", "/api/contributions",
		stagingCreateBody(cred.ID, "上传闭环投稿", resp.Data.FileURL, "维修手册.pdf")); w.Code != http.StatusOK {
		t.Fatalf("刚上传的文件应能立刻投出去，实得 %d body=%s", w.Code, w.Body.String())
	}
}

// TestUploadFileUnauthenticatedIs401 未认证时不得落任何分区（UploadFile 的 userID 只能来自会话）。
func TestUploadFileUnauthenticatedIs401(t *testing.T) {
	t.Parallel()
	r, _, _, _, st := newContributionRouter(t)
	body := &bytes.Buffer{}
	mw := multipart.NewWriter(body)
	part, err := mw.CreateFormFile("file", "a.pdf")
	if err != nil {
		t.Fatalf("构造 multipart 失败: %v", err)
	}
	if _, err := io.WriteString(part, "x"); err != nil {
		t.Fatalf("写入夹具失败: %v", err)
	}
	_ = mw.Close()

	req := httptest.NewRequest("POST", "/api/contributions/upload-file", body)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("未认证上传应 401，实得 %d body=%s", rec.Code, rec.Body.String())
	}
	// 存储侧一个文件都没落（被拒的上传不得写盘）。
	if urls, err := st.List(context.Background(), ""); err != nil || len(urls) != 0 {
		t.Fatalf("未认证的上传不得写存储，urls=%v err=%v", urls, err)
	}
}
