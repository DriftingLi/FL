// 投稿暂存按用户分区 + Create 的四校验（#1361 / ADR-0066 决策 5，真实缺陷 #13）。
//
// 缺陷原文的形状：Create 完全相信客户端提交的 file_url/file_name/file_size/content_type，
// 服务层只校验大小；且所有暂存文件落在扁平的 contributions/ 下 ⇒ 「归属」在数据里根本不存在，
// 于是「猜别人的文件名就能把别人的文件登记进自己的投稿」「绕过上传接口塞非白名单类型」
// 「登记一个不存在的 URL」三件事全都无判据可拒。
//
// 本文件逐条锁四校验各自的事实，外加「老路径不再被写入」（上传落 contributions/<uid>/）。
// HTTP 档位（403 / 400 / 400 / 404）的锁在 internal/api/contribution_staging_contract_test.go。
package service

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
	"forklift-training/internal/notification"
	"forklift-training/internal/points"
	"forklift-training/internal/testutil"
)

// stagedStorage 可控的投稿存储替身：登记在 missing 里的 URL 才算不存在；Save 记录落库 key（分区判据读它）。
type stagedStorage struct {
	memContributionStorage
	savedKeys []string
	missing   map[string]bool
}

func (m *stagedStorage) Save(_ context.Context, key string, _ []byte, _ string) (string, error) {
	m.savedKeys = append(m.savedKeys, key)
	return "/static/uploads/" + key, nil
}

func (m *stagedStorage) Exists(_ context.Context, url string) (bool, error) {
	if m.missing != nil && m.missing[url] {
		return false, nil
	}
	return true, nil
}

// newStagedSvc 构造一套「存储可控」的投稿服务（四校验要问存储侧与文件表，不能用默认替身）。
func newStagedSvc(t *testing.T) (*ContributionService, *gorm.DB, *stagedStorage) {
	t.Helper()
	db := testutil.NewFileDB(t)
	st := &stagedStorage{missing: map[string]bool{}}
	fileSvc := filestore.NewFileStore("", st, zap.NewNop())
	notif := notification.NewService(db, zap.NewNop())
	pointsSvc := points.NewService(db, zap.NewNop(), nil, notif)
	svc := NewContributionService(db, fileSvc, notif, pointsSvc, zap.NewNop(), clock.Real())
	return svc, db, st
}

// stagedURL 学员 userID 暂存位下的一个文件 URL（local 形态，与 storage.LocalStorage.Save 的返回一致）。
func stagedURL(userID int, name string) string {
	return fmt.Sprintf("/static/uploads/contributions/%d/%s", userID, name)
}

// oldFlatURL 老路径（扁平 contributions/）——本次改动后既写不进去、也不再被接受。
func oldFlatURL(name string) string {
	return "/static/uploads/contributions/" + name
}

// stagedInput 用已建好的学员拼一条完整创建入参（每条子用例各建自己的库，故 uid 从 1 起也互不复用）。
func stagedInput(u *model.HrwaiUser, credID int, files ...ContributionFileDTO) CreateContributionInput {
	return CreateContributionInput{
		UserID: u.ID, CredentialID: credID,
		Title: "叉车液压故障排查手册", Intro: "整理自一线维修笔记",
		Files: files,
	}
}

// uploadFileHeader 造一枚 multipart 文件头（真走 net/http 的解析，不手搓 FileHeader 的内部字段）。
func uploadFileHeader(t *testing.T, filename, content string) *multipart.FileHeader {
	t.Helper()
	body := &bytes.Buffer{}
	w := multipart.NewWriter(body)
	part, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("构造 multipart 失败: %v", err)
	}
	if _, err := part.Write([]byte(content)); err != nil {
		t.Fatalf("写入夹具内容失败: %v", err)
	}
	_ = w.Close()

	req := httptest.NewRequest("POST", "/api/contributions/upload-file", body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	_, fh, err := req.FormFile("file")
	if err != nil {
		t.Fatalf("解析上传文件失败: %v", err)
	}
	return fh
}

