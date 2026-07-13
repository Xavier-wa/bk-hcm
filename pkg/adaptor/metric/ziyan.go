/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2024 THL A29 Limited,
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

package metric

import (
	"bytes"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/tidwall/gjson"

	"hcm/pkg/criteria/enumor"
)

// GetZiyanRecordRoundTripper get record round tripper for tcloud
func GetZiyanRecordRoundTripper(next http.RoundTripper) promhttp.RoundTripperFunc {
	if next == nil {
		next = http.DefaultTransport
	}
	return func(req *http.Request) (*http.Response, error) {
		action := strings.Join(req.Header["X-TC-Action"], ",")
		region := strings.Join(req.Header["X-TC-Region"], ",")
		start := time.Now()
		code := "nil"
		ret, err := next.RoundTrip(req)
		if ret != nil {
			code = ret.Status
		}

		if err != nil || (ret != nil && ret.StatusCode != http.StatusOK) {
			cloudApiMetric.errCounter.With(prometheus.Labels{
				"vendor":    string(enumor.Ziyan),
				"endpoint":  req.Host,
				"region":    region,
				"api_name":  action,
				"http_code": code,
			}).Inc()
		}
		cost := time.Since(start)
		cloudApiMetric.lagSec.With(
			prometheus.Labels{
				"vendor":    string(enumor.Ziyan),
				"endpoint":  req.Host,
				"region":    region,
				"api_name":  action,
				"http_code": code,
			}).Observe(cost.Seconds())
		ObserveCloudAPI(string(enumor.Ziyan), action, cost, classifyCloudErr(err, ret))
		// 配合自研云多秘钥请求配置，记录秘钥请求，及其错误码
		var ak = ""
		authHeaders := req.Header["Authorization"]
		if len(authHeaders) == 1 {
			ak = GetTCloudSecretID(authHeaders[0])
		}
		var sdkErr = ""
		if err == nil {
			sdkErr, ret = tryReadError(ret)
		}

		ziyanLabels := prometheus.Labels{
			"endpoint":        req.Host,
			"region":          region,
			"api_name":        action,
			"http_code":       code,
			"secret_id":       ak,
			"tcloud_err_code": sdkErr,
		}
		cloudApiMetric.ziyanAkCounter.With(ziyanLabels).Inc()
		return ret, err
	}
}

func tryReadError(ret *http.Response) (string, *http.Response) {

	var sdkErr = ""
	// 尝试查询记录sdk错误
	b, err := io.ReadAll(ret.Body)
	if err != nil {
		return "ReadBodyError", ret
	}
	sdkErr = gjson.GetBytes(b, "Response.Error.Code").String()
	ret.Body = io.NopCloser(bytes.NewReader(b))
	return sdkErr, ret
}

// GetTCloudSecretID 格式： TC3-HMAC-SHA256 Credential=xxxxxxxx/
func GetTCloudSecretID(authHeader string) string {
	prefix := "TC3-HMAC-SHA256 Credential="
	prefixLength := len(prefix)
	if !strings.HasPrefix(authHeader, prefix) {
		return ""
	}
	// 	find first slash
	idx := strings.IndexByte(authHeader, '/')
	if idx == -1 {
		return ""
	}
	return authHeader[prefixLength:idx]
}
