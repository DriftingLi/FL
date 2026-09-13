package authz

import (
	"os"
	"path/filepath"
	"testing"
)

// 生成物同步契约（ADR-0047 §1 / spec #928 决策 5）：前端 config/authz.ts 必须与能力表
// 渲染结果**字节级全等**。手改生成物、或改了能力表却忘记再生成，本测试即红——
// 与 ai_features_codegen_test.go 同一形态（ADR-0030 先例）。
func TestFrontendAuthzTSInSync(t *testing.T) {
	want, err := RenderFrontendAuthzTS()
	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}
	path := filepath.Join("..", "..", "..", "frontend", "src", "config", "authz.ts")
	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("读取生成物 %s 失败（应先运行 go run ./cmd/gen-authz）: %v", path, err)
	}
	if string(got) != want {
		t.Fatalf("前端 authz.ts 与能力表不同步：请 cd backend && go run ./cmd/gen-authz\n--- want ---\n%s\n--- got ---\n%s", want, string(got))
	}
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