// TestUploadFileWritesIntoUserPartition 判据 2 的写入侧：暂存位按用户分区，老路径不再被写入。
func TestUploadFileWritesIntoUserPartition(t *testing.T) {
	svc, db, st := newStagedSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "stage_uploader", cred.ID)

	fileHeader := uploadFileHeader(t, "维修手册.pdf", "%PDF-1.4 夹具")
	dto, err := svc.UploadFile(context.Background(), u.ID, fileHeader)
	if err != nil {
		t.Fatalf("上传投稿暂存文件失败: %v", err)
	}
	wantDir := fmt.Sprintf("contributions/%d/", u.ID)
	if len(st.savedKeys) != 1 {
		t.Fatalf("存储侧应收到 1 个 key，实得 %d", len(st.savedKeys))
	}
	if !strings.HasPrefix(st.savedKeys[0], wantDir) {
		t.Fatalf("落库 key 必须落在本人分区 %s 下，实得 %q", wantDir, st.savedKeys[0])
	}
	if !strings.Contains(dto.FileURL, "/"+wantDir) {
		t.Fatalf("返回的 URL 必须带本人分区，实得 %q", dto.FileURL)
	}
	// 老路径（扁平 contributions/）不再出现：URL 里 contributions/ 之后的第一段必须是 uid。
	if rest := strings.TrimPrefix(dto.FileURL, "/static/uploads/contributions/"); !strings.HasPrefix(rest, fmt.Sprintf("%d/", u.ID)) {
		t.Fatalf("写回了扁平的老路径: %q", dto.FileURL)
	}
}

// TestUploadFileRejectsUnauthenticatedUser 上传必须带着学员 id 落分区（id<=0 不落任何目录、不写存储）。
func TestUploadFileRejectsUnauthenticatedUser(t *testing.T) {
	svc, _, st := newStagedSvc(t)
	if _, err := svc.UploadFile(context.Background(), 0, uploadFileHeader(t, "a.pdf", "x")); err == nil {
		t.Fatal("userID<=0 时不得落任何暂存目录")
	}
	if len(st.savedKeys) != 0 {
		t.Fatalf("被拒的上传不得写存储，实得 %v", st.savedKeys)
	}
}

// TestCreateValidatesStagedFileOwner 校验①：前缀不属本人 ⇒ 专属哨兵。
// 老路径（扁平 contributions/）与非投稿前缀都落在这一格——它们都答不出「这是谁的」。
// 每条子用例一套独立的库 + 独立的学员（account/phone 那两列也带唯一索引，不能复用名字）。
func TestCreateValidatesStagedFileOwner(t *testing.T) {
	cases := []struct {
		name string
		slug string
		url  func(uid int) string
	}{
		{"别人的分区", "other", func(uid int) string { return stagedURL(uid+999, "his.pdf") }},
		{"扁平老路径（改动前的暂存位）", "flat", func(_ int) string { return oldFlatURL("legacy.pdf") }},
		{"非投稿前缀（头像位）", "avatar", func(uid int) string { return fmt.Sprintf("/static/uploads/avatars/%d/x.pdf", uid) }},
		{"段级穿越", "traversal", func(uid int) string {
			return fmt.Sprintf("/static/uploads/contributions/%d/../../%d/x.pdf", uid, uid+1)
		}},
		{"归属段不是数字", "notuid", func(uid int) string { return fmt.Sprintf("/static/uploads/contributions/team/%d/x.pdf", uid) }},
		{"扁平老路径且非白名单类型（归属判据必须在前）", "flatboth", func(_ int) string { return oldFlatURL("legacy.svg") }},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, db, _ := newStagedSvc(t)
			cred := seedCredential(t, db)
			u := seedContributionUser(t, db, "owner_"+tc.slug, cred.ID)
			in := stagedInput(u, cred.ID, oneFile(tc.url(u.ID), 1024))
			_, err := svc.Create(in)
			if !errors.Is(err, ErrContributionStagedNotOwner) {
				t.Fatalf("%s：应回 ErrContributionStagedNotOwner，实得 %v", tc.name, err)
			}
			if errors.Is(err, ErrContributionFileExtNotAllowed) || errors.Is(err, ErrContributionFileMissing) ||
				errors.Is(err, ErrContributionFileAlreadyClaimed) {
				t.Fatalf("%s：归属判据应抢在后面三条之前，实得 %v", tc.name, err)
			}
		})
	}
}

