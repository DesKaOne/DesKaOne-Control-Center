package providers

import "strings"

type Account struct {
	ID               string `json:"id"`
	ProviderSlug     string `json:"provider"`
	AccountKey       string `json:"account_key"`
	DisplayName      string `json:"display_name"`
	Enabled          bool   `json:"enabled"`
	CredentialStored bool   `json:"credential_stored"`
}

var Allowed = map[string]bool{"proxyrack": true, "repocket": true, "traffmonetizer": true}

func ValidSlug(v string) bool { return Allowed[strings.ToLower(v)] }
func Clean(v string) string   { return strings.TrimSpace(v) }
func ValidAccountKey(v string) bool {
	if len(v) < 1 || len(v) > 63 {
		return false
	}
	for i, r := range v {
		if !((r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') || ((r == '-' || r == '_') && i > 0)) {
			return false
		}
	}
	return true
}
