// 本文件：gen-deploy 的第二份产物——**对象存储直出面的投递分档匹配器**（#1364 / ADR-0066 决策 2）。
//
// 背景（票 #1364 事实 2）：生产跑的是 `STORAGE_DRIVER=r2`，对象由反代直接 `reverse_proxy`
// 给 Ceph，**不经后端 staticHandler**，所以 ADR-0066 决策 2 的分档在生产是死代码
// （实测 `curl -sI https://www.gccsmile.com/avatars/….webp` 既无 content-disposition 也无
// x-content-type-options）。分档必须在反代层补齐，但**不在反代配置里手抄第二份规则**——
// 规则的唯一事实源仍是 `filestore.fileTypeTable`，这里只做渲染。
//
// 落点形态：**就地覆写 `frontend/Caddyfile` 的生成区**（两行标记之间），而不是另出一个
// 需要挂载进容器的片段文件——多一个文件就多一条「打包清单漏了它 ⇒ 容器里引用不到、
// caddy validate 起不来」的部署面风险（2026-09-13 的 PG_VOLUME 事故是同一族）。
//
// 与后端 `applyUploadDeliveryHeaders` 的语义对齐：
//   - safe → 内联；unsafe / unknown → 强制下载；nosniff 两档都设。
//   - **Content-Type 刻意不在反代侧覆写**（#1364 决策 3）：`attachment` 已经决定浏览器不渲染。
//
// 与 nginx 版的语义落差只有一处，必须显式保留：nginx 的 map 键用的是 `~*`（**不区分大小写**）
// 正则，而 Caddy 的 `path` 匹配**区分大小写** ⇒ 这里一律用 `path_regexp` 并以 `(?i)` 开头，
// 原样保留「`.JPG` 与 `.jpg` 同档」这条既有行为（票 #1364 的锁②依赖它）。
package deploy

import (
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"forklift-training/internal/codegen"
	"forklift-training/internal/filestore"
)

// Caddyfile 生成区的两行标记（判据即字面量，改一处要同步改测试）。
const (
	caddyDeliveryStart = "# >>> gen-deploy:caddy-delivery 生成区起（以下到止标记由 backend/cmd/gen-deploy 覆写，勿手改）"
	caddyDeliveryEnd   = "# <<< gen-deploy:caddy-delivery 生成区止"
)

// caddyfileRel 产物所在的仓库内相对路径。
const caddyfileRel = "frontend/Caddyfile"

// extPattern 生成物里允许的扩展名形态：小写字母数字（正则交替项直接拼进去，
// 出现别的字符就拒绝生成，绝不把元字符原样塞进正则）。
var extPattern = regexp.MustCompile(`^[a-z0-9]+$`)

// classLiteral AST 里 class 字段写法 → 运行期分档值。空串 = 源码省了 class 字段，
// Go 零值即 FileTypeUnknown，与运行期同解。
var classLiteral = map[string]filestore.FileTypeClass{
	"":                filestore.FileTypeUnknown,
	"FileTypeUnknown": filestore.FileTypeUnknown,
	"FileTypeSafe":    filestore.FileTypeSafe,
	"FileTypeUnsafe":  filestore.FileTypeUnsafe,
}

// deliveryRow 类型表一行：扩展名 + 分档。
type deliveryRow struct {
	ext   string
	class filestore.FileTypeClass
}

// locateCaddyfile 在 dir 下定位 frontend/Caddyfile。
// 独立成函数（而不是从 CaddyDeliveryGen 里取）：包级变量初始化不许成环
// ——Render 要定位自己的产物，而产物路径声明在 Spec 里。
func locateCaddyfile(dir string) (string, bool) {
	cand := filepath.Join(dir, "frontend", "Caddyfile")
	if st, err := os.Stat(cand); err == nil && !st.IsDir() {
		return cand, true
	}
	return "", false
}

