package contribution

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"forklift-training/internal/clock"
	"forklift-training/internal/filestore"
	"forklift-training/internal/model"
	"forklift-training/internal/notification"
	"forklift-training/internal/points"
	"forklift-training/internal/storage"
	"forklift-training/internal/testutil"
)

// memContributionStorage 投稿测试用内存存储。
type memContributionStorage struct {
	deleted []string
	files   []string
}

func (m *memContributionStorage) Save(_ context.Context, key string, _ []byte, _ string) (string, error) {
	// 照实回显传入的 key（#1361 之后投稿暂存 key 形如 contributions/<uid>/<name>），
	// 不再硬返回一条扁平老路径——否则「上传落哪个分区」这条判据在替身里就被抹平了。
	return "/static/uploads/" + key, nil
}

func (m *memContributionStorage) Delete(_ context.Context, url string) error {
	m.deleted = append(m.deleted, url)
	return nil
}

func (m *memContributionStorage) Exists(context.Context, string) (bool, error) { return true, nil }

// contributionTestStorage 记录 Exists 调用，并可在每次校验时观察真实事务状态。
type contributionTestStorage struct {
	memContributionStorage
	existsCalls int
	existsHook  func()
}

func (m *contributionTestStorage) Exists(ctx context.Context, url string) (bool, error) {
	m.existsCalls++
	if m.existsHook != nil {
		m.existsHook()
	}
	return m.memContributionStorage.Exists(ctx, url)
}

// contributionTxTrackingPool 跟踪真实 BeginTx/Commit/Rollback；Create callback 在显式事务里可能尚未触发，不能用于证明 Exists 在事务外。
type contributionTxTrackingPool struct {
	gorm.ConnPool
	inTx bool
}

func (p *contributionTxTrackingPool) BeginTx(ctx context.Context, opts *sql.TxOptions) (gorm.ConnPool, error) {
	tx, err := p.ConnPool.(gorm.TxBeginner).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	p.inTx = true
	return &contributionTrackedTx{ConnPool: tx, pool: p}, nil
}

func (p *contributionTxTrackingPool) GetDBConn() (*sql.DB, error) {
	return p.ConnPool.(*sql.DB), nil
}

type contributionTrackedTx struct {
	gorm.ConnPool
	pool *contributionTxTrackingPool
}

func (tx *contributionTrackedTx) Commit() error {
	err := tx.ConnPool.(gorm.TxCommitter).Commit()
	tx.pool.inTx = false
	return err
}

func (tx *contributionTrackedTx) Rollback() error {
	err := tx.ConnPool.(gorm.TxCommitter).Rollback()
	tx.pool.inTx = false
	return err
}

func trackContributionTransactions(t *testing.T, db *gorm.DB) *contributionTxTrackingPool {
	t.Helper()
	sqlDB, err := db.DB()
	if err != nil {
		t.Fatalf("取 sql.DB 失败: %v", err)
	}
	pool := &contributionTxTrackingPool{ConnPool: sqlDB}
	db.ConnPool = pool
	db.Statement.ConnPool = pool
	return pool
}

// contributionSequenceClock 按调用次序返回不同时间，用来钉住事务内配额检查重新取时。
type contributionSequenceClock struct {
	times []time.Time
	calls int
}

func (c *contributionSequenceClock) Now() time.Time {
	c.calls++
	if c.calls > len(c.times) {
		return time.Time{}
	}
	return c.times[c.calls-1]
}

func (m *memContributionStorage) List(_ context.Context, _ string) ([]string, error) {
	return m.files, nil
}

// ListWithInfo 模拟存储侧元数据：LastModified 由测试文件 URL 的命名契约推导（见 fileStampToTime）。
func (m *memContributionStorage) ListWithInfo(_ context.Context, _ string) ([]storage.FileInfo, error) {
	infos := make([]storage.FileInfo, 0, len(m.files))
	for _, u := range m.files {
		infos = append(infos, storage.FileInfo{URL: u, LastModified: fileStampToTime(u)})
	}
	return infos, nil
}

func (m *memContributionStorage) Get(_ context.Context, url string) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader(url)), nil
}

