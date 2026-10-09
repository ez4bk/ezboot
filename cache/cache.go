package cache

import (
	"crypto/md5"
	"encoding"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/go-redis/redis"
)

// Cacheable 表示一个可被缓存的值的约束
type Cacheable interface {
	encoding.BinaryMarshaler
	encoding.BinaryUnmarshaler
}

// Cache 是一个缓存组件, 用于加速接口的响应速度
type Cache[T Cacheable] interface {
	// Get 从缓存中根据标识符获取一个值
	Get(identifier any, value T) error

	// Put 将一个值放入到缓存系统中
	Put(identifier any, value T, expiration time.Duration) error

	// Expired 修改一个缓存对象的过期时间
	Expired(identifier any, expired time.Duration) error

	// Delete 删除一个缓存对象
	Delete(identifier any) error

	// Exist 检测一个缓存项是否存在
	Exist(identifier any) (bool, error)
}

// Manager 缓存管理器, 用于返回一个管理器用于管理缓存
type Manager[T Cacheable] struct {
	rdb *redis.Client
}

// Get 从缓存中根据标识符获取一个值
func (m *Manager[T]) Get(identifier any, value T) error {
	data, err := m.rdb.Get(toString(identifier)).Bytes()
	if err != nil {
		return err
	}
	return value.UnmarshalBinary(data)
}

// Put 将一个值放入到缓存系统中
func (m *Manager[T]) Put(identifier any, value T, expiration time.Duration) error {
	return m.rdb.Set(toString(identifier), value, expiration).Err()
}

// Expired 修改一个缓存对象的过期时间
func (m *Manager[T]) Expired(identifier any, expired time.Duration) error {
	return m.rdb.Expire(toString(identifier), expired).Err()
}

// Delete 删除一个缓存对象
func (m *Manager[T]) Delete(identifier any) error {
	return m.rdb.Del(toString(identifier)).Err()
}

// Exist 检测一个缓存项是否存在
func (m *Manager[T]) Exist(identifier any) (bool, error) {
	n, err := m.rdb.Exists(toString(identifier)).Result()
	if n > 0 {
		return true, err
	}
	return false, err
}

// WithPrefix 为缓存增加统一的前缀
func (m *Manager[T]) WithPrefix(prefix string) Cache[T] {
	return &withPrefixManager[T]{Manager: m, prefix: strings.TrimRight(prefix, ":") + ":"}
}

// withPrefixManager 带前缀的缓存管理器
type withPrefixManager[T Cacheable] struct {
	*Manager[T]

	prefix string
}

// Get 从缓存中根据标识符获取一个值
func (m *withPrefixManager[T]) Get(identifier any, value T) error {
	return m.Manager.Get(m.prefix+toString(identifier), value)
}

// Put 将一个值放入到缓存系统中
func (m *withPrefixManager[T]) Put(identifier any, value T, expired time.Duration) error {
	return m.Manager.Put(m.prefix+toString(identifier), value, expired)
}

// Expired 修改一个缓存对象的过期时间
func (m *withPrefixManager[T]) Expired(identifier any, expiration time.Duration) error {
	return m.Manager.Expired(m.prefix+toString(identifier), expiration)
}

// Delete 删除一个缓存对象
func (m *withPrefixManager[T]) Delete(identifier any) error {
	return m.Manager.Delete(m.prefix + toString(identifier))
}

// Exist 检测一个缓存项是否存在
func (m *withPrefixManager[T]) Exist(identifier any) (bool, error) {
	return m.Manager.Exist(m.prefix + toString(identifier))
}

// New 创建一个缓存管理器
func New[T Cacheable](rdb *redis.Client) (*Manager[T], error) {
	return &Manager[T]{rdb: rdb}, nil
}

// NewWithPrefix 创建一个缓存管理器并携带前缀
func NewWithPrefix[T Cacheable](rdb *redis.Client, prefix string) (Cache[T], error) {
	cache, err := New[T](rdb)
	if err != nil {
		return nil, err
	}
	return cache.WithPrefix(prefix), nil
}

// toString 将一个任意的值转换为一个字符串
func toString(v any) string {
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
