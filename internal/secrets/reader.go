package secrets

import (
	"encoding/xml"
	"errors"

	"github.com/seanthegeek/doubleagent/internal/jsonvalue"
	"github.com/seanthegeek/doubleagent/internal/tomldoc"
)

// isReaderError reports whether err is a JSON, TOML or XML reader's error,
// whose message names a position and then ": " and the reader's detail.
func isReaderError(err error) bool {
	var jsonError *jsonvalue.Error
	var tomlError *tomldoc.Error
	var xmlError *xml.SyntaxError
	return errors.As(err, &jsonError) || errors.As(err, &tomlError) || errors.As(err, &xmlError)
}

// isLimit reports whether err is a TOML document refused for a size or
// depth limit, which quotes nothing from the file.
func isLimit(err error) bool {
	var tomlError *tomldoc.Error
	return errors.As(err, &tomlError) && tomlError.IsLimit()
}