// newContributionTestSvc 构造投稿服务（含真 points.Service 与通知）。
func newContributionTestSvc(t *testing.T) (*Service, *gorm.DB) {
	t.Helper()
	db := testutil.NewFileDB(t)
	fileSvc := filestore.NewFileStore("", &memContributionStorage{}, zap.NewNop())
	notif := notification.NewService(db, zap.NewNop())
	pointsSvc := points.NewService(db, zap.NewNop(), nil, notif)
	svc := NewService(db, fileSvc, notif, pointsSvc, zap.NewNop(), clock.Real())
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

// stagedFor 学员 uid 暂存位下的一个文件 URL（#1361：暂存按用户分区 contributions/<uid>/，
// Create 的归属校验读的就是这一段；扁平的 contributions/x.pdf 已是老路径，不再被接受）。
func stagedFor(uid int, name string) string {
	return fmt.Sprintf("/static/uploads/contributions/%d/%s", uid, name)
}

// contributionInput 构造创建入参（缺省文件落在**该学员自己的**暂存位）。
func contributionInput(userID, credID int, files ...ContributionFileDTO) CreateContributionInput {
	in := CreateContributionInput{UserID: userID, CredentialID: credID, Title: "叉车液压故障排查手册", Intro: "整理自一线维修笔记", Files: files}
	if len(files) == 0 {
		in.Files = []ContributionFileDTO{oneFile(stagedFor(userID, "a.pdf"), 1024)}
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
		in := contributionInput(u.ID, cred.ID, oneFile(stagedFor(u.ID, "f"+string(rune(49+i))+".pdf"), 1024))
		if _, err := svc.Create(in); err != nil {
			t.Fatalf("第 %d 次投稿应成功: %v", i+1, err)
		}
	}
	_, err := svc.Create(contributionInput(u.ID, cred.ID, oneFile(stagedFor(u.ID, "f4.pdf"), 1024)))
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
	_, err := svc.Create(contributionInput(u.ID, cred.ID, oneFile(stagedFor(u.ID, "z.pdf"), 1024)))
	if !errors.Is(err, ErrContributionQuotaPending) {
		t.Fatalf("pending=5 应拒投, got %v", err)
	}
}

func TestContribution_QuotaErrorPrecedence(t *testing.T) {
	cases := []struct {
		name    string
		daily   int
		pending int
		want    error
	}{
		{name: "日配额先于 pending", daily: ContributionDailyMax, pending: 2, want: ErrContributionQuotaDaily},
		{name: "pending 配额先于存储校验", daily: 0, pending: ContributionPendingMax, want: ErrContributionQuotaPending},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			svc, db := newContributionTestSvc(t)
			cred := seedCredential(t, db)
			u := seedContributionUser(t, db, "quota_priority", cred.ID)
			now := clock.Now()
			for i := 0; i < tc.daily; i++ {
				createdAt := now.Add(time.Duration(i) * time.Minute)
				seedContributionRow(t, db, u.ID, cred.ID, ContributionStatusPending, createdAt)
			}
			for i := 0; i < tc.pending; i++ {
				createdAt := now.AddDate(0, 0, -(i + 1))
				seedContributionRow(t, db, u.ID, cred.ID, ContributionStatusPending, createdAt)
			}
			st := &contributionTestStorage{}
			svc.fileSvc = filestore.NewFileStore("", st, zap.NewNop())

			_, err := svc.Create(contributionInput(u.ID, cred.ID))
			if !errors.Is(err, tc.want) {
				t.Fatalf("配额错误优先级应为 %v，实得 %v", tc.want, err)
			}
			if st.existsCalls != 0 {
				t.Fatalf("配额拒绝不得进入存储校验，Exists 调用了 %d 次", st.existsCalls)
			}
		})
	}
}

func TestContributionCreateValidatesStorageOutsideTransaction(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "storage_outside_tx", cred.ID)
	pool := trackContributionTransactions(t, db)
	if err := db.Transaction(func(*gorm.DB) error {
		if !pool.inTx {
			return errors.New("事务观察器未看到 BeginTx")
		}
		return nil
	}); err != nil {
		t.Fatalf("事务观察器自检失败: %v", err)
	}
	if pool.inTx {
		t.Fatal("事务提交后观察器应退出事务态")
	}

	st := &contributionTestStorage{}
	observedInTx := make([]bool, 0, 2)
	st.existsHook = func() { observedInTx = append(observedInTx, pool.inTx) }
	svc.fileSvc = filestore.NewFileStore("", st, zap.NewNop())
	files := []ContributionFileDTO{
		oneFile(stagedFor(u.ID, "outside-a.pdf"), 1024),
		oneFile(stagedFor(u.ID, "outside-b.pdf"), 2048),
	}
	if _, err := svc.Create(contributionInput(u.ID, cred.ID, files...)); err != nil {
		t.Fatalf("创建投稿失败: %v", err)
	}
	if st.existsCalls != len(files) || len(observedInTx) != len(files) {
		t.Fatalf("每个文件应校验一次，Exists=%d observations=%d", st.existsCalls, len(observedInTx))
	}
	for i, inTx := range observedInTx {
		if inTx {
			t.Fatalf("第 %d 个文件的 Exists 落在事务内", i+1)
		}
	}
}