// CaddyDeliveryGen gen-deploy 第二产物：类型表 → frontend/Caddyfile 的生成区。
// Render 返回**整份文件**（生成区被覆写、区外内容原样保留），因此 codegen.AssertInSync
// 的「字节级全等」语义与 env.defaults 完全同形：手改生成区、或改了表不重生成，都直接红。
var CaddyDeliveryGen = codegen.Spec{
	Name:     "gen-deploy",
	Hint:     caddyfileRel,
	Locate:   locateCaddyfile,
	NotFound: NotFoundRepoRoot,
	Render:   RenderCaddyfile,
}

// RenderCaddyfile 渲染整份 Caddyfile：把生成区替换为由类型表现算的匹配器。
//
// 行尾一律归一成 LF（与 codegen.AssertInSync 比对 got 时的归一同一口径），否则 Windows
// 上 core.autocrlf=true 的检出（CRLF）会让「现算结果」与「磁盘内容」永远差一个 \r。
func RenderCaddyfile() (string, error) {
	path, err := codegen.Resolve("", locateCaddyfile, NotFoundRepoRoot)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	full := strings.ReplaceAll(string(raw), "\r\n", "\n")
	region, err := RenderCaddyDeliveryRegion()
	if err != nil {
		return "", err
	}
	return spliceRegion(full, region)
}

// RenderCaddyDeliveryRegion 渲染生成区正文（命名匹配器，以换行结尾）。
// 入参固定取自类型表 —— 测试要喂合成行请直接调 renderDeliveryRegion。
func RenderCaddyDeliveryRegion() (string, error) {
	rows, err := fileTypeTableRows()
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", errors.New("从 filestore.fileTypeTable 解析到 0 行：生成器失效（表形态变了？本函数需要跟着改）")
	}
	return renderDeliveryRegion(rows)
}

// renderDeliveryRegion 把行集合渲染成生成区正文（纯函数、确定性输出、无时间戳）。
//
// 每档一条 `path_regexp`，档内扩展名按字典序升序（输出确定性：同表两次生成逐字节相同）。
// 单独拆出来是为了让「表里多一行 ⇒ 生成物必须多一行」这条判据可被测（喂合成行），
// 而不必去动只读的类型表。
func renderDeliveryRegion(rows []deliveryRow) (string, error) {
	sorted := slices.Clone(rows)
	slices.SortFunc(sorted, func(a, b deliveryRow) int { return strings.Compare(a.ext, b.ext) })

	var safe, unsafe_, unknown []string
	for _, r := range sorted {
		if r.ext != normExtKey(r.ext) {
			return "", fmt.Errorf("扩展名键 %q 不是小写形态（生成物与匹配器都要按它拼，拒绝生成）", r.ext)
		}
		if !extPattern.MatchString(r.ext) {
			return "", fmt.Errorf("扩展名 %q 不是小写字母数字（正则交替项拼不出来，拒绝生成）", r.ext)
		}
		switch r.class {
		case filestore.FileTypeSafe:
			safe = append(safe, r.ext)
		case filestore.FileTypeUnsafe:
			unsafe_ = append(unsafe_, r.ext)
		default:
			unknown = append(unknown, r.ext)
		}
	}

	var b strings.Builder
	b.WriteString("# ===== 对象存储直出的投递分档（ADR-0066 决策 2 / 票 #1364）=====\n")
	b.WriteString("# 唯一事实源：backend/internal/filestore/file_type_table.go 的 class 列；\n")
	b.WriteString("# 再生成：cd backend && go run ./cmd/gen-deploy；\n")
	b.WriteString("# 同步契约：backend/internal/deploy/caddy_delivery_gen_test.go（改表不改本段即红）。\n")
	b.WriteString("# 语义与后端 staticHandler 的 applyUploadDeliveryHeaders 对齐：safe 内联、\n")
	b.WriteString("# unsafe 与 unknown 强制下载。Content-Type 不在这里覆写（#1364 决策 3）——\n")
	b.WriteString("# attachment 已是下载语义，覆写收益为零且会把 RGW 回的正确 MIME 弄丢。\n")
	b.WriteString("# 匹配器一律 path_regexp + (?i)：nginx 原用 ~* 不区分大小写，Caddy 的 path 区分，\n")
	b.WriteString("# 这条 (?i) 就是「.JPG 与 .jpg 同档」的唯一保证。\n")
	writeMatcher(&b, "delivery_inline", "safe 档", "inline（内联预览：PDF 内联与图片/视频直出依赖它）", safe)
	writeMatcher(&b, "delivery_unsafe", "unsafe 档", "attachment（显式登记：行为同兜底，登记是为了让「想过它」留在生成物里）", unsafe_)
	writeMatcher(&b, "delivery_unknown", "unknown 档", "attachment（表里显式登记为未识别的行；未登记的扩展名走兜底，同一语义）", unknown)
	return b.String(), nil
}

