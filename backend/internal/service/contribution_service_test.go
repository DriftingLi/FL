package service

import (
	"context"
	"errors"
	"io"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/model"
	"forklift-training/internal/testutil"
)

// memContributionStorage 投稿测试用内存存储。
type memContributionStorage struct {
	deleted []string
	files   []string
}

func (m *memContributionStorage) Save(_ context.Context, _ string, _ []byte, _ string) (string, error) {
	return "/static/uploads/contributions/note_1700000000000.pdf", nil
}

func (m *memContributionStorage) Delete(_ context.Context, url string) error {
	m.deleted = append(m.deleted, url)
	return nil
}

func (m *memContributionStorage) Exists(context.Context, string) (bool, error) { return true, nil }

func (m *memContributionStorage) List(_ context.Context, _ string) ([]string, error) {
	return m.files, nil
}

func (m *memContributionStorage) Get(_ context.Context, url string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(url)), nil
}

// newContributionTestSvc 构造投稿服务（含真 PointsService 与通知）。
func newContributionTestSvc(t *testing.T) (*ContributionService, *gorm.DB) {
	t.Helper()
	db := testutil.NewFileDB(t)
	fileSvc := NewFileStore("", &memContributionStorage{}, zap.NewNop())
	notif := NewNotificationService(db, zap.NewNop())
	points := NewPointsService(db, zap.NewNop(), nil)
	svc := NewContributionService(db, fileSvc, notif, points, zap.NewNop(), clock.Real())
	return svc, db
}

// seedContributionUser 创建学员并可选指定当前证件。
func seedContributionUser(t *testing.T, db *gorm.DB, name string, credID int) *model.HrwaiUser {
	t.Helper()
	u := &model.HrwaiUser{
		UID:       time.Now().UnixNano(),
		Account:   "contrib_" + name,
		Username:  name,
		Password:  "x",
		Phone:     "c_" + name,
		Status:    1,
		CreatedAt: time.Now(),
	}
	if credID > 0 {
		cid := credID
		u.CurrentCredentialID = &cid
	}
	if err := db.Create(u).Error; err != nil {
		t.Fatalf("创建学员失败: %v", err)
	}
	return u
}

// seedCredential 建测试证件。
func seedCredential(t *testing.T, db *gorm.DB) *model.Credential {
	t.Helper()
	c := &model.Credential{
		Code: "N1",
		Name: "叉车司机",
	}
	if err := db.Create(c).Error; err != nil {
		t.Fatalf("创建证件失败: %v", err)
	}
	return c
}

// oneFile 快速构造文件 DTO。
func oneFile(url string, size int64) ContributionFileDTO {
	return ContributionFileDTO{FileName: "a.pdf", FileURL: url, FileSize: size, ContentType: "document"}
}

// contributionInput 构造创建入参。
func contributionInput(userID, credID int, files ...ContributionFileDTO) CreateContributionInput {
	in := CreateContributionInput{UserID: userID, CredentialID: credID, Title: "叉车液压故障排查手册", Intro: "整理自一线维修笔记", Files: files}
	if len(files) == 0 {
		in.Files = []ContributionFileDTO{oneFile("/static/uploads/contributions/a.pdf", 1024)}
	}
	return in
}

// assertLedgerDelta 断言指定 ref 的累计入账（delta 方向区分）。
func assertLedgerDelta(t *testing.T, db *gorm.DB, userID int, reason, refType, refID string, want int) {
	t.Helper()
	var sum int64
	if err := db.Model(&model.PointsLedger{}).
		Where("user_id = ? AND reason = ? AND ref_type = ? AND ref_id = ?", userID, reason, refType, refID).
		Select("COALESCE(SUM(delta),0)").Scan(&sum).Error; err != nil {
		t.Fatalf("查流水失败: %v", err)
	}
	if int(sum) != want {
		t.Fatalf("流水 %s/%s 合计 = %d，want %d", reason, refID, sum, want)
	}
}

// TestContribution_NoCredentialBlocked 未选证件不能投稿。
func TestContribution_NoCredentialBlocked(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	u := seedContributionUser(t, db, "nobe", 0)
	_, err := svc.Create(contributionInput(u.ID, 0))
	if !errors.Is(err, ErrContributionNoCredential) {
		t.Fatalf("want ErrContributionNoCredential, got %v", err)
	}
}

