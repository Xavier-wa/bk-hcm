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

package iacplugin

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"testing"

	coreplugin "hcm/pkg/api/core/plugin"
)

func TestValidateAPIPath(t *testing.T) {
	valid := []string{
		"/",
		"/api/bk-hcm-plugin-cmdb/prod",
		"/a",
		"/api/v1.2/prod",
	}
	for _, p := range valid {
		if err := ValidateAPIPath(p); err != nil {
			t.Errorf("ValidateAPIPath(%q) unexpected err: %v", p, err)
		}
	}

	invalid := []string{
		"",
		"api/x",
		"http://evil.example.com/x",
		"https://evil.example.com/x",
		"//evil.example.com/x",
		"/api/../admin",
		"/api/%2e%2e/admin",
		"/api/x\n",
		"/api/x?a=1",
		"/api/x#f",
		"/api/%zz",
	}
	for _, p := range invalid {
		if err := ValidateAPIPath(p); err == nil {
			t.Errorf("ValidateAPIPath(%q) expect error, got nil", p)
		}
	}
}

func TestJoinPluginURL(t *testing.T) {
	cases := []struct {
		name    string
		prefix  string
		apiPath string
		want    string
		wantErr bool
	}{
		{name: "合法", prefix: "https://bkapi.example.com", apiPath: "/api/p/prod",
			want: "https://bkapi.example.com/api/p/prod"},
		{name: "前缀带尾斜杠", prefix: "https://bkapi.example.com/", apiPath: "/api/p/prod",
			want: "https://bkapi.example.com/api/p/prod"},
		{name: "前缀带路径", prefix: "http://gw.example.com:8080/env", apiPath: "/p",
			want: "http://gw.example.com:8080/env/p"},
		{name: "前缀未配置", prefix: "", apiPath: "/p", wantErr: true},
		{name: "带主机名", prefix: "https://bkapi.example.com", apiPath: "//evil.example.com/x", wantErr: true},
		// 不以 / 开头时，拼接 @ 会把前缀变成 userinfo、host 随之改变，形状校验先拦下。
		{name: "拼接后 host 被改变", prefix: "https://bkapi.example.com", apiPath: "@evil.example.com/x",
			wantErr: true},
		{name: "路径穿越", prefix: "https://bkapi.example.com", apiPath: "/a/../../x", wantErr: true},
	}

	for _, c := range cases {
		got, err := JoinPluginURL(c.prefix, c.apiPath)
		if c.wantErr {
			if err == nil {
				t.Errorf("%s: expect error, got %s", c.name, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("%s: unexpected err: %v", c.name, err)
			continue
		}
		if got != c.want {
			t.Errorf("%s: got %s, want %s", c.name, got, c.want)
		}
	}
}

func TestValidateCrypto(t *testing.T) {
	rsa3072 := genRSAPublicKeyPEM(t, 3072, "PUBLIC KEY")
	rsa2048 := genRSAPublicKeyPEM(t, 2048, "PUBLIC KEY")

	ecKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate ec key failed, err: %v", err)
	}
	ecDer, err := x509.MarshalPKIXPublicKey(&ecKey.PublicKey)
	if err != nil {
		t.Fatalf("marshal ec key failed, err: %v", err)
	}
	ecPEM := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: ecDer}))

	rsaKey, err := rsa.GenerateKey(rand.Reader, 3072)
	if err != nil {
		t.Fatalf("generate rsa key failed, err: %v", err)
	}
	pkcs1PEM := string(pem.EncodeToMemory(&pem.Block{Type: "RSA PUBLIC KEY",
		Bytes: x509.MarshalPKCS1PublicKey(&rsaKey.PublicKey)}))

	cases := []struct {
		name       string
		cryptoType string
		cfg        *coreplugin.HTTPCryptoConfig
		wantErr    bool
	}{
		{name: "合法公钥", cryptoType: CryptoTypeRSA, cfg: &coreplugin.HTTPCryptoConfig{PublicKey: rsa3072}},
		{name: "2048 位", cryptoType: CryptoTypeRSA, cfg: &coreplugin.HTTPCryptoConfig{PublicKey: rsa2048}, wantErr: true},
		{name: "EC 公钥", cryptoType: CryptoTypeRSA, cfg: &coreplugin.HTTPCryptoConfig{PublicKey: ecPEM}, wantErr: true},
		{name: "PKCS1 非 SPKI", cryptoType: CryptoTypeRSA, cfg: &coreplugin.HTTPCryptoConfig{PublicKey: pkcs1PEM},
			wantErr: true},
		{name: "非 PEM", cryptoType: CryptoTypeRSA, cfg: &coreplugin.HTTPCryptoConfig{PublicKey: "not a pem"},
			wantErr: true},
		{name: "多个 PEM 块", cryptoType: CryptoTypeRSA,
			cfg: &coreplugin.HTTPCryptoConfig{PublicKey: rsa3072 + rsa3072}, wantErr: true},
		{name: "不支持的加密类型", cryptoType: "AES", cfg: &coreplugin.HTTPCryptoConfig{PublicKey: rsa3072}, wantErr: true},
		{name: "缺少加密类型", cryptoType: "", cfg: &coreplugin.HTTPCryptoConfig{PublicKey: rsa3072}, wantErr: true},
		{name: "缺少配置", cryptoType: CryptoTypeRSA, cfg: nil, wantErr: true},
		{name: "缺少公钥", cryptoType: CryptoTypeRSA, cfg: &coreplugin.HTTPCryptoConfig{}, wantErr: true},
	}

	for _, c := range cases {
		err := ValidateCrypto(c.cryptoType, c.cfg)
		if c.wantErr != (err != nil) {
			t.Errorf("%s: wantErr %v, got err: %v", c.name, c.wantErr, err)
		}
	}
}

func genRSAPublicKeyPEM(t *testing.T, bits int, pemType string) string {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		t.Fatalf("generate rsa key failed, err: %v", err)
	}
	der, err := x509.MarshalPKIXPublicKey(&key.PublicKey)
	if err != nil {
		t.Fatalf("marshal rsa key failed, err: %v", err)
	}

	return string(pem.EncodeToMemory(&pem.Block{Type: pemType, Bytes: der}))
}
