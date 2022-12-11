package account

import "encoding/json"

type Holder struct {
	ID         string
	FirstName  string
	MiddleName string
	LastName   string
	Accounts   []*Account
}

func (h *Holder) MarshalBinary() ([]byte, error) {
	return json.Marshal(h)
}

func (h *Holder) UnmarshalBinary(data []byte) error {
	if err := json.Unmarshal(data, &h); err != nil {
		return err
	}

	return nil
}