// writeMatcher 输出一档的注释抬头与命名匹配器；空档不输出（某一档为 0 项时，
// 生成物里不该留一条悬空匹配器）。
func writeMatcher(b *strings.Builder, name, label, note string, exts []string) {
	if len(exts) == 0 {
		return
	}
	fmt.Fprintf(b, "\n# %s：%d 项 —— %s\n", label, len(exts), note)
	fmt.Fprintf(b, "@%s path_regexp (?i)\\.(%s)$\n", name, strings.Join(exts, "|"))
}

// normExtKey 与 service 侧 normExt 同口径的归一（去点转小写），这里只做校验用。
func normExtKey(ext string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
}

// fileTypeTableRows 取类型表的**全集**与分档：扩展名键从源码 AST 读，分档值以运行期
// filestore.FileTypeClassOf 为准，并与 AST 字面量逐行对账。
//
// 为什么必须去解析另一个包的源码：`fileTypeTable` 是包内私有，导出面里没有任何「列出全部
// 扩展名」的函数——而「拼不出全集」正是要防的那件事：漏一行就等于让那个扩展名静默落到
// 兜底档，看上去生成了、其实表里新增的类型没人管。
//
// 两侧对账不过 ⇒ 直接报错拒绝生成（fail closed）：解析失配时宁可红，不要产出一份缺档的规则。
func fileTypeTableRows() ([]deliveryRow, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	src := filepath.Join(root, "backend", "internal", "filestore", "file_type_table.go")
	rows, err := parseFileTypeTable(src)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(rows, func(a, b deliveryRow) int { return strings.Compare(a.ext, b.ext) })
	for _, r := range rows {
		if got := filestore.FileTypeClassOf(r.ext); got != r.class {
			return nil, fmt.Errorf("类型表解析失配: %s 在源码声明为 %v、filestore.FileTypeClassOf 返回 %v（生成器与表已不同形，请检查本文件的解析）",
				r.ext, r.class, got)
		}
	}
	return rows, nil
}

