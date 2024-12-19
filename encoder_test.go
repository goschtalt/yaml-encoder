// SPDX-FileCopyrightText: 2022 Weston Schmidt <weston_schmidt@alumni.purdue.edu>
// SPDX-License-Identifier: Apache-2.0

package yamlencoder

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/goschtalt/goschtalt/pkg/meta"
	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v3"
)

func TestExtensions(t *testing.T) {
	assert := assert.New(t)

	var e Encoder
	got := e.Extensions()

	assert.Empty(cmp.Diff([]string{"yaml", "yml"}, got))
}

func TestEncodeExtended(t *testing.T) {
	tests := []struct {
		description      string
		in               meta.Object
		expected         string
		expectedExtended string
		expectedErr      error
	}{
		{
			description:      "A test of empty.",
			expected:         "null\n",
			expectedExtended: "null\n",
		},
		{
			description: "A simple test.",
			// Input vector in yaml:
			//candy: bar
			//cats:
			//    - madd
			//    - tabby
			//other:
			//    things:
			//        red: balloons
			//        green:
			//            - grass
			//            - ground
			//            - "water/nballoons\""
			//    trending: now
			in: meta.Object{
				Origins: []meta.Origin{{File: "file.yml", Line: 1, Col: 1}},
				Map: map[string]meta.Object{
					"candy": {
						Origins: []meta.Origin{{File: "file.yml", Line: 1, Col: 8}},
						Value:   "bar",
					},
					"cats": {
						Origins: []meta.Origin{{File: "file.yml", Line: 2, Col: 1}},
						Array: []meta.Object{
							{
								Origins: []meta.Origin{{File: "file.yml", Line: 3, Col: 7}},
								Value:   "madd",
							},
							{
								Origins: []meta.Origin{{File: "file.yml", Line: 4, Col: 7}},
								Value:   "tabby",
							},
						},
					},
					"other": {
						Origins: []meta.Origin{{File: "file.yml", Line: 5, Col: 1}},
						Map: map[string]meta.Object{
							"things": {
								Origins: []meta.Origin{{File: "file.yml", Line: 6, Col: 5}},
								Map: map[string]meta.Object{
									"red": {
										Origins: []meta.Origin{{File: "file.yml", Line: 7, Col: 14}},
										Value:   "balloons",
									},
									"green": {
										Origins: []meta.Origin{{File: "file.yml", Line: 8, Col: 9}},
										Array: []meta.Object{
											{
												// Leave the origin off here to show what happens if none
												// is present.
												Value: "grass",
											},
											{
												Origins: []meta.Origin{{File: "file.yml", Line: 10, Col: 15}},
												Value:   "ground\nout",
											},
											{
												Origins: []meta.Origin{{File: "file.yml", Line: 11, Col: 15}},
												Value:   "water\nballoons\"",
											},
										},
									},
								},
							},
							"trending": {
								Origins: []meta.Origin{{File: "file.yml", Line: 12, Col: 15}},
								Value:   "now",
							},
						},
					},
				},
			},
			expected: `candy: bar
cats:
    - madd
    - tabby
other:
    things:
        green:
            - grass
            - |-
              ground
              out
            - |-
              water
              balloons"
        red: balloons
    trending: now
`,
			expectedExtended: `candy: bar                                  # file.yml:1[8]
cats:                                       # file.yml:2[1]
    - madd                                  # file.yml:3[7]
    - tabby                                 # file.yml:4[7]
other:                                      # file.yml:5[1]
    things:                                 # file.yml:6[5]
        green:                              # file.yml:8[9]
            - grass                         # unknown
            - "ground\nout"                 # file.yml:10[15]
            - "water\nballoons\""           # file.yml:11[15]
        red: balloons                       # file.yml:7[14]
    trending: now                           # file.yml:12[15]
`,
		},
		{
			description: "try to encode a channel (invalid) for verifying the failure path",
			// Input vector in yaml:
			//candy: bar
			//cats:
			//    - madd
			//    - tabby
			//other:
			//    things:
			//        red: balloons
			//        green:
			//            - grass
			//            - ground
			//            - <invalid channel>
			//    trending: now
			in: meta.Object{
				Origins: []meta.Origin{{File: "file.yml", Line: 1, Col: 1}},
				Map: map[string]meta.Object{
					"candy": {
						Origins: []meta.Origin{{File: "file.yml", Line: 1, Col: 8}},
						Value:   "bar",
					},
					"cats": {
						Origins: []meta.Origin{{File: "file.yml", Line: 2, Col: 1}},
						Array: []meta.Object{
							{
								Origins: []meta.Origin{{File: "file.yml", Line: 3, Col: 7}},
								Value:   "madd",
							},
							{
								Origins: []meta.Origin{{File: "file.yml", Line: 4, Col: 7}},
								Value:   "tabby",
							},
						},
					},
					"other": {
						Origins: []meta.Origin{{File: "file.yml", Line: 5, Col: 1}},
						Map: map[string]meta.Object{
							"things": {
								Origins: []meta.Origin{{File: "file.yml", Line: 6, Col: 5}},
								Map: map[string]meta.Object{
									"red": {
										Origins: []meta.Origin{{File: "file.yml", Line: 7, Col: 14}},
										Value:   "balloons",
									},
									"green": {
										Origins: []meta.Origin{{File: "file.yml", Line: 8, Col: 9}},
										Array: []meta.Object{
											{
												Origins: []meta.Origin{{File: "file.yml", Line: 9, Col: 15}},
												Value:   "grass",
											},
											{
												Origins: []meta.Origin{{File: "file.yml", Line: 10, Col: 15}},
												Value:   "ground",
											},
											{
												Origins: []meta.Origin{{File: "file.yml", Line: 11, Col: 15}},
												Value:   make(chan int),
											},
										},
									},
								},
							},
							"trending": {
								Origins: []meta.Origin{{File: "file.yml", Line: 12, Col: 15}},
								Value:   "now",
							},
						},
					},
				},
			},
			expectedErr: ErrEncoding,
		},
	}
	for _, tc := range tests {
		t.Run(tc.description, func(t *testing.T) {
			assert := assert.New(t)

			var e Encoder
			got, err := e.EncodeExtended(tc.in)

			if tc.expectedErr == nil {
				assert.NoError(err)
				assert.Empty(cmp.Diff(tc.expectedExtended, string(got)), "EncodeExtended(obj) failed")
				if cmp.Diff(tc.expectedExtended, string(got)) != "" {
					t.Logf("expected:\n%s", tc.expectedExtended)
					t.Logf("got:\n%s", string(got))
				}

				raw := tc.in.ToRaw()

				got, err = e.Encode(raw)
				assert.NoError(err)
				assert.Empty(cmp.Diff(tc.expected, string(got)), "Encode(raw) failed")
				return
			}

			assert.ErrorIs(err, tc.expectedErr)
			assert.Nil(got)
		})
	}
}

