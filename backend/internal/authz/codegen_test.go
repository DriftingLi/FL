package authz

import (
	"testing"

	"forklift-training/internal/codegen"
)

// 生成物同步契约（ADR-0047 §1 / spec #928 决策 5）：前端 config/authz.ts 必须与能力表
// 渲染结果**字节级全等**。手改生成物、或改了能力表却忘记再生成，本测试即红。
// 定位与提示走 codegen.AssertInSync（ADR-0053 §9），五个域形态一致。
func TestFrontendAuthzTSInSync(t *testing.T) {
	codegen.AssertInSync(t, FrontendAuthzGen)
}

// 渲染是纯函数：连续两次渲染字节级一致（无时间戳、无随机序），否则同步契约无法成立。
func TestRenderFrontendAuthzTSDeterministic(t *testing.T) {
	a, err := RenderFrontendAuthzTS()
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	b, err := RenderFrontendAuthzTS()
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	if a != b {
		t.Fatal("两次渲染结果不一致：生成物必须确定性输出")
	}
	if len(a) == 0 {
		t.Fatal("渲染结果不得为空")
	}
}