// TestContribution_QuotaDaily 日配额臂。
func TestContribution_QuotaDaily(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "qday", cred.ID)
	for i := 0; i < ContributionDailyMax; i++ {
		in := contributionInput(u.ID, cred.ID, oneFile("/static/uploads/contributions/f"+string(rune(49+i))+".pdf", 1024))
		if _, err := svc.Create(in); err != nil {
			t.Fatalf("第 %d 次投稿应成功: %v", i+1, err)
		}
	}
	_, err := svc.Create(contributionInput(u.ID, cred.ID, oneFile("/static/uploads/contributions/f4.pdf", 1024)))
	if !errors.Is(err, ErrContributionQuotaDaily) {
		t.Fatalf("第 4 份应触达日配额, got %v", err)
	}
}

// TestContribution_QuotaPending 积压配额臂。
func TestContribution_QuotaPending(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "qpend", cred.ID)
	// pending 积压到 5 份：5 份 pending 分散到 5 个历史日期（避免当日 3 份配额先拦）
	for i := 0; i < 5; i++ {
		now := time.Now().AddDate(0, 0, -(i + 1))
		c := model.UserContribution{
			UserID: u.ID, CredentialID: cred.ID, Title: "t" + string(rune(65+i)), Intro: "i",
			Status: ContributionStatusPending, CreatedAt: now, UpdatedAt: now,
		}
		if err := db.Create(&c).Error; err != nil {
			t.Fatalf("seed pending 失败: %v", err)
		}
	}
	_, err := svc.Create(contributionInput(u.ID, cred.ID, oneFile("/static/uploads/contributions/z.pdf", 1024)))
	if !errors.Is(err, ErrContributionQuotaPending) {
		t.Fatalf("pending=5 应拒投, got %v", err)
	}
}

// TestContribution_Lifecycle 过审→下载→达阶→下架追回全生命周期。
func TestContribution_Lifecycle(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	author := seedContributionUser(t, db, "author", cred.ID)

	// 1. 创建（pending）
	in := contributionInput(author.ID, cred.ID, oneFile("/static/uploads/contributions/m.pdf", 2048))
	created, err := svc.Create(in)
	if err != nil {
		t.Fatalf("创建投稿失败: %v", err)
	}
	if created.Status != ContributionStatusPending {
		t.Fatalf("初始应为 pending, got %s", created.Status)
	}
	// 未过审不产生积分
	assertLedgerDelta(t, db, author.ID, ReasonContributionApproved, RefTypeContribution, itoa(int(created.ID)), 0)

	// 2. 路人看不到 pending
	bystander := seedContributionUser(t, db, "looker", cred.ID)
	if _, err := svc.GetDetail(created.ID, bystander.ID); !errors.Is(err, ErrContributionNotFound) {
		t.Fatalf("路人应看不到 pending, got %v", err)
	}

	// 3. 审核通过 → +50
	if _, err := svc.Approve(1, created.ID); err != nil {
		t.Fatalf("审核失败: %v", err)
	}
	assertLedgerDelta(t, db, author.ID, ReasonContributionApproved, RefTypeContribution, itoa(int(created.ID)), 50)
	// 重复通过：CAS 拒（已非 pending）
	if _, err := svc.Approve(1, created.ID); !errors.Is(err, ErrContributionNotPending) {
		t.Fatalf("重复 approve 应拒, got %v", err)
	}
	assertLedgerDelta(t, db, author.ID, ReasonContributionApproved, RefTypeContribution, itoa(int(created.ID)), 50)

	// 4. 路人下载（首次）：计数 +1、无达阶
	r1, err := svc.Download(bystander.ID, created.ID)
	if err != nil || !r1.IsNew || r1.TierAwarded != 0 {
		t.Fatalf("首次下载应新增无达阶: %+v err=%v", r1, err)
	}
	var dlCnt int64
	db.Model(&model.ContributionDownload{}).Where("contribution_id = ?", created.ID).Count(&dlCnt)
	if dlCnt != 1 {
		t.Fatalf("下载事实源应为 1, got %d", dlCnt)
	}

	// 5. 作者下载不计
	rAuth, err := svc.Download(author.ID, created.ID)
	if err != nil || rAuth.IsNew {
		t.Fatalf("作者下载应不计: %+v err=%v", rAuth, err)
	}
	var dlCnt2 int64
	db.Model(&model.ContributionDownload{}).Where("contribution_id = ?", created.ID).Count(&dlCnt2)
	if dlCnt2 != 1 {
		t.Fatalf("作者下载后事实源应仍为 1, got %d", dlCnt2)
	}

	// 6. 重复下载幂等
	r2, err := svc.Download(bystander.ID, created.ID)
	if err != nil || r2.IsNew {
		t.Fatalf("重复下载应幂等不新增: %+v err=%v", r2, err)
	}

	// 7. 匿名投稿作者显示
	anonIn := contributionInput(author.ID, cred.ID, oneFile("/static/uploads/contributions/an.pdf", 1024))
	anonIn.IsAnonymous = true
	anonIn.Title = "匿名资料"
	anon, err := svc.Create(anonIn)
	if err != nil {
		t.Fatalf("匿名投稿失败: %v", err)
	}
	if _, err := svc.Approve(1, anon.ID); err != nil {
		t.Fatalf("匿名过审失败: %v", err)
	}
	detail, err := svc.GetDetail(anon.ID, bystander.ID)
	if err != nil {
		t.Fatalf("匿名详情失败: %v", err)
	}
	if detail.Author.Anonymous != true || detail.Author.Username != "" {
		t.Fatalf("匿名作者不应露名: %+v", detail.Author)
	}
}

