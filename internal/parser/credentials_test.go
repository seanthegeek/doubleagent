package parser

import "testing"

func TestWithoutCredentialsRedactsTheUserInformationOfAURL(t *testing.T) {
	tests := []struct {
		name, text, want string
	}{
		{"a user and password", "https://alice:s3cret@host.example:8443/api?x=1",
			"https://redacted@host.example:8443/api?x=1"},
		{"a token as the user name", "https://ghp_TOKEN@github.com/org/repo.git",
			"https://redacted@github.com/org/repo.git"},
		{"a password holding an at sign", "http://u:p@ss@host/", "http://redacted@host/"},
		{"a URL net/url refuses", "https://tok:se cret@host/", "https://redacted@host/"},
		{"an at sign in the path is kept", "https://host.example/users/@alice", "https://host.example/users/@alice"},
		{"an at sign in the query is kept", "https://host.example?mail=a@b", "https://host.example?mail=a@b"},
		{"a URL without credentials is unchanged", "http://localhost:8080", "http://localhost:8080"},
		{"text that is not a URL is unchanged", "alice@host", "alice@host"},
		{"empty text is unchanged", "", ""},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := WithoutCredentials(test.text); got != test.want {
				t.Errorf("got %q, want %q", got, test.want)
			}
		})
	}
}