// parseFileTypeTable 解析 `var fileTypeTable = map[string]fileTypeEntry{ … }` 的字面量，
// 返回每行的扩展名与 class 字面量对应的分档。
func parseFileTypeTable(path string) ([]deliveryRow, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return nil, fmt.Errorf("解析 %s 失败: %w", path, err)
	}
	var rows []deliveryRow
	var found bool
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			continue
		}
		for _, spec := range gd.Specs {
			vs, ok := spec.(*ast.ValueSpec)
			if !ok || len(vs.Values) != 1 || len(vs.Names) != 1 || vs.Names[0].Name != "fileTypeTable" {
				continue
			}
			lit, ok := vs.Values[0].(*ast.CompositeLit)
			if !ok {
				return nil, fmt.Errorf("%s: fileTypeTable 不是复合字面量，本解析器需要跟着改", filepath.Base(path))
			}
			found = true
			for _, el := range lit.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					continue
				}
				keyLit, ok := kv.Key.(*ast.BasicLit)
				if !ok || keyLit.Kind != token.STRING {
					return nil, fmt.Errorf("%s: fileTypeTable 的键不是字符串字面量", filepath.Base(path))
				}
				ext, err := strconv.Unquote(keyLit.Value)
				if err != nil {
					return nil, fmt.Errorf("%s: 扩展名 %s 解不开引号: %w", filepath.Base(path), keyLit.Value, err)
				}
				if !extPattern.MatchString(ext) {
					return nil, fmt.Errorf("扩展名 %q 不是小写字母数字（匹配器拼不出来，拒绝生成）", ext)
				}
				entry, ok := kv.Value.(*ast.CompositeLit)
				if !ok {
					return nil, fmt.Errorf("%s: %s 的行值不是 fileTypeEntry 复合字面量", filepath.Base(path), ext)
				}
				class, err := classOfEntry(entry)
				if err != nil {
					return nil, fmt.Errorf("%s: %s — %w", filepath.Base(path), ext, err)
				}
				rows = append(rows, deliveryRow{ext: ext, class: class})
			}
		}
	}
	if !found {
		return nil, fmt.Errorf("%s: 没找到 var fileTypeTable = map[…]fileTypeEntry{…}（表的声明形态变了？）", filepath.Base(path))
	}
	if len(rows) == 0 {
		return nil, fmt.Errorf("%s: fileTypeTable 为空", filepath.Base(path))
	}
	// 键唯一性：map 类型本身保证，但重复行会产成同一交替项两次。
	seen := map[string]bool{}
	for _, r := range rows {
		if seen[r.ext] {
			return nil, fmt.Errorf("%s: 扩展名 %s 在表里出现两次", filepath.Base(path), r.ext)
		}
		seen[r.ext] = true
	}
	return rows, nil
}

// classOfEntry 读一行里的 `class:` 字段；省了该字段就是 Go 零值 FileTypeUnknown。
//
// 取值形态两种都要认：表内写的是同包裸标识符（`class: FileTypeSafe`），
// 但把常量引用改成限定式（`filestore.FileTypeSafe`）不该让生成器失配 ——
// 名字对上 classLiteral 即可，两侧的差别由下面的运行期对账兜住。
func classOfEntry(entry *ast.CompositeLit) (filestore.FileTypeClass, error) {
	for _, f := range entry.Elts {
		kv, ok := f.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		id, ok := kv.Key.(*ast.Ident)
		if !ok || id.Name != "class" {
			continue
		}
		name := ""
		switch v := kv.Value.(type) {
		case *ast.Ident:
			name = v.Name
		case *ast.SelectorExpr:
			name = v.Sel.Name
		default:
			return filestore.FileTypeUnknown, errors.New("class 字段不是标识符形态（FileTypeSafe / FileTypeUnsafe / FileTypeUnknown）")
		}
		cls, ok := classLiteral[name]
		if !ok {
			return filestore.FileTypeUnknown, fmt.Errorf("class 字段 %s 不在已知三档里", name)
		}
		return cls, nil
	}
	return filestore.FileTypeUnknown, nil
}

// spliceRegion 用 region 替换两行标记之间的内容（标记本身保留）。
func spliceRegion(full, region string) (string, error) {
	for _, marker := range []string{caddyDeliveryStart, caddyDeliveryEnd} {
		if n := strings.Count(full, marker); n != 1 {
			return "", fmt.Errorf("生成区标记在 %s 里出现 %d 次（必须恰好 1 次）：%s", caddyfileRel, n, marker)
		}
	}
	i := strings.Index(full, caddyDeliveryStart) + len(caddyDeliveryStart)
	j := strings.Index(full, caddyDeliveryEnd)
	if j < i {
		return "", errors.New("生成区标记顺序颠倒（止标记出现在起标记之前）")
	}
	return full[:i] + "\n" + region + full[j:], nil
}

// repoRoot 从当前目录逐级向上找仓库根（判据同 EnvDefaultsGen：docker-compose.prod.yml）。
func repoRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if st, err := os.Stat(filepath.Join(dir, "docker-compose.prod.yml")); err == nil && !st.IsDir() {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New(NotFoundRepoRoot)
		}
		dir = parent
	}
}
