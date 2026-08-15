// 图形验证码契约测试：开关关闭放行 / 开启时无证被拒、错证被拒、对证通过、复用被拒。
package api

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"

	"forklift-training/internal/captcha"
)

// fetchCaptcha 获取一张验证码并返回其 id 与答案（答案从内存存储读取）。
func fetchCaptcha(t *testing.T, r *gin.Engine, store *memCodeStore) (id, answer string) {
	t.Helper()
	w := codeAuthRequest(r, http.MethodGet, "/api/captcha", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/captcha 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}
	var resp struct {
		Data struct {
			ID    string `json:"id"`
			Image string `json:"image"`
		} `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil || resp.Data.ID == "" {
		t.Fatalf("captcha 响应解析失败: %s", w.Body.String())
	}
	if !strings.HasPrefix(resp.Data.Image, "data:image/png;base64,") {
		t.Errorf("image 应为 PNG data URL")
	}
	raw, ok := store.m[captcha.StoreKey(resp.Data.ID)]
	if !ok {
		t.Fatalf("答案未写入存储: id=%s", resp.Data.ID)
	}
	return resp.Data.ID, raw
}

// TestCaptcha_DisabledByDefault 开关关闭（默认）时 send-code 不受人机验证影响。
func TestCaptcha_DisabledByDefault(t *testing.T) {
	r, _, _, _ := newCodeAuthTestRouter(t)
	w := codeAuthRequest(r, http.MethodPost, "/api/auth/email/send-code",
		map[string]interface{}{"email": "nocap@example.com", "purpose": "register"}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("开关关闭时 send-code 应放行: %d\nbody=%s", w.Code, w.Body.String())
	}
}

// TestCaptcha_EnabledContract 开关开启时的完整契约。
func TestCaptcha_EnabledContract(t *testing.T) {
	r, store, _, _, _ := newCodeAuthTestRouterX(t, true)

	// 未带验证码 → 400
	w := codeAuthRequest(r, http.MethodPost, "/api/auth/email/send-code",
		map[string]interface{}{"email": "cap@example.com", "purpose": "register"}, "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "图形验证码") {
		t.Fatalf("未带验证码应 400: %d\nbody=%s", w.Code, w.Body.String())
	}

	// 错误验证码 → 400
	id, _ := fetchCaptcha(t, r, store)
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/email/send-code",
		map[string]interface{}{"email": "cap@example.com", "purpose": "register", "captcha_id": id, "captcha_value": "00000"}, "")
	if w.Code != http.StatusBadRequest || !strings.Contains(w.Body.String(), "图形验证码") {
		t.Fatalf("错误验证码应 400: %d\nbody=%s", w.Code, w.Body.String())
	}

	// 正确验证码 → 发码成功
	id, answer := fetchCaptcha(t, r, store)
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/email/send-code",
		map[string]interface{}{"email": "cap@example.com", "purpose": "register", "captcha_id": id, "captcha_value": answer}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("正确验证码应发码成功: %d\nbody=%s", w.Code, w.Body.String())
	}

	// 复用同一验证码 → 400（已消费）
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/email/send-code",
		map[string]interface{}{"email": "cap2@example.com", "purpose": "register", "captcha_id": id, "captcha_value": answer}, "")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("复用验证码应 400: %d\nbody=%s", w.Code, w.Body.String())
	}

	// 手机号通道同样生效
	id, answer = fetchCaptcha(t, r, store)
	w = codeAuthRequest(r, http.MethodPost, "/api/auth/phone/send-code",
		map[string]interface{}{"phone": "13800138000", "purpose": "register", "captcha_id": id, "captcha_value": answer}, "")
	if w.Code != http.StatusOK {
		t.Fatalf("手机号通道带验证码应成功: %d\nbody=%s", w.Code, w.Body.String())
	}
}

// TestCaptcha_GenerateShapeLock 冻结 GET /api/captcha 的 data 顶层键集 {id, image}（对照 GenerateCaptchaDTO）。
func TestCaptcha_GenerateShapeLock(t *testing.T) {
	gin.SetMode(gin.TestMode)
	store := newMemCodeStore()
	r := gin.New()
	RegisterCaptchaRoutes(r, captcha.NewService(store))

	w := codeAuthRequest(r, http.MethodGet, "/api/captcha", nil, "")
	if w.Code != http.StatusOK {
		t.Fatalf("GET /api/captcha 状态码 = %d\nbody=%s", w.Code, w.Body.String())
	}

	var body struct {
		Data map[string]any `json:"data"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("captcha 响应解析失败: %s", w.Body.String())
	}
	assertDictKeys(t, body.Data, []string{"id", "image"})
}