// TestContribution_TierBonus 达阶奖励（10/50/200）。
func TestContribution_TierBonus(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	author := seedContributionUser(t, db, "tierA", cred.ID)
	in := contributionInput(author.ID, cred.ID)
	created, err := svc.Create(in)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Approve(1, created.ID); err != nil {
		t.Fatalf("过审失败: %v", err)
	}
	// 造 9 个下载者（不含作者）：第 10 次下载触发 +30
	users := make([]int, 0, 10)
	for i := 0; i < 10; i++ {
		u := seedContributionUser(t, db, "d"+string(rune(48+i)), cred.ID)
		users = append(users, u.ID)
	}
	// 9 次无达阶
	for i := 0; i < 9; i++ {
		res, err := svc.Download(users[i], created.ID)
		if err != nil || res.TierAwarded != 0 {
			t.Fatalf("第 %d 次不应达阶: %+v err=%v", i+1, res, err)
		}
	}
	// 第 10 次 → +30
	res, err := svc.Download(users[9], created.ID)
	if err != nil || res.TierAwarded != 30 {
		t.Fatalf("第 10 次应达阶 +30: %+v err=%v", res, err)
	}
	assertLedgerDelta(t, db, author.ID, ReasonContributionTier, RefTypeContribution, itoa(int(created.ID)), 30)
	// 再看下架追回：累计投稿分 = 50+30 = 80
	if _, err := svc.Archive(1, created.ID, "内容违规"); err != nil {
		t.Fatalf("下架失败: %v", err)
	}
	assertLedgerDelta(t, db, author.ID, ReasonRollback, RefTypeContribution, itoa(int(created.ID)), -80)
	// 重复下架幂等（已 archived 拒）
	if _, err := svc.Archive(1, created.ID, "again"); !errors.Is(err, ErrContributionNotApproved) {
		t.Fatalf("重复下架应拒: %v", err)
	}
}

// TestContribution_Report 举报与处置。
func TestContribution_Report(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	author := seedContributionUser(t, db, "repA", cred.ID)
	in := contributionInput(author.ID, cred.ID)
	created, err := svc.Create(in)
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Approve(1, created.ID); err != nil {
		t.Fatalf("过审失败: %v", err)
	}
	reporter := seedContributionUser(t, db, "repR", cred.ID)
	if err := svc.Report(reporter.ID, created.ID, ReportReasonPiracy); err != nil {
		t.Fatalf("举报失败: %v", err)
	}
	// 重复举报合并（不新增行）
	if err := svc.Report(reporter.ID, created.ID, ReportReasonViolation); err != nil {
		t.Fatalf("重复举报应合并: %v", err)
	}
	var cnt int64
	db.Model(&model.ContributionReport{}).Where("contribution_id = ?", created.ID).Count(&cnt)
	if cnt != 1 {
		t.Fatalf("重复举报应合并为 1 行, got %d", cnt)
	}
	// 处置：下架（被举报下架）
	var rep model.ContributionReport
	db.Where("contribution_id = ?", created.ID).First(&rep)
	if err := svc.HandleReport(1, rep.ID, "archive"); err != nil {
		t.Fatalf("处置失败: %v", err)
	}
	var updated model.ContributionReport
	db.First(&updated, rep.ID)
	if updated.Status != 1 {
		t.Fatalf("举报应标记已处理, got %d", updated.Status)
	}
	// 下架后公开列表不可见
	list, err := svc.ListPublic(ListPublicInput{CredentialID: cred.ID, Page: 1, PageSize: 10})
	if err != nil {
		t.Fatalf("列表失败: %v", err)
	}
	for _, it := range list.Items {
		if it.ID == created.ID {
			t.Fatal("下架稿不应出现在公开列表")
		}
	}
}

