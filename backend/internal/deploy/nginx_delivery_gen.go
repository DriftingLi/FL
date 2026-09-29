// 本文件：gen-deploy 的第二份产物——**RGW 直出面的投递分档 nginx map**（#1364 / ADR-0066 决策 2）。
//
// 背景（票 #1364 事实 2）：生产跑的是 `STORAGE_DRIVER=r2`，对象由 nginx 的 RGW location 直接
// `proxy_pass` 给 Ceph，**不经后端 staticHandler**，所以 ADR-0066 决策 2 的分档在生产是死代码
// （实测 `curl -sI https://www.gccsmile.com/avatars/….webp` 既无 content-disposition 也无
// x-content-type-options）。本票的决策是「在 nginx 层补齐分档，但**不在 nginx 手抄第二份规则**」——
// 规则的唯一事实源仍是 `service.fileTypeTable`，这里只做渲染。
//
// 落点形态：**就地覆写 `frontend/nginx-host.conf` 的生成区**（`# >>> …` 与 `# <<< …` 两行标记之间），
// 而不是另出一个需要挂载进容器的片段文件——compose 只把 `nginx-host.conf` 挂成
// `/etc/nginx/conf.d/default.conf.template`（`docker-compose.prod.yml` 的 frontend 卷），
// 多一个文件就多一条「打包清单漏了它 ⇒ 容器里 include 不到、nginx -t 起不来」的部署面风险
// （2026-09-13 的 PG_VOLUME 事故就是同一族：链路中段静默失配）。
//
// 与后端 `applyUploadDeliveryHeaders`（`internal/api/router.go`）的语义对齐：
//   - safe → 内联；unsafe / unknown → 强制下载；nosniff 两档都设。
//   - **Content-Type 刻意不在 nginx 侧覆写**（#1364 决策 3）：`attachment` 已经决定浏览器不渲染，
//     覆写要 `proxy_hide_header Content-Type` + 每扩展名 MIME map，收益为零、风险是把 RGW 给的
//     正确 MIME 弄丢。后端 local 面覆写是因为它自己就是出处（`MimeTypeOf`），nginx 不是。
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
	"forklift-training/internal/service"
)

// nginx 生成区的两行标记（判据即字面量，改一处要同步改测试）。
const (
	nginxDeliveryStart = "# >>> gen-deploy:nginx-delivery-map 生成区起（以下到止标记由 backend/cmd/gen-deploy 覆写，勿手改）"
	nginxDeliveryEnd   = "# <<< gen-deploy:nginx-delivery-map 生成区止"
)

// nginxHostConfRel 产物所在的仓库内相对路径。
const nginxHostConfRel = "frontend/nginx-host.conf"

// extPattern 生成物里允许的扩展名形态：小写字母数字（正则键直接拼 `~*\.ext$`，
// 出现别的字符就拒绝生成，绝不把元字符原样塞进 nginx 正则）。
var extPattern = regexp.MustCompile(`^[a-z0-9]+$`)

// classLiteral AST 里 class 字段写法 → 运行期分档值。空串 = 源码省了 class 字段，
// Go 零值即 FileTypeUnknown，与运行期同解。
var classLiteral = map[string]service.FileTypeClass{
	"":                service.FileTypeUnknown,
	"FileTypeUnknown": service.FileTypeUnknown,
	"FileTypeSafe":    service.FileTypeSafe,
	"FileTypeUnsafe":  service.FileTypeUnsafe,
}

// deliveryRow 类型表一行：扩展名 + 分档。
type deliveryRow struct {
	ext   string
	class service.FileTypeClass
}

// locateNginxHostConf 在 dir 下定位 frontend/nginx-host.conf。
// 独立成函数（而不是从 NginxDeliveryMapGen 里取）：包级变量初始化不许成环
// ——Render 要定位自己的产物，而产物路径声明在 Spec 里。
func locateNginxHostConf(dir string) (string, bool) {
	cand := filepath.Join(dir, "frontend", "nginx-host.conf")
	if st, err := os.Stat(cand); err == nil && !st.IsDir() {
		return cand, true
	}
	return "", false
}

// NginxDeliveryMapGen gen-deploy 第二产物：类型表 → frontend/nginx-host.conf 的生成区。
// Render 返回**整份文件**（生成区被覆写、区外内容原样保留），因此 codegen.AssertInSync
// 的「字节级全等」语义与 env.defaults 完全同形：手改生成区、或改了表不重生成，都直接红。
var NginxDeliveryMapGen = codegen.Spec{
	Name:     "gen-deploy",
	Hint:     nginxHostConfRel,
	Locate:   locateNginxHostConf,
	NotFound: NotFoundRepoRoot,
	Render:   RenderNginxHostConf,
}

