package snowflake

import (
	"context"
)

// IDGen ID生成器
type IDGen interface {
	GenID(ctx context.Context) (ID, error)
}