func TestDecodeComment(t *testing.T) {
	assert := assert.New(t)

	s, err := decodeComment("#")
	assert.Equal("", s)
	assert.Error(err)
}

func TestDetermineStyle(t *testing.T) {
	tests := []struct {
		input    string
		expected yaml.Style
	}{
		{"simple", yaml.TaggedStyle},
		{"multi\nline", yaml.DoubleQuotedStyle},
		{"noleadingColon:", yaml.TaggedStyle},
		{":leadingColon", yaml.DoubleQuotedStyle},
		{"-leadingDash", yaml.DoubleQuotedStyle},
		{"noleading-Dash", yaml.TaggedStyle},
		{"contains\\backslash", yaml.DoubleQuotedStyle},
		{"contains\"quote", yaml.DoubleQuotedStyle},
		{"contains\bbackspace", yaml.DoubleQuotedStyle},
		{"unicode\u0080", yaml.DoubleQuotedStyle},
		{"", yaml.DoubleQuotedStyle},
		{"endsWithSpace ", yaml.DoubleQuotedStyle},
	}

	for _, test := range tests {
		t.Run(test.input, func(t *testing.T) {
			result := determineStyle(test.input)
			if result != test.expected {
				t.Errorf("determineStyle(%q) = %v; want %v", test.input, result, test.expected)
			}
		})
	}
}
