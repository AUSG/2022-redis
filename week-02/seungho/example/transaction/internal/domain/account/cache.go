package account

import "fmt"

func CacheHashRootKey(holderID string) string {
	return "app:holder:" + holderID
}

func CacheHolderField() string {
	return "holder"
}

func CacheHashAccountField(accountType Type) string {
	return fmt.Sprintf("account:%s", accountType)
}
