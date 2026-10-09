package utils

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
)

// ToString 将一个任意的值转换为一个字符串
func ToString(v any) string {
	switch vv := v.(type) {
	case string:
		return vv
	case []byte:
		return string(vv)
	case fmt.Stringer:
		return vv.String()
	}

	h := md5.New()
	h.Write([]byte(fmt.Sprintf("%v", v)))
	return hex.EncodeToString(h.Sum(nil))
}
