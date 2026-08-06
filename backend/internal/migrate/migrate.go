// Package migrate 实现基于 golang-migrate 的迁移运行器 CLI。
package migrate

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// RunMigrations 执行迁移，direction 为 "up"、"down" 或 "force"。
// force 用法: force <version>，强制设置数据库迁移版本并清除 dirty 标志。
// 用于数据库因迁移执行中断进入 dirty 状态后的修复。
func RunMigrations(dsn, direction string, args ...string) error {
	migrationsDir, err := resolveMigrationsDir()
	if err != nil {
		return err
	}

	// 使用 iofs 源驱动器直接通过 OS 文件系统读取迁移文件，
	// 避免 Windows 路径在 file:// URL 中被错误解析。
	src, err := iofs.New(os.DirFS(migrationsDir), ".")
	if err != nil {
		return fmt.Errorf("打开 migrations 目录失败: %w", err)
	}
	defer src.Close()

	m, err := migrate.NewWithSourceInstance("iofs", src, dsn)
	if err != nil {
		return fmt.Errorf("创建 migrate 实例失败: %w", err)
	}
	defer m.Close()

	switch direction {
	case "up":
		if err := m.Up(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migrate up 失败: %w", err)
		}
	case "down":
		if err := m.Down(); err != nil && err != migrate.ErrNoChange {
			return fmt.Errorf("migrate down 失败: %w", err)
		}
	case "force":
		// force <version>: 强制设置数据库迁移版本,清除 dirty 标志
		if len(args) == 0 {
			return fmt.Errorf("force 用法: migrate force <version>,请提供目标版本号")
		}
		version, err := strconv.Atoi(args[0])
		if err != nil || version < 0 {
			return fmt.Errorf("无效的版本号: %s(必须为非负整数)", args[0])
		}
		if err := m.Force(version); err != nil {
			return fmt.Errorf("migrate force %d 失败: %w", version, err)
		}
		slog.Info("迁移版本已强制设置", "version", version)
	case "status":
		// 查看当前迁移版本和 dirty 状态
		version, dirty, err := m.Version()
		if err != nil {
			return fmt.Errorf("查询迁移版本失败: %w", err)
		}
		fmt.Printf("当前迁移版本: %d\n", version)
		if dirty {
			fmt.Println("状态: DIRTY (数据库处于脏状态,迁移被中断,需手动修复)")
			fmt.Println("修复指南:")
			fmt.Println("  1. 确认该版本的迁移是否已完整应用(检查相关表/对象是否存在)")
			fmt.Println("  2. 若已完整应用: migrate force <version>  (标记为已应用且干净)")
			fmt.Println("  3. 若未完整应用: migrate force <version-1>  (回退到上一干净版本)")
			fmt.Println("     然后重新执行: migrate up")
		} else {
			fmt.Println("状态: CLEAN (干净)")
		}
		_ = version
	default:
		return fmt.Errorf("未知的迁移方向: %s", direction)
	}
	return nil
}

// resolveMigrationsDir 解析 migrations 目录的绝对路径。
func resolveMigrationsDir() (string, error) {
	// 优先使用环境变量
	if dir := os.Getenv("MIGRATIONS_DIR"); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		return abs, nil
	}
	// 从 CWD 向上查找 migrations 目录：
	// go test ./... 的测试二进制在包目录运行，go run/cmd 在仓库根运行
	dir, err := filepath.Abs(".")
	if err != nil {
		return "", err
	}
	for {
		candidate := filepath.Join(dir, "migrations")
		if st, err := os.Stat(candidate); err == nil && st.IsDir() {
			return candidate, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	slog.Warn("migrations 目录不存在", "dir", "migrations")
	abs, _ := filepath.Abs("migrations")
	return abs, nil
}