func TestContributionCreateReadsClockSeparatelyInsideTransaction(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "fresh_tx_clock", cred.ID)
	first := time.Date(2026, 11, 18, 23, 59, 59, 0, clock.Location())
	createdAt := first.Add(500 * time.Millisecond)
	insideTx := first.Add(2 * time.Second)
	clk := &contributionSequenceClock{times: []time.Time{first, createdAt, insideTx}}
	svc.clk = clk
	quotaClockCalls := make([]int, 0, 2)
	if err := db.Callback().Query().After("gorm:query").Register("test:observe_daily_quota_clock", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "created_at >=") {
			quotaClockCalls = append(quotaClockCalls, clk.calls)
		}
	}); err != nil {
		t.Fatalf("注册配额查询观察回调失败: %v", err)
	}

	created, err := svc.Create(contributionInput(u.ID, cred.ID))
	if err != nil {
		t.Fatalf("创建投稿失败: %v", err)
	}
	if clk.calls != 3 {
		t.Fatalf("事务外配额、写入时间、事务内配额应各取一次时钟，实得 %d 次", clk.calls)
	}
	if len(quotaClockCalls) != 2 || quotaClockCalls[0] != 1 || quotaClockCalls[1] != 3 {
		t.Fatalf("事务内日配额应使用重新读取的时钟，调用序=%v", quotaClockCalls)
	}
	var row model.UserContribution
	if err := db.First(&row, created.ID).Error; err != nil {
		t.Fatalf("查创建主行失败: %v", err)
	}
	if !row.CreatedAt.Equal(createdAt) {
		t.Fatalf("主行 CreatedAt 应固定为写入取时 %s，实得 %s", createdAt, row.CreatedAt)
	}
}

func TestContributionCreateCategoryOmitsCredential(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	category := testutil.SeedTrainingCategory(t, db, "投稿分类")
	u := seedContributionUser(t, db, "category_contribution", 0)
	mainCreates := 0
	omittedCredential := false
	if err := db.Callback().Create().Before("gorm:create").Register("test:observe_category_credential_omit", func(tx *gorm.DB) {
		if tx.Statement.Table != "user_contribution" {
			return
		}
		mainCreates++
		for _, column := range tx.Statement.Omits {
			if column == "credential_id" {
				omittedCredential = true
			}
		}
	}); err != nil {
		t.Fatalf("注册投稿主行回调失败: %v", err)
	}

	in := contributionInput(u.ID, 0)
	in.CategoryID = &category.ID
	created, err := svc.Create(in)
	if err != nil {
		t.Fatalf("分类投稿创建失败: %v", err)
	}
	if mainCreates != 1 || !omittedCredential {
		t.Fatalf("分类投稿主行应 Omit credential_id，creates=%d omitted=%v", mainCreates, omittedCredential)
	}
	var credentialID sql.NullInt64
	if err := db.Raw("SELECT credential_id FROM user_contribution WHERE id = ?", created.ID).Scan(&credentialID).Error; err != nil {
		t.Fatalf("查分类投稿 credential_id 失败: %v", err)
	}
	if credentialID.Valid {
		t.Fatalf("分类投稿 credential_id 应为 NULL，实得 %d", credentialID.Int64)
	}
}

