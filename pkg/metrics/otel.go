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

package metrics

import (
	"fmt"

	"github.com/prometheus/client_golang/prometheus"
	"go.opentelemetry.io/otel"
	promexporter "go.opentelemetry.io/otel/exporters/prometheus"
	"go.opentelemetry.io/otel/sdk/metric"
)

// InitOTelMetrics 初始化 OpenTelemetry metrics 并桥接到 Prometheus Registry。
// 该函数创建 Prometheus Exporter，注册到提供的 registry，并设置全局 MeterProvider。
// trpc-agent-go 框架的内置指标将通过全局 MeterProvider 自动注册。
func InitOTelMetrics(registry prometheus.Registerer) error {
	// 创建 Prometheus Exporter 并注册到现有 Registry
	exporter, err := promexporter.New(
		promexporter.WithRegisterer(registry),
		promexporter.WithoutScopeInfo(),
	)
	if err != nil {
		return fmt.Errorf("create prometheus exporter: %w", err)
	}

	// 创建 MeterProvider，使用 Prometheus Exporter 作为 Reader
	provider := metric.NewMeterProvider(
		metric.WithReader(exporter),
	)

	// 设置为全局 MeterProvider
	// trpc-agent-go 框架将使用全局 provider 注册其内置的 OTel 指标
	otel.SetMeterProvider(provider)

	return nil
}
