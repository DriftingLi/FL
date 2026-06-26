// Package migrate 实现基于 golang-migrate 的迁移运行器 CLI。
package migrate

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	"github.com/golang-migrate/migrate/v4/source/iofs"
)

// RunMigrations 执行迁移，direction 为 "up" 或 "down"。
func RunMigrations(dsn, direction string) error {
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
	// 默认相对项目根目录
	abs, err := filepath.Abs("migrations")
	if err != nil {
		return "", err
	}
	if _, err := os.Stat(abs); os.IsNotExist(err) {
		slog.Warn("migrations 目录不存在", "dir", abs)
	}
	return abs, nil
}
