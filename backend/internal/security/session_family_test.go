// 令牌族（#1756 第二批）的行为锁：族是**会话隔离**的唯一有效落点，因此这组断言必须存在。
//
// 判据只从外部行为看：签出来的令牌在哪个入口被接受、轮换后还属不属于原来的族。
// 不测私有字段（aud 的具体形态是实现细节）。
package security

import (
	"context"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TestValuationTokenIsNotAcceptedAsWeb 估值族令牌贴到主站入口必须被拒；本族入口接受。
func TestValuationTokenIsNotAcceptedAsWeb(t *testing.T) {
	sess := newTestSession(nil)
	access, _, err := sess.IssuePairFor(FamilyValuation, 7, "u7", "hrwai_user")
	if err != nil {
		t.Fatalf("签发估值族令牌失败: %v", err)
	}
	if _, err := sess.VerifyAccessFor(FamilyWeb, access); err == nil {
		t.Error("估值族令牌被主站入口接受了：会话隔离失效")
	}
	claims, err := sess.VerifyAccessFor(FamilyValuation, access)
	if err != nil {
		t.Fatalf("本族入口拒绝了本族令牌: %v", err)
	}
	if claims.UserID != 7 {
		t.Errorf("claims 串了：user_id = %d，期望 7", claims.UserID)
	}
}

// TestWebTokenIsNotAcceptedAsValuation 反向：主站令牌贴到估值入口必须被拒。
func TestWebTokenIsNotAcceptedAsValuation(t *testing.T) {
	sess := newTestSession(nil)
	access, _, err := sess.IssuePair(7, "u7", "hrwai_user")
	if err != nil {
		t.Fatalf("签发主站令牌失败: %v", err)
	}
	if _, err := sess.VerifyAccessFor(FamilyValuation, access); err == nil {
		t.Error("主站令牌被估值入口接受了：会话隔离失效")
	}
}

// TestLegacyTokenWithoutAudienceIsWeb 切换前签发的存量令牌没有 aud —— 必须按 web 处理，
// 否则这次改动会把所有人踢下线（那次踢下线应由域名切换本身造成，不该由代码再叠一次）。
func TestLegacyTokenWithoutAudienceIsWeb(t *testing.T) {
	sess := newTestSession(nil)
	legacy := &Claims{
		UserID:    9,
		Account:   "u9",
		Role:      "hrwai_user",
		TokenType: TokenTypeAccess,
		RegisteredClaims: jwt.RegisteredClaims{
			ID:        "legacy-1",
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	raw, err := jwt.NewWithClaims(jwt.SigningMethodHS256, legacy).SignedString([]byte(testSecret))
	if err != nil {
		t.Fatalf("签发存量形态令牌失败: %v", err)
	}
	if _, err := sess.VerifyAccessFor(FamilyWeb, raw); err != nil {
		t.Errorf("无 aud 的存量令牌被主站入口拒了（会把所有人踢下线）: %v", err)
	}
	if _, err := sess.VerifyAccessFor(FamilyValuation, raw); err == nil {
		t.Error("无 aud 的存量令牌被估值入口接受了：估值入口只该认本族")
	}
}

// TestRotateRefreshPreservesFamily 轮换必须保住原来的族：否则估值的 refresh 一续期
// 就变成主站令牌，下一次请求就会被自己的入口拒掉（表现为「用一会儿就掉登录」）。
func TestRotateRefreshPreservesFamily(t *testing.T) {
	sess := newTestSession(nil)
	_, refresh, err := sess.IssuePairFor(FamilyValuation, 3, "u3", "hrwai_user")
	if err != nil {
		t.Fatalf("签发失败: %v", err)
	}
	access, nextRefresh, err := sess.RotateRefresh(context.Background(), refresh)
	if err != nil {
		t.Fatalf("轮换失败: %v", err)
	}
	if _, err := sess.VerifyAccessFor(FamilyValuation, access); err != nil {
		t.Errorf("轮换后的 access 丢了估值族: %v", err)
	}
	if claims, err := sess.ValidateRefresh(nextRefresh); err != nil {
		t.Fatalf("轮换后的 refresh 无效: %v", err)
	} else if claims.Family() != FamilyValuation {
		t.Errorf("轮换后的 refresh 族变成 %q，期望 %q", claims.Family(), FamilyValuation)
	}
}