/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package util

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestTruncate(t *testing.T) {
	tests := []struct {
		name   string
		s      string
		maxLen int
		want   string
	}{
		{
			name:   "maxLen is zero returns origin",
			s:      "hello",
			maxLen: 0,
			want:   "hello",
		},
		{
			name:   "maxLen is negative returns origin",
			s:      "hello",
			maxLen: -1,
			want:   "hello",
		},
		{
			name:   "length not exceed maxLen returns origin",
			s:      "hello",
			maxLen: 5,
			want:   "hello",
		},
		{
			name:   "length exceed maxLen truncated with suffix",
			s:      "hello world",
			maxLen: 5,
			want:   "hello... (11 chars total)",
		},
		{
			name:   "empty string returns origin",
			s:      "",
			maxLen: 3,
			want:   "",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, Truncate(tt.s, tt.maxLen))
		})
	}
}

func TestTruncateRune(t *testing.T) {
	tests := []struct {
		name string
		s    string
		n    int
		want string
	}{
		{
			name: "n is zero returns origin",
			s:    "hello",
			n:    0,
			want: "hello",
		},
		{
			name: "n is negative returns origin",
			s:    "hello",
			n:    -1,
			want: "hello",
		},
		{
			name: "rune length not exceed n returns origin",
			s:    "hello",
			n:    5,
			want: "hello",
		},
		{
			name: "ascii string exceed n truncated with suffix",
			s:    "hello world",
			n:    5,
			want: "hello... (11 chars total)",
		},
		{
			name: "multibyte string truncated by rune count",
			s:    "你好世界abc",
			n:    2,
			want: "你好... (7 chars total)",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.want, TruncateRune(tt.s, tt.n))
		})
	}
}