// TestContribution_Withdraw 撤回 pending。
func TestContribution_Withdraw(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "wd", cred.ID)
	created, err := svc.Create(contributionInput(u.ID, cred.ID))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if err := svc.Withdraw(u.ID, created.ID); err != nil {
		t.Fatalf("撤回失败: %v", err)
	}
	var c model.UserContribution
	db.First(&c, created.ID)
	if c.Status != ContributionStatusWithdrawn {
		t.Fatalf("撤回后应为 withdrawn, got %s", c.Status)
	}
	// 已撤回不能再次撤回
	if err := svc.Withdraw(u.ID, created.ID); !errors.Is(err, ErrContributionNotPending) {
		t.Fatalf("重复撤回应拒, got %v", err)
	}
}

// TestContribution_Reject 驳回必填原因。
func TestContribution_Reject(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "rj", cred.ID)
	created, err := svc.Create(contributionInput(u.ID, cred.ID))
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}
	if _, err := svc.Reject(1, created.ID, ""); !errors.Is(err, ErrContributionRejectReason) {
		t.Fatalf("空原因应拒, got %v", err)
	}
	if _, err := svc.Reject(1, created.ID, "格式不对"); err != nil {
		t.Fatalf("驳回失败: %v", err)
	}
	assertLedgerDelta(t, db, u.ID, ReasonContributionApproved, RefTypeContribution, itoa(int(created.ID)), 0)
}

// TestContribution_CleanupOrphans 悬空文件回收：超 24h 未引用的暂存文件被清，已提交的不清。
func TestContribution_CleanupOrphans(t *testing.T) {
	t.Helper()
	db := testutil.NewFileDB(t)
	st := &memContributionStorage{
		files: []string{
			"/static/uploads/contributions/orphan_1699999999999.pdf", // 24h 前（旧）未引用
			"/static/uploads/contributions/used_1700000000000.pdf",   // 已被投稿引用
			"/static/uploads/contributions/fresh_1900000000000.pdf",  // 新传未引用（未到 TTL）
		},
	}
	fileSvc := NewFileStore("", st, zap.NewNop())
	notif := NewNotificationService(db, zap.NewNop())
	points := NewPointsService(db, zap.NewNop(), nil)
	svc := NewContributionService(db, fileSvc, notif, points, zap.NewNop(), clock.Real())
	// 一条已提交投稿引用 used 文件
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "cleanup", cred.ID)
	now := time.Now()
	contr := model.UserContribution{UserID: u.ID, CredentialID: cred.ID, Title: "used稿", Intro: "x",
		Status: ContributionStatusApproved, CreatedAt: now, UpdatedAt: now}
	if err := db.Create(&contr).Error; err != nil {
		t.Fatalf("seed 投稿失败: %v", err)
	}
	if err := db.Create(&model.UserContributionFile{ContributionID: contr.ID,
		FileURL: "/static/uploads/contributions/used_1700000000000.pdf", FileName: "used.pdf",
		FileSize: 10, ContentType: "document", CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed 文件失败: %v", err)
	}
	cleaned := svc.CleanupOrphanFiles(context.Background())
	if cleaned != 1 {
		t.Fatalf("应清 1 个孤儿, got %d (deleted=%v)", cleaned, st.deleted)
	}
	if len(st.deleted) != 1 || st.deleted[0] != "/static/uploads/contributions/orphan_1699999999999.pdf" {
		t.Fatalf("应删旧孤儿, deleted=%v", st.deleted)
	}
}
