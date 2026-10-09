package snowflake

import (
	"github.com/ez4bk/ezboot/sbs"
)

const (
	Zero    ID = 0
	Invalid ID = -1
)

// String returns a string of the snowflake ID
func (f ID) String() string {
	if f == 0 {
		return ""
	}
	return f.Base58()
}

// ParseString converts a string into a snowflake ID (override)
func ParseString(id string) (ID, error) {
	if id == "" {
		return Zero, nil
	}
	return ParseBase58(sbs.Bytes(id))
}

// ParsePointPString converts a string into a snowflake ID (override)
func ParsePointPString(id *string) (*ID, error) {
	if id == nil {
		return nil, nil
	}

	res, err := ParseBase58(sbs.Bytes(*id))
	return &res, err
}

// MustParse 强制把Base58字符串转换为ID
func MustParse(s string) ID {
	id, err := ParseString(s)
	if err != nil {
		panic(err)
	}
	return id
}

// MustPointParse 强制把Base58字符串转换为ID（兼容指针类型）
func MustPointParse(s *string) *ID {
	if s == nil {
		return nil
	}

	id, err := ParseString(*s)
	if err != nil {
		panic(err)
	}

	return &id
}

// SoftParse 把Base58字符串转换为ID, 如果存在错误则返回0
func SoftParse(s string) ID {
	id, err := ParseString(s)
	if err != nil {
		return Zero
	}
	return id
}
