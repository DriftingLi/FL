package credentialscope

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// 前端常量的窄域生成器（形态与 internal/authz、internal/deploy、internal/apitypes 同构）：
// 声明表 → 纯函数渲染 → 生成物 → 字节级同步契约 + 覆盖锁。

// tsTemplate 生成物模板（%s = 登记项字面量）。
const tsTemplate = `// 生成文件，勿手改（ADR-0047 §4 收尾 / spec #940 片六）。
// 唯一事实源：backend/internal/credentialscope/registry.go。
// 再生成：cd backend && go run ./cmd/gen-credscope
// 同步契约：backend/internal/credentialscope/codegen_test.go（字节全等 + 覆盖锁）。
//
// 「不随当前证件切换重装」的消费者登记表：前端源码里每一处 %s
// 都必须在下表出现 —— 覆盖锁扫前端源码比对，新增例外不登记即报红。
//
// 运行期判据本身**不变**（调用点仍传字面量，零行为变化）：本文件不是运行时开关，
// 而是「有哪些例外、各自为什么」的唯一可核对答案。判据定义处见
// composables/useAsyncPage.ts（它自己的文档注释里也会出现该字面量，不计入消费者）。

export interface CredentialScopeOptOut {
  /** 相对 frontend/src 的路径。 */
  readonly file: string
  /** 为什么它不随当前证件过滤（域理由，评审看这一句）。 */
  readonly reason: string
}

/** 例外登记表（按路径排序，生成序稳定）。 */
export const CREDENTIAL_SCOPE_OPT_OUTS: readonly CredentialScopeOptOut[] = [
%s]
`

// RenderFrontendTS 渲染前端登记表（纯函数、确定性输出：按 File 排序，无时间戳）。
func RenderFrontendTS() (string, error) {
	if len(OptOuts) == 0 {
		return "", errors.New("例外登记表为空，拒绝生成空文件（真的一个例外都没有？先核对覆盖锁）")
	}
	sorted := append([]OptOut(nil), OptOuts...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].File < sorted[j].File })
	var b strings.Builder
	for _, o := range sorted {
		if o.File == "" || o.Reason == "" {
			return "", fmt.Errorf("登记项不完整: %+v", o)
		}
		fmt.Fprintf(&b, "  { file: '%s', reason: '%s' },\n", tsEscape(o.File), tsEscape(o.Reason))
	}
	return fmt.Sprintf(tsTemplate, Marker, b.String()), nil
}

// tsEscape 单引号字符串转义（登记表里目前只有中文与斜杠，转义是为将来加引号时不炸）。
func tsEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "'", "\\'")
}

// ScanOptOutFiles 扫描前端源码，返回含 Marker 的消费者文件（相对 srcDir，斜杠分隔、已排序）。
//
// 排除：判据定义处（DefinitionFile）、测试文件（__tests__ / .spec.）、node_modules。
func ScanOptOutFiles(srcDir string) ([]string, error) {
	found := map[string]bool{}
	err := filepath.WalkDir(srcDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			name := d.Name()
			if name == "node_modules" || name == "__tests__" || name == "dist" {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".vue") && !strings.HasSuffix(path, ".ts") {
			return nil
		}
		if strings.HasSuffix(path, ".spec.ts") {
			return nil
		}
		raw, readErr := os.ReadFile(path)
		if readErr != nil {
			return readErr
		}
		if !strings.Contains(string(raw), Marker) {
			return nil
		}
		rel, relErr := filepath.Rel(srcDir, path)
		if relErr != nil {
			return relErr
		}
		rel = filepath.ToSlash(rel)
		if rel == DefinitionFile || rel == GeneratedFile {
			return nil
		}
		found[rel] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	out := make([]string, 0, len(found))
	for f := range found {
		out = append(out, f)
	}
	sort.Strings(out)
	return out, nil
}

// RegisteredFiles 登记表里的文件集合（已排序）。
func RegisteredFiles() []string {
	out := make([]string, 0, len(OptOuts))
	for _, o := range OptOuts {
		out = append(out, o.File)
	}
	sort.Strings(out)
	return out
}
