package credentials

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/DesKaOne/DesKaOne-Control-Center/internal/vault"
)

type Repository interface {
	PutCredential(context.Context, string, vault.Ciphertext) error
	Credential(context.Context, string) (vault.Ciphertext, error)
}
type Service struct {
	repo  Repository
	vault *vault.Vault
}

func New(repo Repository, masterKey []byte) (*Service, error) {
	v, err := vault.New(masterKey)
	if err != nil {
		return nil, err
	}
	return &Service{repo: repo, vault: v}, nil
}
func (s *Service) Put(ctx context.Context, provider string, values map[string]string) error {
	if provider == "" || len(values) == 0 {
		return errors.New("credential tidak lengkap")
	}
	raw, err := json.Marshal(values)
	if err != nil {
		return err
	}
	encrypted, err := s.vault.Encrypt(raw, []byte(provider))
	if err != nil {
		return err
	}
	return s.repo.PutCredential(ctx, provider, encrypted)
}
func (s *Service) Get(ctx context.Context, provider string) (map[string]string, error) {
	encrypted, err := s.repo.Credential(ctx, provider)
	if err != nil {
		return nil, err
	}
	raw, err := s.vault.Decrypt(encrypted, []byte(provider))
	if err != nil {
		return nil, err
	}
	var out map[string]string
	if err = json.Unmarshal(raw, &out); err != nil {
		return nil, errors.New("format credential tersimpan tidak valid")
	}
	return out, nil
}
