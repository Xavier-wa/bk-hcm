/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on
 * an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package cmdb

import (
	"encoding/base64"
	"strconv"
	"strings"
	"time"

	"hcm/pkg/kit"
	"hcm/pkg/logs"
)

const (
	// cursorFieldSep cursor 解码后各字段的分隔符。
	cursorFieldSep = "\r"
	// cursorEventTimeFieldIdx 事件时间（unix 秒）在解码后字段中的下标，即第 5 个字段。
	cursorEventTimeFieldIdx = 4
	// cursorMinFieldNum cursor 解码后的最少字段数，不足则视为格式不符。
	cursorMinFieldNum = cursorEventTimeFieldIdx + 1
)

// CursorDetail is the decoded result of a cc watch cursor.
// cursor 格式为 cc 服务端约定（base64 编码、`\r` 分隔文本），非本仓库契约，
// 故解码失败时降级保留原值，调用方据 Decoded 判断事件时间是否可信。
type CursorDetail struct {
	// Raw cursor 原值，未解码或解码失败时仍可用于人工排查。
	Raw string
	// Decoded 是否成功解出事件时间。
	Decoded bool
	// EventTime 事件时间，Decoded 为 false 时为零值。
	EventTime time.Time
	// Fields 解码后的全部字段，便于人工比对 cc 侧格式变化。
	Fields []string
}

// EventTimeString returns the formatted event time, or "unknown" if not decoded.
func (c *CursorDetail) EventTimeString() string {
	if c == nil || !c.Decoded {
		return "unknown"
	}
	return c.EventTime.Format(time.DateTime)
}

// DecodeCursor decodes a cc watch cursor and extracts its event time.
// It always returns a detail for degradation when decode fails.
func DecodeCursor(kt *kit.Kit, raw string) *CursorDetail {
	detail := &CursorDetail{Raw: raw}
	if len(raw) == 0 {
		return detail
	}
	rid := ""
	if kt != nil {
		rid = kt.Rid
	}

	decoded, err := decodeCursorBase64(raw)
	if err != nil {
		logs.Warnf("decode cc event cursor base64 failed, err: %v, cursor: %s, rid: %s", err, raw, rid)
		return detail
	}

	detail.Fields = strings.Split(string(decoded), cursorFieldSep)
	if len(detail.Fields) < cursorMinFieldNum {
		logs.Warnf("decode cc event cursor fields failed, fields: %d, min fields: %d, cursor: %s, rid: %s",
			len(detail.Fields), cursorMinFieldNum, raw, rid)
		return detail
	}

	sec, err := strconv.ParseInt(detail.Fields[cursorEventTimeFieldIdx], 10, 64)
	if err != nil {
		logs.Warnf("decode cc event cursor time failed, err: %v, value: %s, cursor: %s, rid: %s",
			err, detail.Fields[cursorEventTimeFieldIdx], raw, rid)
		return detail
	}
	if sec <= 0 {
		logs.Warnf("decode cc event cursor time failed, invalid event time: %d, cursor: %s, rid: %s",
			sec, raw, rid)
		return detail
	}

	detail.Decoded = true
	detail.EventTime = time.Unix(sec, 0)
	return detail
}

// decodeCursorBase64 tries the padded and un-padded base64 variants, since the
// encoding used by cc is not part of any published contract.
func decodeCursorBase64(raw string) ([]byte, error) {
	decoded, err := base64.StdEncoding.DecodeString(raw)
	if err == nil {
		return decoded, nil
	}
	// 标准格式失败，降级尝试无 padding 格式
	return base64.RawStdEncoding.DecodeString(raw)
}
