package store

import (
	"time"
)

// mustParseTime 解析 RFC3339 时间；解析失败返回零值（正常路径不会发生）。
func mustParseTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return time.Time{}
	}
	return t
}

// boolInt 将 bool 转为 SQLite 整数。
func boolInt(b bool) int {
	if b {
		return 1
	}
	return 0
}
