package geolocation

import "crypto/md5"

// md5Hex 供内嵌数据指纹校验用（人工替换 xdb 时同步更新 DataSourceMD5）。
func md5Hex(b []byte) string {
	sum := md5.Sum(b)
	const hexDigits = "0123456789abcdef"
	out := make([]byte, 0, len(sum)*2)
	for _, c := range sum {
		out = append(out, hexDigits[c>>4], hexDigits[c&0x0f])
	}
	return string(out)
}