// RenderNginxHostConf 渲染整份 nginx-host.conf：把生成区替换为由类型表现算的 map。
//
// 行尾一律归一成 LF（与 codegen.AssertInSync 比对 `got` 时的归一同一口径），否则 Windows
// 上 core.autocrlf=true 的检出（CRLF）会让「现算结果」与「磁盘内容」永远差一个 \r。
func RenderNginxHostConf() (string, error) {
	path, err := codegen.Resolve("", locateNginxHostConf, NotFoundRepoRoot)
	if err != nil {
		return "", err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("读取 %s 失败: %w", path, err)
	}
	full := strings.ReplaceAll(string(raw), "\r\n", "\n")
	region, err := RenderNginxDeliveryRegion()
	if err != nil {
		return "", err
	}
	return spliceRegion(full, region)
}

// RenderNginxDeliveryRegion 渲染生成区正文（map 块，以换行结尾）。
// 入参固定取自类型表 —— 测试要喂合成行请直接调 renderDeliveryRegion。
func RenderNginxDeliveryRegion() (string, error) {
	rows, err := fileTypeTableRows()
	if err != nil {
		return "", err
	}
	if len(rows) == 0 {
		return "", errors.New("从 service.fileTypeTable 解析到 0 行：生成器失效（表形态变了？本函数需要跟着改）")
	}
	return renderDeliveryRegion(rows)
}

// renderDeliveryRegion 把行集合渲染成生成区正文（纯函数、确定性输出、无时间戳）。
//
// 与运行期分档的对应关系只有一处（deliveryOf）：safe → inline，unsafe 与 unknown → attachment。
// 单独拆出来是为了让「表里多一行 ⇒ 生成物必须多一行」这条判据可被测（喂合成行），
// 而不必去动只读的类型表。
func renderDeliveryRegion(rows []deliveryRow) (string, error) {
	// 三档分别成组，档内按扩展名升序（输出确定性：同表两次生成必须逐字节相同）。
	sorted := slices.Clone(rows)
	slices.SortFunc(sorted, func(a, b deliveryRow) int { return strings.Compare(a.ext, b.ext) })
	var safe, unsafe_, unknown []string
	for _, r := range sorted {
		if r.ext != normExtKey(r.ext) {
			return "", fmt.Errorf("扩展名键 %q 不是小写形态（生成物与 nginx 正则都要按它拼，拒绝生成）", r.ext)
		}
		if !extPattern.MatchString(r.ext) {
			return "", fmt.Errorf("扩展名 %q 不是小写字母数字（nginx 正则键拼不出来，拒绝生成）", r.ext)
		}
		line := deliveryLine(r.ext, deliveryOf(r.class))
		switch r.class {
		case service.FileTypeSafe:
			safe = append(safe, line)
		case service.FileTypeUnsafe:
			unsafe_ = append(unsafe_, line)
		default:
			unknown = append(unknown, line)
		}
	}

	var b strings.Builder
	b.WriteString("# ===== RGW 直出的投递分档（ADR-0066 决策 2 / 票 #1364）=====\n")
	b.WriteString("# 唯一事实源：backend/internal/service/file_type_table.go 的 class 列；\n")
	b.WriteString("# 再生成：cd backend && go run ./cmd/gen-deploy；\n")
	b.WriteString("# 同步契约：backend/internal/deploy/nginx_delivery_gen_test.go（改表不改本段即红）。\n")
	b.WriteString("# 语义与后端 staticHandler 的 applyUploadDeliveryHeaders 对齐：safe 内联、\n")
	b.WriteString("# unsafe 与 unknown 强制下载。Content-Type 不在这里覆写（#1364 决策 3）——\n")
	b.WriteString("# attachment 已是下载语义，覆写要 proxy_hide_header + 每扩展名 MIME map，\n")
	b.WriteString("# 收益为零且会把 RGW 回的正确 MIME 弄丢。\n")
	b.WriteString("map $uri $delivery_disposition {\n")
	b.WriteString("    # unknown：未登记扩展名（含无扩展名对象）一律强制下载 —— ADR-0066 决策 6 锁②。\n")
	b.WriteString(pad("default", "attachment") + "\n")
	writeGroup(&b, "safe 档", "inline（内联预览：PDF 内联与图片/视频直出依赖它）", safe)
	writeGroup(&b, "unsafe 档", "attachment（显式登记：行为同 default，登记是为了让「想过它」留在生成物里）", unsafe_)
	writeGroup(&b, "unknown 档", "attachment（表里显式登记为未识别的行；未登记的走上面的 default，同一语义）", unknown)
	b.WriteString("}\n")
	return b.String(), nil
}

// normExtKey 与 service 侧 normExt 同口径的归一（去点转小写），这里只做校验用。
func normExtKey(ext string) string {
	return strings.ToLower(strings.TrimPrefix(strings.TrimSpace(ext), "."))
}

