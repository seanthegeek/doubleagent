package secrets

import (
	"slices"

	"github.com/seanthegeek/doubleagent/internal/secrets/urlcredentials"
	"github.com/seanthegeek/doubleagent/internal/textset"
)

// identifyURLs calls found with the credentials of the URLs in text that
// the output replaces: every one urlcredentials finds but user information
// without a colon shorter than EverywhereLength bytes, which is a user name
// (git@, ci@) more often than a token.
func identifyURLs(text string, found func(urlcredentials.Credential), work textset.Counter) {
	if !urlcredentials.MayHold(text) {
		return
	}
	urlcredentials.Find(text, func(secret urlcredentials.Credential) {
		if !secret.NameOnly || secret.End-secret.Start >= EverywhereLength {
			found(secret)
		}
	}, work)
}

// URLCredentials returns the credentials the output finds in the URLs of
// text, each secret once in the order found, for a test that checks the
// output against them.
func URLCredentials(text string) []string {
	var found []string
	identifyURLs(text, func(secret urlcredentials.Credential) {
		if value := text[secret.Start:secret.End]; !slices.Contains(found, value) {
			found = append(found, value)
		}
	}, nil)
	return found
}

// URLCredentialRanges returns the byte ranges, [start, end), that the
// output replaces in text for the credentials of its URLs, in the order
// found, for a test that checks the output against them; with an empty
// placeholder the range of user information takes its @.
func URLCredentialRanges(text string, emptyPlaceholder bool) [][2]int {
	var ranges [][2]int
	identifyURLs(text, func(secret urlcredentials.Credential) {
		ranges = append(ranges, [2]int{secret.Start, secret.ReplacedEnd(emptyPlaceholder)})
	}, nil)
	return ranges
}
