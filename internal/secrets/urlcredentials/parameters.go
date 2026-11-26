package urlcredentials

// isCredentialParameter reports whether a lowered parameter name is one whose
// value is a secret. Each case names the documents that say so; the research
// documents record the rest.
func isCredentialParameter(name string) bool {
	switch name {
	case "token", // RFC 7009, RFC 7662; Okta
		"access_token",                 // RFC 6750 section 2.3; Google, GitHub, Okta, Microsoft
		"refresh_token",                // RFC 6749, RFC 7636; GitHub, Microsoft
		"id_token",                     // OpenID Connect Core 1.0; Okta, Microsoft
		"key",                          // Google API keys
		"api_key", "apikey", "api-key", // API keys in a query string
		"secret", "password", "auth", "authorization", // as the agents' exports redact them
		"client_secret",         // RFC 6749 section 2.3.1; GitHub, Microsoft
		"client_assertion",      // Microsoft identity platform: a signed JWT
		"sig",                   // Azure Storage service SAS signature
		"signature",             // AWS Signature Version 2; Amazon CloudFront signed URLs
		"private_token",         // GitLab REST API
		"code", "code_verifier", // RFC 6749, RFC 7636; GitHub, Okta, Microsoft
		"device_code",                                                                   // RFC 8628; GitHub device flow
		"x-amz-signature", "x-amz-security-token", "x-amz-credential", "awsaccesskeyid", // AWS
		"x-goog-signature", "x-goog-credential", "googleaccessid", // Google Cloud Storage signed URLs
		"sessiontoken", "statetoken", "interclient_token": // Okta
		return true
	}
	return false
}
