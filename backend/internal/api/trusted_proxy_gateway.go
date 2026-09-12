package api

// 可信代理的启动自检：只诊断、不自动信任。
//
// 本部署 frontend 用 host 网络模式，经宿主机回环发布端口访问后端，容器看到的 TCP 对端
// 就是宿主机在本网桥上的地址（= 本机默认网关）。换宿主机或重建 docker 网络后网段会变，
// TRUSTED_PROXIES 里的旧网关不再匹配——症状不是报错，而是所有请求被算作同一个客户端
// （按 IP 限流退化为全局限流）。这条日志把那个静默退化变成启动时的一句话。
import (
	"net"
	"os"
	"strconv"
	"strings"

	"go.uber.org/zap"
)

// defaultRoutePath Linux 路由表：默认网关的唯一来源。
const defaultRoutePath = "/proc/net/route"

// warnIfGatewayNotTrusted 在声明了可信代理、但本机默认网关不在其中时告警。
// 网关由调用方传入（applyTrustedProxies 传 defaultGateway()），便于测试覆盖两侧分支。
// 不自动信任网关：网关是不是一个会清洗请求头的代理是部署事实，不能猜
// （见 docker-compose.prod.yml 的 TRUSTED_PROXIES 注释）。
func warnIfGatewayNotTrusted(gateway string, proxies []string, logger *zap.Logger) {
	if gateway == "" || ipCoveredByProxies(proxies, gateway) {
		return
	}
	logger.Warn("本机默认网关不在 TRUSTED_PROXIES 内：若代理与后端同网络栈（本部署 frontend 即如此），"+
		"客户端 IP 会退化为该网关地址，所有请求被算作同一个客户端",
		zap.String("default_gateway", gateway), zap.Strings("trusted_proxies", proxies))
}

// defaultGateway 返回本机默认网关地址；读不到（非 Linux、无默认路由）返回空串。
func defaultGateway() string {
	data, err := os.ReadFile(defaultRoutePath)
	if err != nil {
		return ""
	}
	return parseDefaultGateway(string(data))
}

// parseDefaultGateway 从 /proc/net/route 内容解析默认网关：Destination 为 0 的那一行，
// Gateway 字段是 16 进制小端序 IPv4。
func parseDefaultGateway(routeTable string) string {
	for _, line := range strings.Split(routeTable, "\n") {
		// 列：Iface Destination Gateway Flags RefCnt Use Metric Mask MTU Window IRTT
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[1] != "00000000" {
			continue
		}
		return parseLittleEndianHexIPv4(fields[2])
	}
	return ""
}

// parseLittleEndianHexIPv4 把 /proc/net/route 的网关字段（如 01F0A8C0）转成点分 IPv4。
func parseLittleEndianHexIPv4(hex string) string {
	v, err := strconv.ParseUint(hex, 16, 32)
	if err != nil {
		return ""
	}
	return net.IPv4(byte(v), byte(v>>8), byte(v>>16), byte(v>>24)).String()
}

// ipCoveredByProxies 判断 ip 是否落在可信代理列表（IP 或 CIDR）内。
func ipCoveredByProxies(proxies []string, ip string) bool {
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return false
	}
	for _, p := range proxies {
		if !strings.Contains(p, "/") {
			if entry := net.ParseIP(p); entry != nil && entry.Equal(parsed) {
				return true
			}
			continue
		}
		if _, cidr, err := net.ParseCIDR(p); err == nil && cidr.Contains(parsed) {
			return true
		}
	}
	return false
}