// TestCreateAcceptsOwnStagedFileBothURLShapes 正样本：本人暂存位的 local 与 R2 两种形态都认
// （本站判定走 internal/filestore/attachment.go 的单点，两种形态同源，不能只认一种）。
func TestCreateAcceptsOwnStagedFileBothURLShapes(t *testing.T) {
	svc, db, _ := newStagedSvc(t)
	cred := seedCredential(t, db)
	local := seedContributionUser(t, db, "stage_ok_local", cred.ID)
	if _, err := svc.Create(stagedInput(local, cred.ID, oneFile(stagedURL(local.ID, "a.pdf"), 1024))); err != nil {
		t.Fatalf("local 形态的本人暂存文件应可创建: %v", err)
	}

	r2User := seedContributionUser(t, db, "stage_ok_r2", cred.ID)
	r2 := fmt.Sprintf("https://cdn.example.com/contributions/%d/b.pdf", r2User.ID)
	if _, err := svc.Create(stagedInput(r2User, cred.ID, oneFile(r2, 1024))); err != nil {
		t.Fatalf("R2 形态的本人暂存文件应可创建: %v", err)
	}
}

// TestCreateValidatesStagedFileType 校验②：类型不在白名单 ⇒ 专属哨兵（判 URL，不判客户端的 file_name）。
func TestCreateValidatesStagedFileType(t *testing.T) {
	svc, db, _ := newStagedSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "stage_ext", cred.ID)
	in := stagedInput(u, cred.ID, oneFile(stagedURL(u.ID, "payload.svg"), 1024))
	// 客户端把 file_name 洗成合法的 .pdf 也绕不开——判据读的是服务端铸造的 URL。
	in.Files[0].FileName = "维修手册.pdf"
	if _, err := svc.Create(in); !errors.Is(err, ErrContributionFileExtNotAllowed) {
		t.Fatalf("非白名单类型应回 ErrContributionFileExtNotAllowed，实得 %v", err)
	}

	// 白名单内的对照面：同一分区换个 pdf 就能建成。
	in.Files[0].FileURL = stagedURL(u.ID, "manual.pdf")
	if _, err := svc.Create(in); err != nil {
		t.Fatalf("白名单内类型应可创建: %v", err)
	}
}

// TestCreateValidatesStagedFileNotClaimed 校验③：同一 URL 被登记过 ⇒ 专属哨兵。
func TestCreateValidatesStagedFileNotClaimed(t *testing.T) {
	svc, db, _ := newStagedSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "stage_claimed", cred.ID)
	url := stagedURL(u.ID, "one.pdf")
	if _, err := svc.Create(stagedInput(u, cred.ID, oneFile(url, 1024))); err != nil {
		t.Fatalf("首次创建应成功: %v", err)
	}

	// 同一个作者换个标题复用同一份暂存文件 ⇒ 必须被拒（引用即归属，一份文件只许登记一次）。
	again := stagedInput(u, cred.ID, oneFile(url, 1024))
	again.Title = "换个标题复用同一个文件"
	if _, err := svc.Create(again); !errors.Is(err, ErrContributionFileAlreadyClaimed) {
		t.Fatalf("重复登记应回 ErrContributionFileAlreadyClaimed，实得 %v", err)
	}
	var files int64
	if err := db.Model(&model.UserContributionFile{}).Count(&files).Error; err != nil {
		t.Fatalf("计数失败: %v", err)
	}
	if files != 1 {
		t.Fatalf("文件行应仍是 1 行（被拒的创建不得落库），实得 %d", files)
	}
}

// TestCreateValidatesStagedFileExists 校验④：存储侧查不到 ⇒ 专属哨兵。
func TestCreateValidatesStagedFileExists(t *testing.T) {
	svc, db, st := newStagedSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "stage_missing", cred.ID)
	url := stagedURL(u.ID, "gone.pdf")
	st.missing[url] = true
	if _, err := svc.Create(stagedInput(u, cred.ID, oneFile(url, 1024))); !errors.Is(err, ErrContributionFileMissing) {
		t.Fatalf("文件不存在应回 ErrContributionFileMissing，实得 %v", err)
	}
}

// TestCreateFailsClosedWithoutStorageBackend 装配里没接存储时，第四校验必须报错而不是静默放行
// （放行 = 把「文件真实存在」换成「客户端说了算」，正是本票要消掉的形状）。
func TestCreateFailsClosedWithoutStorageBackend(t *testing.T) {
	db := testutil.NewFileDB(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "stage_nostore", cred.ID)
	bare := NewContributionService(db, filestore.NewFileStore("", nil, zap.NewNop()), nil, nil, zap.NewNop(), clock.Real())
	_, err := bare.Create(stagedInput(u, cred.ID, oneFile(stagedURL(u.ID, "a.pdf"), 1024)))
	if !errors.Is(err, ErrContributionStorageUnconfigured) {
		t.Fatalf("未配置存储时应回 ErrContributionStorageUnconfigured（落 500），实得 %v", err)
	}
}

