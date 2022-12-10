package repository

import (
	"context"
	"example/transaction/internal/domain/account"
)

type Account interface {
	Create(ctx context.Context, holder account.Holder) error
}
