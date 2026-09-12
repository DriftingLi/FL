package middleware

import (
	"net"

	"github.com/gin-gonic/gin"
)

// ClientIP 返回服务端认定的客户端 IP——全后端唯一的取 IP 口径
// （限流键、审计日志、访问日志都经此处，不再各自调用 c.ClientIP()）。
//
// 是否采信 X-Forwarded-For / X-Real-IP 由路由装配时的 Engine.SetTrustedProxies 决定
// （见 api.NewRouter 与 config.Config.TrustedProxies）：只有直连对端落在可信代理网段内
// 才采信请求头，且从右往左跳过可信代理、取第一个不可信地址。nginx 用
// $proxy_add_x_forwarded_for 把真实客户端「追加」在末尾，因此真实客户端胜出、
// 客户端自带的伪造值被忽略；直连（对端不可信）时一律取 TCP 对端地址。
//
// 返回值做规范化（IPv4-mapped IPv6 归一成 IPv4 点分形式），保证同一客户端在限流键
// 与审计记录里是同一个字符串。对端地址无法解析时返回空串——此时限流退化为同一个桶
// （fail closed），而不是凭空造出一个客户端身份。
func ClientIP(c *gin.Context) string {
	ip := c.ClientIP()
	if ip == "" {
		return ""
	}
	// 转发头里的写法不受控（::ffff:1.2.3.4 与 1.2.3.4 混用），归一后再作为键使用。
	parsed := net.ParseIP(ip)
	if parsed == nil {
		return ip
	}
	if v4 := parsed.To4(); v4 != nil {
		return v4.String()
	}
	return parsed.String()
}
