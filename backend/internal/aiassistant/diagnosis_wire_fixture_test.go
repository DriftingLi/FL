// 诊断助手**线上传真值**夹具测试：夹具是原样回写的响应字节（不经 Go 类型回编码），
// 厂商改帧形状时这条路径会红，而「用 Go 类型回编码」的 fake 只会把「解析不出字段」
// 伪装成「字段为空」而恒绿。
//
// v1（lxc101 阻塞 JSON：code/data/sop_text/answer_images/[IMG:] 令牌）的传真夹具随
// v1 通道一起退役（票 2237）；这里保留的是 v2 的帧形状与图片归一契约。
package aiassistant

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"go.uber.org/zap"
)

// wireV2Done 2026-11-03 契约 §5.2 的 done 帧原样形状（answer 为清洗后 markdown，
// 图片以 OSS 公网链接出现；badges 九个固定词表之一；trace 排查用）。
const wireV2Done = `data: {"type":"done","answer":"# 制动系统异响 SOP\n\n## 一、前置安全准备\n1. 停机确认。\n\n![制动蓄能器](https://hrwai-guangzhou.oss-cn-beijing.aliyuncs.com/forklift/repair_img/linde/Linde_A-005-01__image-20260824100446677.png)\n\n2. 断开主电源插头。\n\n![别家图](https://cdn.example.com/x/y.png)\n\n## 二、验收\n恢复供电后试运行。","badges":[{"name":"知识检索","emoji":"📚","cls":"cap-rag"},{"name":"风险警示","emoji":"⚠️","cls":"cap-risk"}],"trace":["intent=diagnose","rag=hit"],"elapsed":11.8}

`

// TestDiagnosisWireV2_DoneFrame 传真帧驱动真实 adapter：白名单内图改写、白名单外降级、
// badges 原样三字段、trace 不进元数据。
func TestDiagnosisWireV2_DoneFrame(t *testing.T) {
	server := httptest.NewServer((&fakeDiagnosisServer{t: t, raw: wireV2Done}).handler())
	defer server.Close()

	ctx := WithDiagnosisMeta(context.Background())
	content, _, err := newDiagnosisForTest(server).Stream(ctx, ModelSelector{FeatureKey: FeatureFaultDiagnosis}, msgsSample("制动异响怎么查？", 0), nil)
	if err != nil {
		t.Fatalf("Stream 异常: %v", err)
	}

	// 白名单内：改成站内代理路径
	if !strings.Contains(content, "!["+"制动蓄能器"+"]("+diagnosisImageRoute+"linde/Linde_A-005-01__image-20260824100446677.png)") {
		t.Fatalf("白名单内图片应改写为站内代理: %q", content)
	}
	// 白名单外：降级为 alt 文字，原 URL 不出现
	if strings.Contains(content, "cdn.example.com") {
		t.Fatalf("白名单外图片不得留在正文: %q", content)
	}
	if !strings.Contains(content, "别家图") {
		t.Fatalf("白名单外图片应降级为 alt 文字: %q", content)
	}
	// 正文其余部分原样
	if !strings.Contains(content, "## 二、验收") || !strings.Contains(content, "恢复供电后试运行。") {
		t.Fatalf("正文结构被破坏: %q", content)
	}

	meta, ok := DiagnosisMetaFrom(ctx)
	if !ok || len(meta.Badges) != 2 {
		t.Fatalf("徽章应两条: %+v ok=%v", meta, ok)
	}
	if meta.Badges[1].Emoji != "⚠️" || meta.Badges[1].Cls != "cap-risk" {
		t.Fatalf("徽章三字段应原样: %+v", meta.Badges[1])
	}
	if meta.Elapsed != 11.8 {
		t.Fatalf("elapsed 应为 11.8，got %v", meta.Elapsed)
	}
}

// TestNormalizeDiagnosisImages 归一函数的独立用例（不走 HTTP）。
func TestNormalizeDiagnosisImages(t *testing.T) {
	base := "https://" + testOSSHostPath + testOSSBasePath

	cases := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "白名单内改写",
			in:   "![](https://" + testOSSHostPath + testOSSBasePath + "/linde/a.png)",
			want: "![诊断配图](" + diagnosisImageRoute + "linde/a.png)",
		},
		{
			name: "带 alt 的白名单内改写",
			in:   "![制动图](https://" + testOSSHostPath + testOSSBasePath + "/linde/a.png)",
			want: "![制动图](" + diagnosisImageRoute + "linde/a.png)",
		},
		{
			name: "白名单外降级为 alt",
			in:   "![第三方](https://cdn.example.com/a.png)",
			want: "第三方",
		},
		{
			name: "白名单外无 alt 用固定文案",
			in:   "![](https://cdn.example.com/a.png)",
			want: diagnosisFallbackAlt,
		},
		{
			name: "本站代理图二次归一不动",
			in:   "![已归一](" + diagnosisImageRoute + "linde/a.png)",
			want: "![已归一](" + diagnosisImageRoute + "linde/a.png)",
		},
		{
			name: "pdf 降级（图片语法里只会是坏图）",
			in:   "![手册](https://" + testOSSHostPath + testOSSBasePath + "/manual/a.pdf)",
			want: "手册",
		},
		{
			name: "http 明文降级",
			in:   "![明文](http://" + testOSSHostPath + testOSSBasePath + "/linde/a.png)",
			want: "明文",
		},
		{
			name: "正文无图原样",
			in:   "## 一、检查方向\n\n无图正文。",
			want: "## 一、检查方向\n\n无图正文。",
		},
		{
			name: "多图混合",
			in: "![一](https://" + testOSSHostPath + testOSSBasePath + "/images_all/a.png)\n" +
				"![二](https://cdn.example.com/b.png)",
			want: "![一](" + diagnosisImageRoute + "images_all/a.png)\n二",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := normalizeDiagnosisImages(base, c.in, zap.NewNop()); got != c.want {
				t.Fatalf("归一不符:\n got=%q\nwant=%q", got, c.want)
			}
		})
	}

	// 基准为空：所有图都降级（配置没给 OSS 基准就不服务任何图）
	t.Run("基准为空全降级", func(t *testing.T) {
		in := "![一](https://" + testOSSHostPath + testOSSBasePath + "/linde/a.png)"
		if got := normalizeDiagnosisImages("", in, zap.NewNop()); got != "一" {
			t.Fatalf("got %q", got)
		}
	})
}
