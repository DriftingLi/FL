// 用户唯一标识（uid）生成：应用层雪花算法（bwmarrin/snowflake）。
// uid 为 64 位雪花 ID，DB 列 BIGINT UNIQUE NOT NULL，API 层以字符串序列化
// （超出 JS Number.MAX_SAFE_INTEGER，前端必须按字符串处理）。
package service

import (
	"strconv"
	"sync"

	"github.com/bwmarrin/snowflake"
)

var (
	uidNodeOnce sync.Once
	uidNode     *snowflake.Node
)

// initUIDNode 惰性初始化雪花节点（worker ID 固定为 1，单实例部署足够）。
func initUIDNode() {
	uidNodeOnce.Do(func() {
		node, err := snowflake.NewNode(1)
		if err != nil {
			panic("snowflake init failed: " + err.Error())
		}
		uidNode = node
	})
}

// NextUID 生成下一个用户 uid。
func NextUID() int64 {
	initUIDNode()
	return uidNode.Generate().Int64()
}

// FormatUID 将 uid 序列化为字符串（雪花 19 位超出 JS 安全整数，API 一律字符串输出）。
func FormatUID(uid int64) string {
	return strconv.FormatInt(uid, 10)
}
