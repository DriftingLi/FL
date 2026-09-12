package api

// 可信代理启动自检（ticket #888 的运维面）：只测纯函数——路由表内容与网关都按参数注入，
// 保证 Windows / WSL / CI 三处口径一致，也不依赖跑测试的机器真的有默认路由。
import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

func TestParseDefaultGateway(t *testing.T) {
	cases := []struct {
		name  string
		table string
		want  string
	}{
		{
			name: "production 容器实测（172.19.0.1）",
			table: "Iface\tDestination\tGateway \tFlags\tRefCnt\tUse\tMetric\tMask\t\tMTU\tWindow\tIRTT\n" +
				"eth0\t00000000\t010013AC\t0003\t0\t0\t0\t00000000\t0\t0\t0\n" +
				"eth0\t000013AC\t00000000\t0001\t0\t0\t0\t0000FFFF\t0\t0\t0\n",
			want: "172.19.0.1",
		},
		{
			name:  "testing 容器实测（192.168.240.1）",
			table: "eth0\t00000000\t01F0A8C0\t0003\t0\t0\t0\t00000000\t0\t0\t0\n",
			want:  "192.168.240.1",
		},
		{name: "只有同网段路由、没有默认路由", table: "eth0\t00F0A8C0\t00000000\t0001\n", want: ""},
		{name: "空内容", table: "", want: ""},
		{name: "只有表头", table: "Iface\tDestination\tGateway\tFlags\n", want: ""},
		{name: "网关字段损坏", table: "eth0\t00000000\tZZZZZZZZ\t0003\n", want: ""},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := parseDefaultGateway(tc.table); got != tc.want {
				t.Errorf("parseDefaultGateway() = %q, want %q", got, tc.want)
			}
		})
	}
}

func TestIPCoveredByProxies(t *testing.T) {
	proxies := []string{"127.0.0.1/32", "172.19.0.1", "192.168.240.0/20"}
	cases := []struct {
		name string
		ip   string
		want bool
	}{
		{"裸 IP 精确命中", "172.19.0.1", true},
		{"CIDR 段内（网关）", "192.168.240.1", true},
		{"CIDR 段内（其它地址）", "192.168.250.9", true},
		{"CIDR 段外", "192.168.239.9", false},
		{"局域网地址不在列表", "172.17.1.41", false},
		{"空值", "", false},
		{"非法地址", "not-an-ip", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := ipCoveredByProxies(proxies, tc.ip); got != tc.want {
				t.Errorf("ipCoveredByProxies(%v, %q) = %v, want %v", proxies, tc.ip, got, tc.want)
			}
		})
	}
}

// 网段变化（换宿主机 / 重建 docker 网络）时必须在启动日志里点名，而不是静默退化成全局限流。
func TestWarnIfGatewayNotTrusted(t *testing.T) {
	cases := []struct {
		name    string
		gateway string
		proxies []string
		want    bool
	}{
		{"网关在列表内不告警", "172.19.0.1", []string{"172.19.0.1"}, false},
		{"网关落在 CIDR 内不告警", "192.168.240.1", []string{"192.168.240.0/20"}, false},
		{"网段变了要告警", "172.19.0.1", []string{"192.168.240.1"}, true},
		{"读不到网关（非 Linux / 无默认路由）不告警", "", []string{"192.168.240.1"}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			core, logs := observer.New(zap.WarnLevel)
			warnIfGatewayNotTrusted(tc.gateway, tc.proxies, zap.New(core))
			if got := logs.Len() > 0; got != tc.want {
				t.Fatalf("告警 = %v, want %v（日志：%v）", got, tc.want, logs.All())
			}
			if tc.want {
				if v := logs.All()[0].ContextMap()["default_gateway"]; v != tc.gateway {
					t.Errorf("告警应带上实际网关便于排障: got %v, want %q", v, tc.gateway)
				}
			}
		})
	}
}
