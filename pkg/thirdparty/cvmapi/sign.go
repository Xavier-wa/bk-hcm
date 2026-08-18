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

package cvmapi

import (
	"crypto/hmac"
	"crypto/sha1"
	"encoding/hex"
	"strconv"
	"time"
)

// authParams 生成云梯接口签名鉴权的 query 参数。
// 云梯要求每次请求携带 api_key、api_ts、api_sign 三个参数，签名默认 10 分钟过期，因此每次请求都重新计算。
// 签名规范 docs: iwiki/p/4019306305
func (c *cvmAPI) authParams() map[string]string {
	timestamp := strconv.FormatInt(time.Now().Unix(), 10)

	return map[string]string{
		CvmAPIKey:  c.apiKey,
		CvmAPITs:   timestamp,
		CvmAPISign: signAPIRequest(c.apiSecret, timestamp, c.apiKey),
	}
}

// signAPIRequest 用 apiSecret 对文本（timestamp+apiKey）做 HmacSHA1 加密，返回十六进制签名串。
// 注意待加密文本的拼接顺序是时间戳在前、api_key 在后，与云梯侧保持一致。
func signAPIRequest(apiSecret, timestamp, apiKey string) string {
	mac := hmac.New(sha1.New, []byte(apiSecret))
	mac.Write([]byte(timestamp + apiKey))

	return hex.EncodeToString(mac.Sum(nil))
}
