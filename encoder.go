// SPDX-FileCopyrightText: 2022 Weston Schmidt <weston_schmidt@alumni.purdue.edu>
// SPDX-License-Identifier: Apache-2.0

// yaml provides a way to encode both the simple form and the detailed form
// of configuration data for the goschtalt library.
package yaml

import (
	"errors"
	"sort"

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

	return yml.Marshal(&doc)
}

// encoderWrapper handles the fact that the yaml decoder may panic instead of
// returning an error.
func encoderWrapper(n *yml.Node, v any) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = ErrEncoding
		}
	}()

	return n.Encode(v)
}

// encode is an internal helper function that builds the yml.Node based tree
// to give to the yaml encoder.  This is likely specific to this yaml encoder.
func encode(obj meta.Object) (n yml.Node, err error) {
	n.LineComment = obj.OriginString()
	kind := obj.Kind()

	if kind == meta.Value {
		err = encoderWrapper(&n, obj.Value)

		if err != nil {
			return yml.Node{}, err
		}
		n.LineComment = obj.OriginString() // The encode wipes this out.
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
			LineComment: v.OriginString(),
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
