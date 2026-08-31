// 迁移文件可发现性测试（#364 顺带补的守卫）。
//
// 为什么需要：CI 的 migration-check job 只比较 *.up.sql 与 *.down.sql 的**数量**是否相等，
// 从不连数据库、从不执行 SQL（虽设了 DATABASE_URL 但没有命令消费它）。所以迁移 SQL 真正
// 跑起来的地方只有部署时。本测试把"部署前就能查出的一类错误"补上：
//   - 文件名不符合 <version>_<title>.(up|down).sql（golang-migrate 直接读不到）
//   - 版本号重复（migrate 启动即失败）
//   - 缺 down 配对（要回滚时才发现没救）
//   - 文件在但内容读不出来 / 是空的
//
// ⚠️ 本测试**不**验证 SQL 语义。例如 000004 的两条 CHECK 是否合法、是否与既有数据冲突，
// 只有真实 Postgres 能回答。写下这个文件的人（我）也没能验它 —— 别把它当保险。
// 有意不校验"版本号连续"：那是给全仓未来所有迁移立规矩，且并行分支各自加迁移时必然误报。
package migrate

import (
	"io"
	"os"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/golang-migrate/migrate/v4/source"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"go.uber.org/zap"
)

// migrationFileRe 匹配 golang-migrate 要求的命名：<version>_<title>.(up|down).sql。
var migrationFileRe = regexp.MustCompile(`^(\d+)_.+\.(up|down)\.sql$`)

// parseMigrationVersions 从目录里解析出 up / down 两个版本号集合。
func parseMigrationVersions(t *testing.T, dir string) (ups, downs map[int]string) {
	t.Helper()
	ups, downs = map[int]string{}, map[int]string{}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("读 migrations 目录失败: %v", err)
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") {
			continue // README 之类的旁支文件不参与校验
		}
		m := migrationFileRe.FindStringSubmatch(name)
		if m == nil {
			t.Fatalf("迁移文件命名不合规，golang-migrate 读不到: %s", name)
		}
		version, err := strconv.Atoi(m[1])
		if err != nil {
			t.Fatalf("版本号非数字: %s", name)
		}
		bucket := ups
		if m[2] == "down" {
			bucket = downs
		}
		if dup, ok := bucket[version]; ok {
			t.Fatalf("版本号 %d 重复：%s 与 %s", version, dup, name)
		}
		bucket[version] = name
	}
	return ups, downs
}

func TestMigrationFilesArePairedAndNamed(t *testing.T) {
	// 复用生产的路径解析（同包非导出函数），避免测试与实现各写一套目录上溯逻辑。
	dir, err := resolveMigrationsDir(zap.NewNop())
	if err != nil {
		t.Fatalf("定位 migrations 目录失败: %v", err)
	}
	ups, downs := parseMigrationVersions(t, dir)
	if len(ups) == 0 {
		t.Fatal("没发现任何 up 迁移")
	}
	for v, up := range ups {
		if _, ok := downs[v]; !ok {
			t.Fatalf("%s 缺对应的 down 迁移（版本 %d 无法回滚）", up, v)
		}
	}
	for v, down := range downs {
		if _, ok := ups[v]; !ok {
			t.Fatalf("%s 缺对应的 up 迁移（版本 %d 是孤儿 down）", down, v)
		}
	}
}

// TestMigrationsLoadViaRunner 用生产同一个源驱动器（iofs）把每个版本读一遍。
// RunMigrations 就是用 iofs 打开这个目录，所以这条比手工解析更贴近真实路径：
// 它挡的是"文件在、名字也对，但迁移工具读不出正文"。
func TestMigrationsLoadViaRunner(t *testing.T) {
	dir, err := resolveMigrationsDir(zap.NewNop())
	if err != nil {
		t.Fatalf("定位 migrations 目录失败: %v", err)
	}
	src, err := iofs.New(os.DirFS(dir), ".")
	if err != nil {
		t.Fatalf("iofs 打开 migrations 目录失败: %v", err)
	}
	defer src.Close()

	ups, _ := parseMigrationVersions(t, dir)
	for v := range ups {
		// 生产驱动器只提供 ReadUp / ReadDown 两个入口，这里把它们排成一列逐个验。
		for _, part := range []struct {
			name string
			read func(source.Driver, uint) (io.ReadCloser, string, error)
		}{
			{"up", func(d source.Driver, ver uint) (io.ReadCloser, string, error) { return d.ReadUp(ver) }},
			{"down", func(d source.Driver, ver uint) (io.ReadCloser, string, error) { return d.ReadDown(ver) }},
		} {
			// 前面的 TestMigrationFilesArePairedAndNamed 已断言 up/down 成对且命名合规，
			// 所以这里任何 error 都是"文件在但驱动器读不出正文"，不必再分错误类型。
			r, identifier, err := part.read(src, uint(v))
			if err != nil {
				t.Fatalf("版本 %d 的 %s 迁移读取失败（identifier=%s）: %v", v, part.name, identifier, err)
			}
			body, err := io.ReadAll(r)
			_ = r.Close()
			if err != nil {
				t.Fatalf("版本 %d 的 %s 正文读取失败: %v", v, part.name, err)
			}
			if strings.TrimSpace(string(body)) == "" {
				t.Fatalf("版本 %d 的 %s 迁移正文为空", v, part.name)
			}
		}
	}
}
