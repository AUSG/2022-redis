package storage

import (
	"context"
	"example/transaction/internal/domain/account"
	"github.com/go-redis/redis/v8"
	"github.com/stretchr/testify/assert"
	"testing"
)

func TestAccount_Create(t *testing.T) {
	rds := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})

	storage := NewAccount(rds)

	holder := account.Holder{
		ID:         "144c8dcb89dc293f55c68cc74adda88b",
		FirstName:  "Al",
		MiddleName: "",
		LastName:   "Pacino",
		Accounts: []*account.Account{
			{
				Type:     account.TypeCurrent,
				Number:   "10101010",
				SortCode: "10-10-10",
				Active:   true,
			},
			{
				Type:     account.TypeSavings,
				Number:   "20202020",
				SortCode: "20-20-20",
				Active:   false,
			},
		},
	}

	assert.NoError(t, storage.Create(context.Background(), holder))
}
