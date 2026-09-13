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

// tsTemplate 生成物模板（%s₁ = 判据字面量，%s₂ = 登记项字面量）。
const tsTemplate = `// 生成文件，勿手改（ADR-0047 §4 收尾 / spec #940 片六）。
// 唯一事实源：backend/internal/credentialscope/registry.go。
// 再生成：cd backend && go run ./cmd/gen-credscope
// 同步契约：backend/internal/credentialscope/codegen_test.go（字节全等 + 覆盖锁）。
//
// 「不随当前证件切换重装」的消费者登记表：前端源码里每一处 %s
// 都必须在下表出现 —— 覆盖锁**按出现次数**扫前端源码比对，新增例外不登记即报红。
//
// 注意：本表管的是「切换后是否**重装**」，与「数据按哪个证件**过滤**」是两件事（ADR-0047 §4）。
//
// 运行期判据本身**不变**（调用点仍传字面量，零行为变化）：本文件不是运行时开关，
// 而是「有哪些例外、各自为什么」的唯一可核对答案。判据定义处见
// composables/useAsyncPage.ts（它自己的文档注释里也会出现该字面量，不计入消费者）。

export interface CredentialScopeOptOut {
  /** 相对 frontend/src 的路径。 */
  readonly file: string
  /** 该文件里 opt-out 字面量的出现次数（同一文件再加一处例外也要改登记表）。 */
  readonly count: number
  /** 为什么它不随当前证件重装（域理由，评审看这一句）。 */
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
		if o.File == "" || o.Reason == "" || o.Count <= 0 {
			return "", fmt.Errorf("登记项不完整: %+v", o)
		}
		fmt.Fprintf(&b, "  { file: '%s', count: %d, reason: '%s' },\n", tsEscape(o.File), o.Count, tsEscape(o.Reason))
	}
	return fmt.Sprintf(tsTemplate, Marker, b.String()), nil
}

// tsEscape 单引号字符串转义（登记表里目前只有中文与斜杠，转义是为将来加引号时不炸）。
func tsEscape(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	return strings.ReplaceAll(s, "'", "\\'")
}

// ScanOptOutCounts 扫描前端源码，返回「消费者文件 → opt-out 出现次数」（斜杠路径）。
//
// 排除：判据定义处（DefinitionFile）、生成物自身（GeneratedFile）、测试文件（__tests__ / .spec.）、
// node_modules 与构建产物。
func ScanOptOutCounts(srcDir string) (map[string]int, error) {
	found := map[string]int{}
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
		n := strings.Count(string(raw), Marker)
		if n == 0 {
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
		found[rel] = n
		return nil
	})
	if err != nil {
		return nil, err
	}
	return found, nil
}

// RegisteredCounts 登记表里的「文件 → 次数」表。
func RegisteredCounts() map[string]int {
	out := make(map[string]int, len(OptOuts))
	for _, o := range OptOuts {
		out[o.File] = o.Count
	}
	return out
}

// RegisteredFiles 登记表里的文件集合（已排序；供生成物头部与人工核对使用）。
func RegisteredFiles() []string {
	out := make([]string, 0, len(OptOuts))
	for _, o := range OptOuts {
		out = append(out, o.File)
	}
	sort.Strings(out)
	return out
}
