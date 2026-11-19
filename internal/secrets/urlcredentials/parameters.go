package urlcredentials

// credentialParameter is a query or fragment parameter whose value is a
// credential, by name, and where that is documented.
type credentialParameter struct {
	name, source string
}

// credentialParameters returns the parameters whose values are secrets,
// each name compared without regard to ASCII case. A name several sources
// document is listed once, with each source.
func credentialParameters() []credentialParameter {
	oauth := "RFC 6749 (OAuth 2.0) and RFC 7636: the token, code and verifier parameters"
	return []credentialParameter{
		{"token", "OAuth token revocation and introspection (RFC 7009, RFC 7662); Okta"},
		{"access_token", "RFC 6750 section 2.3 (a bearer token in the query); Google OAuth; GitHub OAuth; " +
			"Okta; Microsoft identity platform"},
		{"refresh_token", oauth + "; GitHub OAuth; Microsoft identity platform"},
		{"id_token", "OpenID Connect Core 1.0; Okta; Microsoft identity platform"},
		{"key", "Google API keys: the key query parameter"},
		{"api_key", "API keys passed in a query string, as the agents' research documents record"},
		{"apikey", "API keys passed in a query string, as the agents' research documents record"},
		{"api-key", "API keys passed in a query string, as the agents' research documents record"},
		{"secret", "a shared secret passed in a query string, as the agents' exports redact it"},
		{"client_secret", "RFC 6749 section 2.3.1 (the client password); GitHub OAuth; Microsoft identity " +
			"platform, authorization code flow"},
		{"client_assertion", "Microsoft identity platform, authorization code flow: a signed JWT that " +
			"authenticates the client"},
		{"password", "a password passed in a query string, as the agents' exports redact it"},
		{"auth", "an authorization value passed in a query string, as the agents' exports redact it"},
		{"authorization", "an Authorization header passed in a query string, as the agents' exports redact it"},
		{"sig", "Azure Storage, Create a service SAS: the signature field"},
		{"Signature", "a request signature in a query string; AWS Signature Version 2; Amazon CloudFront " +
			"signed URLs"},
		{"private_token", "GitLab REST API authentication: the private_token parameter"},
		{"code", oauth + "; GitHub OAuth; Okta; Microsoft identity platform"},
		{"code_verifier", oauth + "; GitHub OAuth; Microsoft identity platform"},
		{"device_code", "RFC 8628 (OAuth device authorization); GitHub OAuth device flow"},
		{"X-Amz-Signature", "AWS IAM user guide, Authentication methods: query string parameters"},
		{"X-Amz-Security-Token", "AWS IAM user guide, Authentication methods: query string parameters; " +
			"Use temporary credentials with AWS resources: the session token in a query string parameter"},
		{"X-Amz-Credential", "AWS IAM user guide, Authentication methods: query string parameters"},
		{"AWSAccessKeyId", "AWS Signature Version 2"},
		{"X-Goog-Signature", "Google Cloud Storage signed URLs"},
		{"X-Goog-Credential", "Google Cloud Storage signed URLs"},
		{"GoogleAccessId", "Google Cloud Storage signed URLs (V2 signing)"},
		{"sessionToken", "Okta authentication API"},
		{"stateToken", "Okta authentication API"},
		{"interclient_token", "Okta"},
	}
}
