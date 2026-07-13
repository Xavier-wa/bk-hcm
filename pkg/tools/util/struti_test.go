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
)

func TestCheckLen(t *testing.T) {
	type args struct {
		sInput string
		min    int
		max    int
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{
			args: args{"123", 0, 3},
			want: true,
		},
		{
			args: args{"123", 1, 2},
			want: false,
		},
		{
			args: args{"123", -1, 3},
			want: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := CheckLen(tt.args.sInput, tt.args.min, tt.args.max); got != tt.want {
				t.Errorf("CheckLen() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsChar(t *testing.T) {
	type args struct {
		sInput string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{args: args{"c"}, want: true},
		{args: args{" c"}, want: false},
		{args: args{"c "}, want: false},
		{args: args{"和"}, want: false},
		{args: args{"_"}, want: false},
		{args: args{"3"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsChar(tt.args.sInput); got != tt.want {
				t.Errorf("IsChar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsNumChar(t *testing.T) {
	type args struct {
		sInput string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{args: args{"1"}, want: true},
		{args: args{"aA1"}, want: true},
		{args: args{" 1"}, want: false},
		{args: args{"1 "}, want: false},
		{args: args{"和"}, want: false},
		{args: args{"_"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsNumChar(tt.args.sInput); got != tt.want {
				t.Errorf("IsNumChar() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsDate(t *testing.T) {
	type args struct {
		sInput string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{args: args{"2018-10-10"}, want: true},
		{args: args{"2018/10/10"}, want: false},
		{args: args{`2018\10\10`}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDate(tt.args.sInput); got != tt.want {
				t.Errorf("IsDate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsTime(t *testing.T) {
	type args struct {
		sInput string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{args: args{"2018-10-10 10:56:67"}, want: true},
		{args: args{"105667"}, want: false},
		{args: args{`10-56-67`}, want: false},
		{args: args{"2021-04-07T21:50:50.351153+08:00"}, want: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if _, got := IsTime(tt.args.sInput); got != tt.want {
				t.Errorf("IsTime() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestContainsWord(t *testing.T) {
	type args struct {
		s    string
		word string
	}
	tests := []struct {
		name string
		args args
		want bool
	}{
		{name: "首部独立单词", args: args{"a2 instance core", "a2"}, want: true},
		{name: "中部独立单词", args: args{"g2 instance ram", "g2"}, want: true},
		{name: "尾部独立单词", args: args{"core running g4", "g4"}, want: true},
		{name: "整串相等", args: args{"a3", "a3"}, want: true},
		{name: "左边界为字母不命中", args: args{"a3ultra core", "a3"}, want: false},
		{name: "右边界为数字不命中", args: args{"a1000 series", "a100"}, want: false},
		{name: "前后均为字母不命中", args: args{"xg4y component", "g4"}, want: false},
		{name: "子串非独立单词不命中", args: args{"p40 gpu", "p4"}, want: false},
		{name: "不存在返回 false", args: args{"n1 instance", "g4"}, want: false},
		{name: "空 word 返回 false", args: args{"a2 instance", ""}, want: false},
		{name: "区分大小写", args: args{"A2 Instance", "a2"}, want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ContainsWord(tt.args.s, tt.args.word); got != tt.want {
				t.Errorf("ContainsWord(%q, %q) = %v, want %v", tt.args.s, tt.args.word, got, tt.want)
			}
		})
	}
}
