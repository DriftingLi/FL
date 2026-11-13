// 本文件：R2Storage 的分片上传实现（#2494）——S3 原生 multipart 三段 + abort。
//
// 语义与本地模拟（multipart.go）对齐：uploadID 来自 CreateMultipartUpload、
// UploadPart 同序号重复上传幂等覆盖（S3 语义）、Complete 按 Part 序号升序组装、
// Abort 清理未完成会话（重复 abort 幂等——S3 对未知 uploadId 返回 NoSuchUpload，此处吞掉）。
package storage

import (
	"bytes"
	"context"
	"errors"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/aws/aws-sdk-go-v2/service/s3/types"
	smithy "github.com/aws/smithy-go"
)

// InitMultipart 开启 S3 multipart 会话，返回 UploadId。
func (s *R2Storage) InitMultipart(ctx context.Context, key, contentType string) (string, error) {
	out, err := s.client.CreateMultipartUpload(ctx, &s3.CreateMultipartUploadInput{
		Bucket:      aws.String(s.bucket),
		Key:         aws.String(key),
		ContentType: aws.String(contentType),
	})
	if err != nil {
		return "", fmt.Errorf("R2 CreateMultipartUpload 失败: %w", err)
	}
	return aws.ToString(out.UploadId), nil
}

// UploadPart 上传第 partNumber 片（1 起），返回存储侧 etag；同序号重复上传幂等覆盖。
func (s *R2Storage) UploadPart(ctx context.Context, key, uploadID string, partNumber int, content []byte) (string, error) {
	out, err := s.client.UploadPart(ctx, &s3.UploadPartInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int32(int32(partNumber)),
		Body:       bytes.NewReader(content),
	})
	if err != nil {
		return "", fmt.Errorf("R2 UploadPart 失败: %w", err)
	}
	return aws.ToString(out.ETag), nil
}

// CompleteMultipart 按 Part 升序组装对象，返回 https://<publicDomain>/<key> 形式 URL。
func (s *R2Storage) CompleteMultipart(ctx context.Context, key, uploadID string, parts []Part) (string, error) {
	if len(parts) == 0 {
		return "", fmt.Errorf("分片清单为空")
	}
	completed := make([]types.CompletedPart, 0, len(parts))
	for _, p := range parts {
		completed = append(completed, types.CompletedPart{
			ETag:       aws.String(p.ETag),
			PartNumber: aws.Int32(int32(p.Number)),
		})
	}
	_, err := s.client.CompleteMultipartUpload(ctx, &s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &types.CompletedMultipartUpload{
			Parts: completed,
		},
	})
	if err != nil {
		return "", fmt.Errorf("R2 CompleteMultipartUpload 失败: %w", err)
	}
	return s.publicDomain + "/" + key, nil
}

// AbortMultipart 中止会话；会话已不存在（NoSuchUpload）时幂等返回 nil。
func (s *R2Storage) AbortMultipart(ctx context.Context, key, uploadID string) error {
	_, err := s.client.AbortMultipartUpload(ctx, &s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	if err != nil {
		var nsk *types.NoSuchUpload
		if errors.As(err, &nsk) {
			return nil
		}
		var apiErr smithy.APIError
		if errors.As(err, &apiErr) && apiErr.ErrorCode() == "NoSuchUpload" {
			return nil
		}
		return fmt.Errorf("R2 AbortMultipartUpload 失败: %w", err)
	}
	return nil
}