// writeGroup 输出一档的注释抬头与逐行映射；空档不输出抬头（某一档为 0 项时，
// 生成物里不该留一句悬空注释）。
func writeGroup(b *strings.Builder, label, note string, lines []string) {
	if len(lines) == 0 {
		return
	}
	fmt.Fprintf(b, "\n    # %s：%d 项 —— %s\n", label, len(lines), note)
	for _, l := range lines {
		b.WriteString(l)
	}
}

// deliveryLine 一行映射：`    ~*\.ext$   inline;`。键用大小写不敏感的正则（`~*`），
// 与后端 normExt 的「去点转小写」同一口径；行尾带换行。
func deliveryLine(ext, disposition string) string {
	return pad(fmt.Sprintf("~*\\.%s$", ext), disposition) + "\n"
}

// pad 把「键 + 取值」写成对齐到第 deliveryValueColumn 列的一行（以分号结尾）。
// 对齐纯排版：宽度按键长算，键过长时退化为一个空格（语法不变）。
func pad(key, value string) string {
	const deliveryValueColumn = 24
	padLen := deliveryValueColumn - deliveryKeyIndent - len(key)
	if padLen < 1 {
		padLen = 1
	}
	return strings.Repeat(" ", deliveryKeyIndent) + key + strings.Repeat(" ", padLen) + value + ";"
}

// deliveryKeyIndent map 条目（含 default）的缩进列数。
const deliveryKeyIndent = 4

// deliveryOf 分档 → nginx map 取值（safe 内联，其余强制下载）。
func deliveryOf(class service.FileTypeClass) string {
	if class == service.FileTypeSafe {
		return "inline"
	}
	return "attachment"
}

// fileTypeTableRows 取类型表的**全集**与分档：扩展名键从源码 AST 读，分档值以运行期
// service.FileTypeClassOf 为准，并与 AST 字面量逐行对账。
//
// 为什么必须去解析另一个包的源码：`fileTypeTable` 是包内私有，导出面里没有任何「列出全部
// 扩展名」的函数（AllowedExtensionsFor 按上传类别取，unsafe 与投稿专用行的 upload 都是空串，
// 拿它拼不出全集）——而「拼不出全集」正是要防的那件事：漏一行就等于让那个扩展名静默落到
// default，看上去生成了、其实表里新增的类型没人管。同形态先例见 topology_test.go 用 AST 读
// internal/config/config.go 的默认值。本函数只读，不改 service 一行。
//
// 两侧对账不过 ⇒ 直接报错拒绝生成（fail closed）：解析失配时宁可红，不要产出一份缺档的 map。
func fileTypeTableRows() ([]deliveryRow, error) {
	root, err := repoRoot()
	if err != nil {
		return nil, err
	}
	src := filepath.Join(root, "backend", "internal", "service", "file_type_table.go")
	rows, err := parseFileTypeTable(src)
	if err != nil {
		return nil, err
	}
	slices.SortFunc(rows, func(a, b deliveryRow) int { return strings.Compare(a.ext, b.ext) })
	for _, r := range rows {
		if got := service.FileTypeClassOf(r.ext); got != r.class {
			return nil, fmt.Errorf("类型表解析失配: %s 在源码声明为 %v、service.FileTypeClassOf 返回 %v（生成器与表已不同形，请检查本文件的解析）",
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
					return nil, fmt.Errorf("扩展名 %q 不是小写字母数字（nginx 正则键拼不出来，拒绝生成）", ext)
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
	// 键唯一性：map 类型本身保证，但重复行会产成两个同名 nginx 正则键（nginx 直接判重失败）。
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
// 但把常量引用改成限定式（`service.FileTypeSafe`）不该让生成器失配 ——
// 名字对上 classLiteral 即可，两侧的差别由下面的运行期对账兜住。
func classOfEntry(entry *ast.CompositeLit) (service.FileTypeClass, error) {
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
			return service.FileTypeUnknown, errors.New("class 字段不是标识符形态（FileTypeSafe / FileTypeUnsafe / FileTypeUnknown）")
		}
		cls, ok := classLiteral[name]
		if !ok {
			return service.FileTypeUnknown, fmt.Errorf("class 字段 %s 不在已知三档里", name)
		}
		return cls, nil
	}
	return service.FileTypeUnknown, nil
}

// spliceRegion 用 region 替换两行标记之间的内容（标记本身保留）。
func spliceRegion(full, region string) (string, error) {
	for _, marker := range []string{nginxDeliveryStart, nginxDeliveryEnd} {
		if n := strings.Count(full, marker); n != 1 {
			return "", fmt.Errorf("生成区标记在 %s 里出现 %d 次（必须恰好 1 次）：%s", nginxHostConfRel, n, marker)
		}
	}
	i := strings.Index(full, nginxDeliveryStart) + len(nginxDeliveryStart)
	j := strings.Index(full, nginxDeliveryEnd)
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
