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
	"testing"
	"time"
)

func TestSignAPIRequest(t *testing.T) {
	tests := []struct {
		name      string
		apiSecret string
		timestamp string
		apiKey    string
		want      string
	}{
		{
			// 期望值来自 openssl: printf '%s' "1612345209test_api_key" | openssl dgst -sha1 -hmac "test_secret"
			name:      "正常签名",
			apiSecret: "test_secret",
			timestamp: "1612345209",
			apiKey:    "test_api_key",
			want:      "3db77f9b6a18aa215ae77e2a4e9b0ead490a2640",
		},
		{
			name:      "空密钥仍可计算签名",
			apiSecret: "",
			timestamp: "1612345209",
			apiKey:    "test_api_key",
			want:      "1c8ab7be9ed29ce0292d682ca491f3b99a60ea15",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := signAPIRequest(tt.apiSecret, tt.timestamp, tt.apiKey); got != tt.want {
				t.Errorf("signAPIRequest() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestSignAPIRequestConcatOrder 待加密文本必须是时间戳在前、api_key 在后，顺序反了云梯侧会鉴权失败
func TestSignAPIRequestConcatOrder(t *testing.T) {
	const (
		secret    = "test_secret"
		timestamp = "1612345209"
		apiKey    = "test_api_key"
	)

	if got, reversed := signAPIRequest(secret, timestamp, apiKey),
		hmacSHA1Hex(secret, apiKey+timestamp); got == reversed {
		t.Errorf("signAPIRequest() should sign timestamp+apiKey, not apiKey+timestamp")
	}

	if got, want := signAPIRequest(secret, timestamp, apiKey),
		hmacSHA1Hex(secret, timestamp+apiKey); got != want {
		t.Errorf("signAPIRequest() = %v, want %v", got, want)
	}
}

func TestAuthParams(t *testing.T) {
	cli := &cvmAPI{apiKey: "test_api_key", apiSecret: "test_secret"}

	before := time.Now().Unix()
	params := cli.authParams()
	after := time.Now().Unix()

	if len(params) != 3 {
		t.Fatalf("authParams() returns %d params, want 3", len(params))
	}

	if params[CvmAPIKey] != cli.apiKey {
		t.Errorf("authParams()[%s] = %v, want %v", CvmAPIKey, params[CvmAPIKey], cli.apiKey)
	}

	timestamp, err := strconv.ParseInt(params[CvmAPITs], 10, 64)
	if err != nil {
		t.Fatalf("authParams()[%s] is not a valid unix timestamp: %v", CvmAPITs, err)
	}
	if timestamp < before || timestamp > after {
		t.Errorf("authParams()[%s] = %d, want in range [%d, %d]", CvmAPITs, timestamp, before, after)
	}

	want := hmacSHA1Hex(cli.apiSecret, params[CvmAPITs]+cli.apiKey)
	if params[CvmAPISign] != want {
		t.Errorf("authParams()[%s] = %v, want %v", CvmAPISign, params[CvmAPISign], want)
	}
}

func TestNewCVMClientInterfaceValidate(t *testing.T) {
	tests := []struct {
		name    string
		opts    CVMCli
		wantErr bool
	}{
		{
			name:    "密钥完整",
			opts:    CVMCli{CvmAPIAddr: "http://127.0.0.1", APIKey: "test_api_key", APISecret: "test_secret"},
			wantErr: false,
		},
		{
			name:    "缺少api_key",
			opts:    CVMCli{CvmAPIAddr: "http://127.0.0.1", APISecret: "test_secret"},
			wantErr: true,
		},
		{
			name:    "缺少api_secret",
			opts:    CVMCli{CvmAPIAddr: "http://127.0.0.1", APIKey: "test_api_key"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewCVMClientInterface(tt.opts, nil)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewCVMClientInterface() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// hmacSHA1Hex 测试内独立实现一份签名算法，用于交叉校验被测函数
func hmacSHA1Hex(secret, data string) string {
	mac := hmac.New(sha1.New, []byte(secret))
	mac.Write([]byte(data))

	return hex.EncodeToString(mac.Sum(nil))
}
