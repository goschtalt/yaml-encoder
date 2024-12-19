// SPDX-FileCopyrightText: 2022 Weston Schmidt <weston_schmidt@alumni.purdue.edu>
// SPDX-License-Identifier: Apache-2.0

// yamlencoder provides a way to encode both the simple form and the detailed form
// of configuration data for the goschtalt library.
//
// # Detailed Output
//
// The details about where the configuration value originated are included as
// a list of `file:line[col]` values in a comment if goschtalt knows the origin.
// Not all decoders support tracking all this information.  Comment will always
// be present so it's easier to handle the file using simple cli text processors.
//
// Example
//
//	candy: bar                      # file.yml:1[8]
//	cats:                           # file.yml:2[1]
//	    - madd                      # file.yml:3[7]
//	    - tabby                     # file.yml:4[7]
//	other:                          # file.yml:5[1]
//	    things:                     # file.yml:6[5]
//	        green:                  # file.yml:8[9]
//	            - grass             # unknown
//	            - ground            # file.yml:10[15]
//	        red: balloons           # file.yml:7[14]
//	    trending: now               # file.yml:12[15]
package yamlencoder

import (
	"bufio"
	"bytes"
	"encoding/base32"
	"errors"
	"sort"
	"strings"
	"unicode"

	"github.com/goschtalt/goschtalt"
	"github.com/goschtalt/goschtalt/pkg/encoder"
	"github.com/goschtalt/goschtalt/pkg/meta"
	yml "gopkg.in/yaml.v3"
)

var (
	ErrEncoding = errors.New("encoding error")
)

// Ensure interface compliance.
var _ encoder.Encoder = (*Encoder)(nil)

// Use init to automatically wire this encoder as one available for goschtalt
// simply by including this package.
func init() {
	var e Encoder
	goschtalt.DefaultOptions = append(goschtalt.DefaultOptions, goschtalt.WithEncoder(e))
}

// Encoder is a class for the yaml encoder.
type Encoder struct{}

// Extensions returns the supported extensions.
func (e Encoder) Extensions() []string {
	return []string{"yaml", "yml"}
}

// Encode encodes the value provided into yaml and returns the bytes.
func (e Encoder) Encode(a any) ([]byte, error) {
	return yml.Marshal(a)
}

// Encode encodes the meta.Object provided into yaml with comments showing the
// origin of the configuration and returns the bytes.
func (e Encoder) EncodeExtended(obj meta.Object) ([]byte, error) {
	if len(obj.Map) == 0 {
		return []byte("null\n"), nil
	}

	doc := yml.Node{
		Kind: yml.DocumentNode,
		Tag:  "!!map",
	}

	n, err := encode(obj)
	if err != nil {
		return nil, err
	}
	doc.Content = append(doc.Content, &n)

	b, err := yml.Marshal(&doc)
	if err != nil {
		return nil, err
	}

	return alignComments(b)
}

// determineStyle determines the best YAML style (|- or quoted) for a given string.
func determineStyle(input string) yml.Style {
	// Check flags to decide whether we need to quote the string
	needsQuotes := false
	containsNewlines := false

	for idx, ch := range input {
		switch {
		case ch == '\n':
			// Newlines are fine in a block scalar
			containsNewlines = true
		case ch < 0x20 && ch != '\t': // Non-printable ASCII except tab
			needsQuotes = true
		case (ch == ':' || ch == '-') && idx == 0:
			// Leading `:` or `-` must be quoted
			needsQuotes = true
		case ch == '\\':
			// Backslash must be quoted to preserve literal value
			needsQuotes = true
		case ch == '"':
			// Double quotes must be escaped if quoted
			needsQuotes = true
		case ch > 0x7F:
			// Unicode characters above ASCII 127
			needsQuotes = true
		}
	}

	// If the string contains newlines and doesn't need quotes, use |-
	if containsNewlines && !needsQuotes {
		// return yml.LiteralStyle <-- This is ideal, but there is a bug
		// in the yaml encoder that causes it to encode the output wrong that
		// I can't figure out how to work around.  So we'll use the next best
		// thing.
		return yml.DoubleQuotedStyle
	}

	// If the string needs quotes or is empty or ends with a space, use ""
	if needsQuotes || len(input) == 0 || unicode.IsSpace(rune(input[len(input)-1])) {
		return yml.DoubleQuotedStyle
	}

	// Default to plain style
	return yml.TaggedStyle
}

