package account

import "encoding/json"

type Type string

const (
	TypeCurrent Type = "Current"
	TypeSavings Type = "Savings"
)

type Account struct {
	Type     Type
	Number   string
	SortCode string
	Active   bool
}

func (a *Account) MarshalBinary() ([]byte, error) {
	return json.Marshal(a)
}

func (a *Account) UnmarshalBinary(data []byte) error {
	if err := json.Unmarshal(data, &a); err != nil {
		return err
	}

	return nil
}
