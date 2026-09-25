package bugs

import (
	"context"
)

type Db interface {
	GetUsername(ctx context.Context, userId int64) (username string, isNotFoundErr bool, err error)
}