func TestContributionCreateBatchFilesPreservesFields(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "batch_files", cred.ID)
	now := time.Date(2026, 11, 18, 10, 20, 30, 0, clock.Location())
	svc.clk = clock.At(now)
	files := []ContributionFileDTO{
		{FileName: "液压手册.pdf", FileURL: stagedFor(u.ID, "hydraulic.pdf"), FileSize: 1024, ContentType: "document"},
		{FileName: "安全演示.pptx", FileURL: stagedFor(u.ID, "safety.pptx"), FileSize: 4096, ContentType: "ppt"},
		{FileName: "实操视频.mp4", FileURL: stagedFor(u.ID, "practice.mp4"), FileSize: 8192, ContentType: "video"},
	}
	fileCreates := 0
	batchSizes := make([]int, 0, 1)
	if err := db.Callback().Create().Before("gorm:create").Register("test:observe_contribution_file_batch", func(tx *gorm.DB) {
		if tx.Statement.Table != "user_contribution_file" {
			return
		}
		fileCreates++
		batchSizes = append(batchSizes, tx.Statement.ReflectValue.Len())
	}); err != nil {
		t.Fatalf("注册投稿文件回调失败: %v", err)
	}

	created, err := svc.Create(contributionInput(u.ID, cred.ID, files...))
	if err != nil {
		t.Fatalf("批量创建投稿文件失败: %v", err)
	}
	if fileCreates != 1 || len(batchSizes) != 1 || batchSizes[0] != len(files) {
		t.Fatalf("文件行应一次 Create 批量写入，calls=%d sizes=%v", fileCreates, batchSizes)
	}
	if len(created.Files) != len(files) {
		t.Fatalf("详情文件数=%d，want %d", len(created.Files), len(files))
	}
	var rows []model.UserContributionFile
	if err := db.Where("contribution_id = ?", created.ID).Order("id ASC").Find(&rows).Error; err != nil {
		t.Fatalf("查投稿文件行失败: %v", err)
	}
	if len(rows) != len(files) {
		t.Fatalf("库内文件行数=%d，want %d", len(rows), len(files))
	}
	for i, want := range files {
		row := rows[i]
		if row.ID == 0 || row.ContributionID != created.ID || row.FileURL != want.FileURL ||
			row.FileName != want.FileName || row.FileSize != want.FileSize || row.ContentType != want.ContentType ||
			!row.CreatedAt.Equal(now) {
			t.Fatalf("第 %d 个文件字段不完整：%+v，want %+v contribution_id=%d created_at=%s", i+1, row, want, created.ID, now)
		}
		got := created.Files[i]
		if got.FileID != row.ID || got.FileName != want.FileName || got.FileURL != want.FileURL ||
			got.FileSize != want.FileSize || got.ContentType != want.ContentType {
			t.Fatalf("第 %d 个文件详情字段不完整：%+v，want %+v", i+1, got, want)
		}
	}
}

func TestContributionCreateBatchFailureRollsBack(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	u := seedContributionUser(t, db, "batch_rollback", cred.ID)
	cause := errors.New("contribution file batch blocked")
	fileCreates := 0
	if err := db.Callback().Create().Before("gorm:create").Register("test:fail_contribution_file_batch", func(tx *gorm.DB) {
		if tx.Statement.Table != "user_contribution_file" {
			return
		}
		fileCreates++
		_ = tx.AddError(cause)
	}); err != nil {
		t.Fatalf("注册投稿文件失败回调失败: %v", err)
	}
	files := []ContributionFileDTO{
		oneFile(stagedFor(u.ID, "rollback-a.pdf"), 1024),
		oneFile(stagedFor(u.ID, "rollback-b.pdf"), 2048),
	}

	created, err := svc.Create(contributionInput(u.ID, cred.ID, files...))
	if created != nil || !errors.Is(err, cause) {
		t.Fatalf("批量失败应整体返回错误，created=%+v err=%v", created, err)
	}
	if fileCreates != 1 {
		t.Fatalf("批量文件写入应只尝试一次，实得 %d 次", fileCreates)
	}
	var contributions, contributionFiles int64
	if err := db.Model(&model.UserContribution{}).Where("user_id = ?", u.ID).Count(&contributions).Error; err != nil {
		t.Fatalf("查投稿主行数失败: %v", err)
	}
	if err := db.Model(&model.UserContributionFile{}).Count(&contributionFiles).Error; err != nil {
		t.Fatalf("查投稿文件行数失败: %v", err)
	}
	if contributions != 0 || contributionFiles != 0 {
		t.Fatalf("批量失败后不得留下主行或文件行，contributions=%d files=%d", contributions, contributionFiles)
	}
}

