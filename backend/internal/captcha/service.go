package captcha

import (
	"context"
	"encoding/base64"
	"strconv"
	"time"

	"github.com/google/uuid"

	"forklift-training/internal/cache"
)

// TTL 验证码答案有效期。
const TTL = 2 * time.Minute

// Store 验证码答案存储接口（生产 Redis，测试内存）。
type Store interface {
	Get(ctx context.Context, key string) (string, error)
	Set(ctx context.Context, key, value string, ttl time.Duration) error
	Del(ctx context.Context, keys ...string) error
}

// RedisStore 基于全局 Redis 缓存的验证码存储。
type RedisStore struct{}

// Get 读取验证码答案。
func (RedisStore) Get(ctx context.Context, key string) (string, error) {
	return cache.Get(ctx, key)
}

// Set 写入验证码答案。
func (RedisStore) Set(ctx context.Context, key, value string, ttl time.Duration) error {
	return cache.Set(ctx, key, value, ttl)
}

// Del 删除验证码答案。
func (RedisStore) Del(ctx context.Context, keys ...string) error {
	return cache.Del(ctx, keys...)
}

func storeKey(id string) string { return cache.SafeKey("captcha", id) }

// Service 图形验证码服务：生成图片 + 校验（校验即消费）。
type Service struct {
	store Store
}

// NewService 构造图形验证码服务。
func NewService(store Store) *Service {
	return &Service{store: store}
}

// Generate 生成新验证码：渲染图片、答案写入 store，返回 id 与 base64 data URL。
func (s *Service) Generate(ctx context.Context) (id, imageURL string, err error) {
	eq := NewEquation()
	id = uuid.NewString()
	if err := s.store.Set(ctx, storeKey(id), strconv.Itoa(eq.Answer), TTL); err != nil {
		return "", "", err
	}
	return id, "data:image/png;base64," + base64.StdEncoding.EncodeToString(RenderPNG(eq.Text)), nil
}

// Verify 校验验证码；无论对错，答案都立即消费（删除），防止同图暴力重试。
func (s *Service) Verify(ctx context.Context, id, value string) bool {
	if id == "" || value == "" {
		return false
	}
	answer, err := s.store.Get(ctx, storeKey(id))
	_ = s.store.Del(ctx, storeKey(id))
	if err != nil {
		return false
	}
	return answer != "" && answer == value
}
