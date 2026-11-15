package application

import (
	"context"
	"contractors-platform/backend/internal/modules/onboarding/domain"
	"errors"
	"testing"
)

type memoryUsers struct{ users map[string]domain.User }

func (repository *memoryUsers) Save(user domain.User) (domain.User, error) {
	user.ID = "vendor-1"
	repository.users[user.ID] = user
	return user, nil
}

func (repository *memoryUsers) Find(id string) (domain.User, bool) {
	user, found := repository.users[id]
	return user, found
}

func (repository *memoryUsers) FindByCLABE(clabe string) (domain.User, bool) {
	for _, user := range repository.users {
		if user.CLABE == clabe {
			return user, true
		}
	}
	return domain.User{}, false
}

func (repository *memoryUsers) Update(user domain.User) (domain.User, error) {
	repository.users[user.ID] = user
	return user, nil
}

func (repository *memoryUsers) HasCLABE(string) (bool, error) { return false, nil }

func (repository *memoryUsers) HasOtherCLABE(string, string) (bool, error) { return false, nil }

func (repository *memoryUsers) RecordBankAccount(user domain.User) (domain.User, error) {
	return user, nil
}

func (repository *memoryUsers) Remove(id string) error {
	delete(repository.users, id)
	return nil
}

type failingClients struct{}

func (failingClients) SaveFirst(string, domain.Customer) error {
	return errors.New("clients down")
}

func (failingClients) First(string) (domain.Customer, bool, error) {
	return domain.Customer{}, false, nil
}

type countingBanks struct{ calls int }

func (banks *countingBanks) Register(context.Context, []byte) ([]byte, error) {
	banks.calls++
	return []byte(`{"id":"01BANK","codiStatus":"PENDING"}`), nil
}

func TestClientStoreFailureRemovesTheVendor(t *testing.T) {
	repository := &memoryUsers{users: map[string]domain.User{}}
	banks := &countingBanks{}
	user, fieldErrors := domain.Build(domain.Input{
		Name: "Ana", Profession: "Electrician",
		Client:       domain.Client{Name: "Luis", Phone: "5551234567", Email: "luis@example.com"},
		BusinessName: "Electrical Services", BusinessActivity: "Electrical installations", RFC: "AAA010101AAA", CLABE: "002010077777777771",
	})
	if len(fieldErrors) > 0 {
		t.Fatal(fieldErrors)
	}
	_, _, err := NewRegisterUser(repository, banks, nil, failingClients{}).Create(context.Background(), domain.Input{
		Name: user.Name, Profession: user.Profession, Client: domain.Client{Name: user.Customer.Name, Phone: user.Customer.Phone, Email: user.Customer.Email},
		BusinessName: user.BusinessName, BusinessActivity: user.BusinessActivity, RFC: user.RFC, CLABE: user.CLABE,
	})
	if err == nil || banks.calls != 0 {
		t.Fatalf("err=%v calls=%d", err, banks.calls)
	}
	if _, found := repository.Find("vendor-1"); found {
		t.Fatal("vendor remained after the client store failed")
	}
}
