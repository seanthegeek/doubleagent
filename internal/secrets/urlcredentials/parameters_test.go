package urlcredentials

import "testing"

func TestIsCredentialNameTakesEveryListedName(t *testing.T) {
	listed := []string{"token", "access_token", "refresh_token", "id_token", "key", "api_key",
		"apikey", "api-key", "secret", "client_secret", "client_assertion", "password", "auth",
		"authorization", "sig", "Signature", "private_token", "code", "code_verifier",
		"device_code", "X-Amz-Signature", "X-Amz-Security-Token", "X-Amz-Credential",
		"AWSAccessKeyId", "X-Goog-Signature", "X-Goog-Credential", "GoogleAccessId",
		"sessionToken", "stateToken", "interclient_token"}
	for _, name := range listed {
		if !IsCredentialName(name) {
			t.Errorf("IsCredentialName(%q) = false, want true", name)
		}
	}
	for _, name := range []string{"TOKEN", "To%6Ben", "%41pi_Key"} {
		if !IsCredentialName(name) {
			t.Errorf("IsCredentialName(%q) = false, want true", name)
		}
	}
}

func TestIsCredentialNameRefusesOtherNames(t *testing.T) {
	for _, name := range []string{"", "tokens", "toke", "keys", "Key", "to ken", "%zz", "state"} {
		if IsCredentialName(name) {
			t.Errorf("IsCredentialName(%q) = true, want false", name)
		}
	}
}
