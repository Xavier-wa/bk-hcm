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

// Package util provides ...
package util

import "fmt"

// Truncate truncates a string to a maximum length, appending "..." and the total length if truncated.
func Truncate(s string, maxLen int) string {
	if maxLen <= 0 {
		return s
	}
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + fmt.Sprintf("... (%d chars total)", len(s))
}

// TruncateRune truncates a string to a maximum rune length, appending "..." and the total length if truncated.
func TruncateRune(s string, n int) string {
	if n <= 0 {
		return s
	}

	runes := []rune(s)
	if len(runes) <= n {
		return s
	}
	return string(runes[:n]) + fmt.Sprintf("... (%d chars total)", len(runes))
}
