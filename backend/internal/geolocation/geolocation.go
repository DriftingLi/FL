// Package geolocation 把客户端 IP 解析成属地（省 / 市两档）。
//
// 属地是**发布那一刻的快照**（ADR-0045）：本包只做「IP → 属地」这一次纯查询，
// 不持有请求状态、不做缓存，调用方负责在发帖/回复时取一次并落库。
//
// 数据来源是**内嵌**的 ip2region 离线库（MIT）：构建可复现、不引入外部账号、
// 也不把用户 IP 发给第三方。库会过时——更新方式是人工替换 data/ 下的 xdb 后重新构建，
// 版本记录见 DataVersion（口径见 CONTEXT.md 的「属地」与 docs/adr/ADR-0045）。
package geolocation

import (
	_ "embed"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/lionsoul2014/ip2region/binding/golang/xdb"
)

// ip2regionV4 内嵌的 IPv4 离线库。
//
// 只做 IPv4：本部署的域名没有 AAAA 记录，客户端一律以 IPv4 到达；
// IPv6 地址会落到库的版本校验上，返回空属地（不报错）。
//
//go:embed data/ip2region_v4.xdb
var ip2regionV4 []byte

// 数据文件来源，供人工替换与留痕（替换后同步更新 DataSourceMD5）。
const (
	DataSource       = "ip2region_v4.xdb"
	DataSourceOrigin = "https://github.com/lionsoul2014/ip2region/raw/master/data/ip2region_v4.xdb"
	DataSourceMD5    = "c30f0c57e4ba14cab6aa5a15e976ac1c"
)

// Region 属地：省 / 市两档，任一项都可能为空。
//
// 空值语义（展示侧「为空即整段不渲染」）：内网 / 保留地址 / 库中无该段 / 库只给到省级。
type Region struct {
	Province string
	City     string
}

// IsEmpty 两档都空即属地为空。
func (r Region) IsEmpty() bool { return r.Province == "" && r.City == "" }

// DataVersion 返回内嵌库自身的版本记录：ip2region 把生成时间写在 xdb 头里，
// 比在代码里手抄一个版本号可信。取不到时返回空串（不影响解析）。
func DataVersion() string {
	_, header, err := load()
	if err != nil || header == nil {
		return ""
	}
	return fmt.Sprintf("%s@%s", DataSource, time.Unix(int64(header.CreatedAt), 0).UTC().Format(time.RFC3339))
}

// Resolve 解析 IP 属地。**永不返回 error**：非法 IP、IPv6、内网 / 保留地址、库加载失败
// 一律返回零值 Region——属地是氛围标识，解析不出来不该影响发帖（spec #887「边界与降级」）。
func Resolve(ip string) Region {
	searcher, _, err := load()
	if err != nil || searcher == nil {
		return Region{}
	}
	raw, err := searcher.Search(strings.TrimSpace(ip))
	if err != nil {
		// 非法地址与版本不符（IPv6）都走这里：落空而不是报错。
		return Region{}
	}
	return parseRegion(raw)
}

var (
	loadOnce sync.Once
	loaded   *xdb.Searcher
	loadedHd *xdb.Header
	loadErr  error
)

// load 懒加载内嵌库（进程内一次）。Search 只读 content buffer，可并发调用。
func load() (*xdb.Searcher, *xdb.Header, error) {
	loadOnce.Do(func() {
		// 先读头：头里的 CreatedAt 就是版本记录，坏库在这一步就会失败。
		header, err := xdb.NewHeader(ip2regionV4[:xdb.HeaderInfoLength])
		if err != nil {
			loadErr = fmt.Errorf("读取 %s 头失败: %w", DataSource, err)
			return
		}
		searcher, err := xdb.NewWithBuffer(xdb.IPv4, ip2regionV4)
		if err != nil {
			loadErr = fmt.Errorf("加载 %s 失败: %w", DataSource, err)
			return
		}
		loaded, loadedHd = searcher, header
	})
	return loaded, loadedHd, loadErr
}

// parseRegion 解析 ip2region 的原始记录串——竖线分隔的「国家|省州|市|ISP|国家代码」。
// v4 xdb 实测：114.114.114.114 → 中国|江苏省|南京市|0|CN。
//
// 字段不足或占位值（0 / Reserved）一律折成空串：
//   - 内网 / 保留地址返回的正是 Reserved|Reserved|Reserved|0|0，折空后展示侧自然整段不渲染；
//   - 境外地址的「省」是州 / 省级地名（如 United States|California|0|...），原样保留——
//     它就是那条 IP 的属地；本批没有国家级字段，不做国别映射（见 ADR-0045）。
func parseRegion(raw string) Region {
	fields := strings.Split(raw, "|")
	if len(fields) < 3 {
		return Region{}
	}
	return Region{
		Province: normalizeRegionField(fields[1]),
		City:     normalizeRegionField(fields[2]),
	}
}

// normalizeRegionField 把库里的占位值折成空串（0 = 该级无数据，Reserved = 保留地址）。
func normalizeRegionField(v string) string {
	switch strings.TrimSpace(v) {
	case "", "0", "Reserved":
		return ""
	}
	return strings.TrimSpace(v)
}
