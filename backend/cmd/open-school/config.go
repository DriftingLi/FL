package main

import (
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"
)

// 默认值：与本地预览栈的端口口径对齐（后端 18080、学员站 18091），公开面占 18090。
// 三个端口都在 18xxx 段，因为 8080/5173 一带落在 Windows Hyper-V 的动态排除端口段内
// （见 backend/docker-compose.yml 的注释），换机会踩「port is already allocated」。
const (
	defaultListen    = "127.0.0.1:18090"
	defaultAPIBase   = "http://127.0.0.1:18080"
	defaultLearnBase = "http://training.localhost:18091"
)

// config 是本进程的全部配置。只有四个键，全部走环境变量（#2747 的独立进程边界：
// 公开面不读 .env、不读配置文件，部署时一个环境变量列表就够了）。
type config struct {
	// Listen 监听地址。默认只允许回环：宿主机直跑时进程不直接暴露全部网卡。
	Listen string
	// APIBase 只读后端 API 的基址（不带结尾斜杠）。
	APIBase string
	// LearnBase 受信任学员站的基址（不带结尾斜杠），用于「进入学习」整页导航。
	LearnBase string
	// AllowNonLoopback 显式允许绑定非回环地址。
	//
	// 为什么要有这个开关：容器里公开网校是独立进程，网关在**另一个**容器里按 Host
	// 反代进来，此时必须绑 0.0.0.0，绑回环则网关永远连不上（#2753 节点 E 实测）。
	// 但「能绑」不等于「默认绑」——宿主机直跑仍默认回环，非回环必须显式置真，
	// 让偏离安全默认的那一步在部署清单里看得见，启动日志里也留一条 Warn。
	AllowNonLoopback bool
}

// envTrue 解析布尔型环境变量：只认 1/true（大小写不敏感），其余一律当关。
// 不用 strconv.ParseBool 的完整集合（它认 t/T/TRUE 等）：部署清单里写 t 的
// 多半是手误，宁可让它落到「关」这一侧也不要静默放行。
func envTrue(raw string) bool {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "1", "true":
		return true
	default:
		return false
	}
}

// envConfig 从环境变量装配配置并校验。getenv 注入是为了测试能脱离真实环境。
func envConfig(getenv func(string) string) (config, error) {
	pick := func(key, fallback string) string {
		if v := strings.TrimSpace(getenv(key)); v != "" {
			return v
		}
		return fallback
	}
	cfg := config{
		Listen:           pick("OPEN_SCHOOL_LISTEN", defaultListen),
		APIBase:          pick("OPEN_SCHOOL_API", defaultAPIBase),
		LearnBase:        pick("OPEN_SCHOOL_LEARN_BASE", defaultLearnBase),
		AllowNonLoopback: envTrue(getenv("OPEN_SCHOOL_ALLOW_NON_LOOPBACK")),
	}
	if err := cfg.validate(); err != nil {
		return config{}, err
	}
	return cfg, nil
}

// validate 校验并规范化配置。规范化（去掉结尾斜杠）放在这里做一次，
// 后续拼 URL 时不必到处 TrimSuffix。
func (c *config) validate() error {
	host, port, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return fmt.Errorf("OPEN_SCHOOL_LISTEN 必须是 host:port 形式，收到 %q", c.Listen)
	}
	// 空主机（":18090"）等价于监听全部网卡，与 0.0.0.0 同一条判据。
	if !c.AllowNonLoopback && !isLoopbackHost(host) {
		return fmt.Errorf("OPEN_SCHOOL_LISTEN 只允许绑定回环地址（127.0.0.1 / ::1 / localhost），收到 %q；"+
			"容器内需经反向代理接入时设 OPEN_SCHOOL_ALLOW_NON_LOOPBACK=1", host)
	}
	if p, err := strconv.Atoi(port); err != nil || p < 1 || p > 65535 {
		return fmt.Errorf("OPEN_SCHOOL_LISTEN 端口非法，收到 %q", port)
	}

	api, err := normalizeBaseURL("OPEN_SCHOOL_API", c.APIBase)
	if err != nil {
		return err
	}
	c.APIBase = api

	learn, err := normalizeBaseURL("OPEN_SCHOOL_LEARN_BASE", c.LearnBase)
	if err != nil {
		return err
	}
	c.LearnBase = learn
	return nil
}

// bindsAllInterfaces 判断监听地址是否落在「所有网卡」上（0.0.0.0 / :: / 空主机）。
// 单独拎出来是给启动日志用：容器里绑它是正常的，但要 Warn 一句留痕——
// 这是偏离默认安全姿态的事实，事后查「谁把端口开到公网上了」得有记录。
func (c *config) bindsAllInterfaces() bool {
	host, _, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return false
	}
	if host == "" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsUnspecified()
}

// isLoopbackHost 判断监听主机是否回环。
// 空串（`:18090`）也算不合法：那等价于监听全部网卡。
func isLoopbackHost(host string) bool {
	if host == "" {
		return false
	}
	if strings.EqualFold(host, "localhost") {
		return true
	}
	if ip := net.ParseIP(host); ip != nil {
		return ip.IsLoopback()
	}
	return false
}

// normalizeBaseURL 校验基址并去掉结尾斜杠。
// 只接受 http/https：这两个基址都会进 href，留着别的 scheme（如 javascript:）
// 就是把页面改写成脚本执行的入口。
func normalizeBaseURL(key, raw string) (string, error) {
	u, err := url.Parse(raw)
	if err != nil {
		return "", fmt.Errorf("%s 不是合法 URL：%w", key, err)
	}
	if u.Scheme != "http" && u.Scheme != "https" {
		return "", fmt.Errorf("%s 只接受 http/https，收到 %q", key, u.Scheme)
	}
	if u.Host == "" {
		return "", fmt.Errorf("%s 缺少主机名：%q", key, raw)
	}
	if u.User != nil {
		return "", fmt.Errorf("%s 不允许带用户信息（user:pass@）：%q", key, raw)
	}
	if u.RawQuery != "" || u.Fragment != "" {
		return "", fmt.Errorf("%s 不允许带查询串或片段：%q", key, raw)
	}
	return strings.TrimRight(u.String(), "/"), nil
}
