package storage

import (
	"context"
	"example/transaction/internal/domain/account"
	"fmt"
	"github.com/go-redis/redis/v8"
)

type Account struct {
	rds *redis.Client
}

func NewAccount(rds *redis.Client) Account {
	return Account{rds: rds}
}

func (a Account) Create(ctx context.Context, holder account.Holder) error {
	accounts := holder.Accounts
	holder.Accounts = nil

	err := a.rds.Watch(ctx, func(tx *redis.Tx) error {

		_, err := tx.TxPipelined(ctx, func(pipe redis.Pipeliner) error {
			if _, err := pipe.HSetNX(
				ctx,
				account.CacheHashRootKey(holder.ID),
				account.CacheHolderField(),
				&holder,
			).Result(); err != nil {
				return fmt.Errorf("create: holder: %w", err)
			}

			for _, acc := range accounts {
				if _, err := pipe.HSetNX(
					ctx,
					account.CacheHashRootKey(holder.ID),
					account.CacheHashAccountField(acc.Type),
					acc,
				).Result(); err != nil {
					return fmt.Errorf("create: account: %w", err)
				}
			}
			return nil
		})
		return err
	})
	if err != nil {
		return fmt.Errorf("create: transaction: %w", err)
	}
	return nil
}
