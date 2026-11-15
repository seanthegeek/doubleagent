//go:build parity

package parity

import (
	"archive/tar"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

// hashText returns the hex SHA-256 of text.
func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// readArchive reads the gzip-compressed tar at path and returns a description
// of each member keyed by its name: the content's SHA-256 for a regular file,
// the link target for a link, the member type for the rest. The manifest
// member is returned as its text, which the comparison normalises first.
func readArchive(path string) (map[string]string, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	unzipped, err := gzip.NewReader(file)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	reader := tar.NewReader(unzipped)
	members := map[string]string{}
	for {
		header, err := reader.Next()
		if errors.Is(err, io.EOF) {
			return members, nil
		}
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		description, err := describeMember(header, reader)
		if err != nil {
			return nil, fmt.Errorf("read %s: %w", path, err)
		}
		members[header.Name] = description
	}
}

// describeMember returns the description of the member the reader is at.
func describeMember(header *tar.Header, reader io.Reader) (string, error) {
	switch header.Typeflag {
	case tar.TypeReg:
		content, err := io.ReadAll(reader)
		if err != nil {
			return "", err
		}
		if header.Name == "./manifest.jsonl" {
			return string(content), nil
		}
		return hashText(string(content)), nil
	case tar.TypeSymlink, tar.TypeLink:
		return fmt.Sprintf("link %q", header.Linkname), nil
	}
	return fmt.Sprintf("type %c", header.Typeflag), nil
}
