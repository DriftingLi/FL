package config

import (
	"strings"
	"testing"
)

// 外部诊断 v2 通道的自洽校验（ADR-0091 决策 4 + 票 2237 D 节）。
//
// 判据的形状是「要么都没配，要么配齐且自洽」：默认部署（Key 空）必须能起来，
// 否则这条锁会把整个部署卡死；而 Key 一旦非空，OSS 基准就成了白名单的唯一依据，
// 缺了或不合法的后果是「图集体消失但没有任何报错」——那比起不来更难查。
func TestValidateDiagnosisV2(t *testing.T) {
	const goodBase = "https://hrwai-guangzhou.oss-cn-beijing.aliyuncs.com/forklift/repair_img"

	tests := []struct {
		name    string
		apiKey  string
		ossBase string
		wantErr string // 空 = 期望通过
	}{
		{name: "都没配（默认部署态）", apiKey: "", ossBase: "", wantErr: ""},
		{name: "只有 Key 也算没配", apiKey: "  ", ossBase: "", wantErr: ""},
		{
			name:   "Key 配齐且 OSS 基准合法",
			apiKey: "k" + strings.Repeat("0", 31), ossBase: goodBase, wantErr: "", // 32 位是契约里的形状
		},
		{
			name:   "Key 非空但 OSS 基准缺失",
			apiKey: "k123", ossBase: "",
			wantErr: "必须同时配置 DIAGNOSIS_V2_OSS_BASE",
		},
		{
			name:   "OSS 基准非 https",
			apiKey: "k123", ossBase: "http://hrwai-guangzhou.oss-cn-beijing.aliyuncs.com/forklift/repair_img",
			wantErr: "必须是 https 地址",
		},
		{
			name:   "OSS 基准前缀错（对方 oss_base 的 /create 分支）",
			apiKey: "k123", ossBase: "https://hrwai-guangzhou.oss-cn-beijing.aliyuncs.com/create",
			wantErr: "的路径必须以 /forklift/repair_img 开头",
		},
		{
			name:   "OSS 基准前缀只是擦边（/forklift/repair_imgX 不算）",
			apiKey: "k123", ossBase: "https://hrwai-guangzhou.oss-cn-beijing.aliyuncs.com/forklift/repair_img2/a.png",
			wantErr: "的路径必须以 /forklift/repair_img 开头",
		},
		{
			name:   "OSS 基准缺主机名",
			apiKey: "k123", ossBase: "https:///forklift/repair_img",
			wantErr: "缺少主机名",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateDiagnosisV2(tt.apiKey, tt.ossBase)
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("期望通过，实得错误: %v", err)
				}
				return
			}
			if err == nil {
				t.Fatalf("期望报错（含 %q），实得 nil", tt.wantErr)
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("错误文案不符：期望含 %q，实得 %q", tt.wantErr, err.Error())
			}
		})
	}
}

// Validate 的挂点语义：这条校验在**所有环境**都跑（与可信代理同形），
// 而不是只挂 production —— 默认部署走 testing 面，那边配错了同样会「图全消失」。
func TestValidate_DiagnosisV2AppliesOutsideProduction(t *testing.T) {
	cfg := &Config{
		AppEnv:            "development",
		DiagnosisV2APIKey: "k123",
		DiagnosisV2OSSBase: "http://hrwai-guangzhou.oss-cn-beijing.aliyuncs.com" +
			"/forklift/repair_img",
	}
	if err := cfg.Validate(); err == nil {
		t.Fatal("非生产环境的非法诊断配置也必须启动失败")
	}
}