// ===== 判据 2 的静态面：老路径（扁平 contributions/）不再被任何写入点使用 =====
//
// 为什么光有 TestUploadFileWritesIntoUserPartition 不够：那条用例钉住的是**现在这一个**写入点。
// 日后谁在投稿域另开一处 `Save(…, filestore.ContributionFileDirPrefix)`（漏了 uid 段），它的 URL 依然过不了
// 校验①，于是学员看到的是「上传成功但提交被拒」——上面那条用例照样绿。扁平前缀一旦还能被写入，
// 「归属由路径承载」就有了第二个不承载归属的出口，所以这条要从源码面上钉死。
//
// 手法与 credential_scope_guard_test.go 同一族：判据做成纯函数 + 正负探针，探针保证规则不是空转。

// 判据做成「同一行里同时出现 .Save( 与裸的 filestore.ContributionFileDirPrefix」——不用正则括住实参：
// Save 的实参本身带括号（[]byte("x")、contributionStagedDir(userID)），任何 [^)] 风格的正则
// 都会在第一个内层括号处断掉，那样这条扫描是假绿的（本文件末尾的探针正是这一格）。
func flatContributionSaveHits(src string) []string {
	var hits []string
	for i, line := range strings.Split(src, "\n") {
		t := strings.TrimSpace(line)
		if strings.HasPrefix(t, "//") {
			continue
		}
		if strings.Contains(t, ".Save(") && strings.Contains(t, "filestore.ContributionFileDirPrefix") {
			hits = append(hits, fmt.Sprintf("%d: %s", i+1, t))
		}
	}
	return hits
}

// TestContributionDomainHasNoFlatStagingWriter 扫描 service 包的全部非测试源文件。
func TestContributionDomainHasNoFlatStagingWriter(t *testing.T) {
	// 射程 = **生产代码全域**（原先是「本包目录」）：扁平暂存写入点若在新的域包里长回来，
	// 这条锁照样抓得住（实测宽化后全域 0 新命中）。
	scanned := 0
	var violations []string
	for _, f := range testutil.ScanBackendCode(t) {
		if !testutil.Production(f) {
			continue
		}
		scanned++
		for _, h := range flatContributionSaveHits(f.Src) {
			violations = append(violations, f.Path+":"+h)
		}
	}
	// 实测射程：internal/ + pkg/ 下的生产代码 275 个（下界留余量；真正防恒绿的是
	// testutil.ScanBackendCode 的「一个都没读到即 Fatal」）。
	if scanned < 250 {
		t.Fatalf("扫描面异常：只读到 %d 个源文件（规则可能在空转）", scanned)
	}
	// 规则活性：回归形态必须被抓住，分区的写法必须放过——两条都在本用例里当场验一遍。
	if len(flatContributionSaveHits(`url, err := s.fileSvc.Save(content, fileHeader.Filename, filestore.ContributionFileDirPrefix)`)) != 1 {
		t.Fatal("扫描规则失灵：扁平前缀的写入形态没被抓住")
	}
	// 实参里带内层括号的回归形态（[]byte("x")）——正则版正是从这一格假绿的。
	if len(flatContributionSaveHits(`_, _ = s.fileSvc.Save([]byte("x"), "n.pdf", filestore.ContributionFileDirPrefix)`)) != 1 {
		t.Fatal("扫描规则失灵：带内层括号的扁平前缀写入形态没被抓住")
	}
	if len(flatContributionSaveHits(`url, err := s.fileSvc.Save(content, fileHeader.Filename, contributionStagedDir(userID))`)) != 0 {
		t.Fatal("扫描规则误伤：按用户分区的写入形态被判违规")
	}
	// 前缀登记 / 分区目录的构造处都出现裸前缀，但都不是写入点。
	if len(flatContributionSaveHits(`return fmt.Sprintf("%s/%d", filestore.ContributionFileDirPrefix, userID)`)) != 0 {
		t.Fatal("扫描规则误伤：contributionStagedDir 的构造行被判违规")
	}
	if len(violations) > 0 {
		t.Fatalf("投稿域仍有把文件落到扁平 contributions/ 的写入点（老路径重新被写入 = #1361 判据 2 失守）：\n%s",
			strings.Join(violations, "\n"))
	}
	t.Logf("已扫 %d 个源文件，无扁平暂存写入点", scanned)
}