// encoderWrapper handles the fact that the yaml decoder may panic instead of
// returning an error.
func encoderWrapper(n *yml.Node, v any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrEncoding
		}
	}()

	// This is to work around a bug in the yaml encoder where encodes the output
	// wrong if the string contains a newline.
	if s, ok := v.(string); ok {
		n.Style = determineStyle(s)
		n.Kind = yml.ScalarNode
		n.Value = s
		return nil
	}

	return n.Encode(v)
}

// encode is an internal helper function that builds the yml.Node based tree
// to give to the yaml encoder.  This is likely specific to this yaml encoder.
// Also always be sure to include a comment on each line so the alignment process
// in alignComments() is simpler logic.
func encode(obj meta.Object) (n yml.Node, err error) {
	n.LineComment = encodeComment(obj.OriginString())
	kind := obj.Kind()

	if kind == meta.Value {
		err = encoderWrapper(&n, obj.Value)

		if err != nil {
			return yml.Node{}, err
		}
		n.LineComment = encodeComment(obj.OriginString()) // The encode wipes this out.
		return n, nil
	}

	if kind == meta.Array {
		n.Kind = yml.SequenceNode

		for _, v := range obj.Array {
			sub, err := encode(v)
			if err != nil {
				return yml.Node{}, err
			}
			n.Content = append(n.Content, &sub)
		}

		return n, nil
	}

	n.Kind = yml.MappingNode

	// Sort the keys so the output order is predictable, making testing easier.
	keys := make([]string, 0, len(obj.Map))
	for key := range obj.Map {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	for _, k := range keys {
		v := obj.Map[k]
		key := yml.Node{
			Kind:        yml.ScalarNode,
			LineComment: encodeComment(v.OriginString()),
			Value:       k,
		}
		val, err := encode(v)
		if err != nil {
			return yml.Node{}, err
		}

		n.Content = append(n.Content, &key)
		n.Content = append(n.Content, &val)
	}

	return n, nil
}

// encodeComment base32 encodes the comment so the processing needed to align
// the comments is easier.  We can simply look for the right-most # because of
// the encoding excluding # from the character set.
func encodeComment(s string) string {
	if len(s) == 0 {
		s = "unknown"
	}
	return base32.StdEncoding.EncodeToString([]byte(s))
}

// decodeComment is the reverse of encodeComment(), but handles the case of if
// decoding fails.  It should never fail, but it checks for it anyway.
func decodeComment(s string) (string, error) {
	buf, err := base32.StdEncoding.DecodeString(s)
	if err != nil {
		return "", err
	}

	return string(buf), nil
}

// alignComments finds the longest line, adds 8 spaces, then aligns the comments
// to the next tabstop (assuming tabwidth is 4).  This is also where the comments
// are decoded from base32.
func alignComments(buf []byte) ([]byte, error) {
	// Assume each line is about 24 bytes long as a starting buffer size.
	// A smaller line size guess reduces the re-allocations needed later.
	lines := make([]string, 0, len(buf)/24)
	scanner := bufio.NewScanner(bytes.NewReader(buf))

	var widest int

	for scanner.Scan() {
		line := scanner.Text()
		if found := strings.LastIndex(line, "# "); found > widest {
			widest = found
		}

		lines = append(lines, line)
	}

	widest += 8 + (widest % 4)

	var b strings.Builder
	for _, line := range lines {
		if found := strings.LastIndex(line, "# "); found > 0 {
			left := line[:found]
			right := line[found:]
			comment, err := decodeComment(right[2:])
			if err != nil {
				// This  isn't really possible unless the encoder below
				// changes.  This seems better than either a silent failure
				// or a panic.
				return nil, err
			}

			b.WriteString(left)
			for found < widest {
				b.WriteString(" ")
				found++
			}
			b.WriteString("# ")
			b.WriteString(comment)
			b.WriteString("\n")
		}
	}

	return []byte(b.String()), nil
}
