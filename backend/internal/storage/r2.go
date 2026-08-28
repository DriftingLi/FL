package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	awscfg "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	smithyhttp "github.com/aws/smithy-go/transport/http"
)

// R2Storage S3 兼容对象存储。
//
// 文件上传到指定 bucket，返回 "https://<publicDomain>/<key>" 形式的公开访问 URL。
// 前提：bucket 已开启公开访问（Cloudflare R2 自定义域名；自建 RGW 用 bucket policy）。
// 默认 endpoint 为 Cloudflare R2；传入 endpoint 时可接自建 S3（如 Ceph RGW）。
type R2Storage struct {
	client       *s3.Client
	bucket       string
	publicDomain string
}

// NewR2Storage 创建对象存储实例。
//
// 参数：
//   - endpoint: 自定义 S3 endpoint（自建 RGW 等）；空则使用 Cloudflare R2 默认
//   - accountID: Cloudflare 账号 ID（endpoint 空时用于拼装 R2 endpoint）
//   - accessKeyID / secretAccessKey: S3/R2 凭证
//   - bucket: bucket 名称
//   - publicDomain: 绑定的公开域名（如 https://cdn.example.com），尾部斜杠会被去除
func NewR2Storage(ctx context.Context, endpoint, accountID, accessKeyID, secretAccessKey, bucket, publicDomain string) (*R2Storage, error) {
	if endpoint == "" {
		endpoint = fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	}
	publicDomain = strings.TrimSuffix(publicDomain, "/")

	cfg, err := awscfg.LoadDefaultConfig(ctx,
		awscfg.WithRegion("auto"),
		awscfg.WithCredentialsProvider(credentials.NewStaticCredentialsProvider(accessKeyID, secretAccessKey, "")),
	)
	if err != nil {
		return nil, fmt.Errorf("加载 R2 配置失败: %w", err)
	}

	client := s3.NewFromConfig(cfg, func(o *s3.Options) {
		o.BaseEndpoint = aws.String(endpoint)
		o.UsePathStyle = true
	})

	return &R2Storage{
		client:       client,
		bucket:       bucket,
		publicDomain: publicDomain,
	}, nil
}

// Save 上传内容到 R2，返回 https://<publicDomain>/<key> 形式的公开 URL。
func (s *R2Storage) Save(ctx context.Context, key string, content []byte, contentType string) (string, error) {
	input := &s3.PutObjectInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(content),
		ContentType: aws.String(contentType),
	}
	if _, err := s.client.PutObject(ctx, input); err != nil {
		return "", fmt.Errorf("R2 PutObject 失败: %w", err)
	}
	return s.publicDomain + "/" + key, nil
}

// Delete 从 R2 删除文件，URL 为空时直接返回 nil。
func (s *R2Storage) Delete(ctx context.Context, url string) error {
	if url == "" {
		return nil
	}
	key := s.urlToKey(url)
	if key == "" {
		return nil
	}
	_, err := s.client.DeleteObject(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return fmt.Errorf("R2 DeleteObject 失败: %w", err)
	}
	return nil
}

// Exists 通过 HeadObject 检查 R2 中是否存在该文件。
// 仅 404 NotFound 视为文件不存在；其余错误（网络/权限等）原样返回，避免掩盖真实故障。
func (s *R2Storage) Exists(ctx context.Context, url string) (bool, error) {
	if url == "" {
		return false, nil
	}
	key := s.urlToKey(url)
	if key == "" {
		return false, nil
	}
	_, err := s.client.HeadObject(ctx, &s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		var respErr *smithyhttp.ResponseError
		if errors.As(err, &respErr) && respErr.HTTPStatusCode() == http.StatusNotFound {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// urlToKey 从完整 URL 中解析出 R2 key。
// URL 格式: https://<publicDomain>/<key>
// 兼容直接传入 key 或 /static/uploads/<key> 的情况（local 模式遗留 URL）。
func (s *R2Storage) urlToKey(url string) string {
	prefix := s.publicDomain + "/"
	if strings.HasPrefix(url, prefix) {
		return strings.TrimPrefix(url, prefix)
	}
	// 兼容 local 模式 URL
	return strings.TrimPrefix(url, "/static/uploads/")
}

// Get 流式读取 R2 对象内容（GetObject → Body），调用方负责关闭。
func (s *R2Storage) Get(ctx context.Context, url string) (io.ReadCloser, error) {
	if url == "" {
		return nil, fmt.Errorf("url 为空")
	}
	key := s.urlToKey(url)
	if key == "" || key == url {
		return nil, fmt.Errorf("无法从 URL 解析对象 key: %s", url)
	}
	out, err := s.client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	if err != nil {
		return nil, fmt.Errorf("R2 GetObject 失败: %w", err)
	}
	return out.Body, nil
}

// List 按 key 前缀列出 R2 中的文件，返回 https://<publicDomain>/<key> 形式的 URL 列表。
// 前缀为空时列出 bucket 全部文件；前缀按 key 字符前缀匹配（如 "images/forum" 匹配 "images/forum/xxx.webp"）。
func (s *R2Storage) List(ctx context.Context, prefix string) ([]string, error) {
	prefix = strings.Trim(prefix, "/")
	input := &s3.ListObjectsV2Input{
		Bucket: aws.String(s.bucket),
	}
	if prefix != "" {
		input.Prefix = aws.String(prefix + "/")
	}
	var urls []string
	paginator := s3.NewListObjectsV2Paginator(s.client, input)
	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("R2 ListObjectsV2 失败: %w", err)
		}
		for _, obj := range page.Contents {
			key := aws.ToString(obj.Key)
			if key == "" {
				continue
			}
			urls = append(urls, s.publicDomain+"/"+key)
		}
	}
	return urls, nil
}