func TestCreateContributionFilesEmptySliceIsNoop(t *testing.T) {
	db := testutil.NewMemoryDB(t)
	fileCreates := 0
	if err := db.Callback().Create().Before("gorm:create").Register("test:observe_empty_contribution_files", func(tx *gorm.DB) {
		if tx.Statement.Table == "user_contribution_file" {
			fileCreates++
		}
	}); err != nil {
		t.Fatalf("注册投稿文件回调失败: %v", err)
	}
	if err := createContributionFiles(db, 1, nil, time.Now()); err != nil {
		t.Fatalf("空文件切片应无操作: %v", err)
	}
	if fileCreates != 0 {
		t.Fatalf("空文件切片不得触发 Create，实得 %d 次", fileCreates)
	}
}

func seedContributionRow(t *testing.T, db *gorm.DB, userID, credentialID int, status string, createdAt time.Time) {
	t.Helper()
	row := model.UserContribution{
		UserID: userID, CredentialID: credentialID, Title: "配额种子", Intro: "配额种子",
		Status: status, CreatedAt: createdAt, UpdatedAt: createdAt,
	}
	if err := db.Create(&row).Error; err != nil {
		t.Fatalf("seed 投稿失败: %v", err)
	}
}

// TestContribution_Lifecycle 过审→下载→达阶→下架追回全生命周期。
func TestContribution_Lifecycle(t *testing.T) {
	svc, db := newContributionTestSvc(t)
	cred := seedCredential(t, db)
	author := seedContributionUser(t, db, "author", cred.ID)

	// 1. 创建（pending）
	in := contributionInput(author.ID, cred.ID, oneFile(stagedFor(author.ID, "m.pdf"), 2048))
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
	anonIn := contributionInput(author.ID, cred.ID, oneFile(stagedFor(author.ID, "an.pdf"), 1024))
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
	assertLedgerDelta(t, db, author.ID, points.ReasonRollback, RefTypeContribution, itoa(int(created.ID)), -80)
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
			stagedFor(1, "orphan_1699999999999.pdf"), // 24h 前（旧）未引用
			stagedFor(1, "used_1700000000000.pdf"),   // 已被投稿引用
			stagedFor(1, "fresh_1900000000000.pdf"),  // 新传未引用（未到 TTL）
		},
	}
	fileSvc := filestore.NewFileStore("", st, zap.NewNop())
	notif := notification.NewService(db, zap.NewNop())
	pointsSvc := points.NewService(db, zap.NewNop(), nil, notif)
	svc := NewService(db, fileSvc, notif, pointsSvc, zap.NewNop(), clock.Real())
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
		FileURL: stagedFor(u.ID, "used_1700000000000.pdf"), FileName: "used.pdf",
		FileSize: 10, ContentType: "document", CreatedAt: now}).Error; err != nil {
		t.Fatalf("seed 文件失败: %v", err)
	}
	cleaned := svc.CleanupOrphanFiles(context.Background())
	if cleaned != 1 {
		t.Fatalf("应清 1 个孤儿, got %d (deleted=%v)", cleaned, st.deleted)
	}
	if len(st.deleted) != 1 || st.deleted[0] != stagedFor(1, "orphan_1699999999999.pdf") {
		t.Fatalf("应删旧孤儿, deleted=%v", st.deleted)
	}
}

// ---- 域包自带的测试脚手架副本（域包不得 import internal/core 的测试文件；原定义留在 internal/core）----

func itoa(n int) string { return strconv.Itoa(n) }

// fileStampToTime 测试 helper：从测试文件 URL 的命名契约（<name>_<ms>.<ext>）解析毫秒时间戳
// 作为 LastModified；无法解析（无内嵌时间戳）返回零值（sweep 对零值保守保留）。
// 仅测试用——生产代码时间戳来自 storage 层真实元数据，不反向解析文件名（ADR-0027 C2）。
func fileStampToTime(url string) time.Time {
	base := url
	if i := strings.LastIndex(base, "/"); i >= 0 {
		base = base[i+1:]
	}
	dot := strings.LastIndex(base, ".")
	if dot < 0 {
		return time.Time{}
	}
	underscore := strings.LastIndex(base[:dot], "_")
	if underscore < 0 {
		return time.Time{}
	}
	ms, err := strconv.ParseInt(base[underscore+1:dot], 10, 64)
	if err != nil || ms <= 0 {
		return time.Time{}
	}
	return time.UnixMilli(ms)
}
