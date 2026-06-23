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

package mcp

import (
	"fmt"

	"hcm/pkg/logs"

	mcpsdk "trpc.group/trpc-go/trpc-mcp-go"
)

// hcmLoggerAdapter 把 trpc-mcp-go.Logger 接口适配到 hcm/pkg/logs。
//
// trpc-mcp-go 框架内部通过 Logger 输出协议层与传输层日志，复用 hcm/pkg/logs 后
// 这些日志可以与项目其他模块的日志一同走 glog 通路，统一文件 / 滚动策略。
//
// 由于 hcm/pkg/logs 没有 Debug 级别 API，Debug/Debugf 通过 logs.V(2) 输出，
// 仅在调试 verbosity ≥ 2 时落盘。
type hcmLoggerAdapter struct {
	// component 写到每条日志前作为前缀，便于在大量日志中筛选 MCP 相关项，
	// 形如 `[mcp/ingress] ...`。
	component string
}

// newLoggerAdapter 构造一个把 trpc-mcp-go 日志接到 hcm 日志的适配器。
// component 取值如 "mcp/ingress"、"mcp/internal"，会作为日志前缀。
func newLoggerAdapter(component string) mcpsdk.Logger {
	return &hcmLoggerAdapter{component: component}
}

// prefix 返回带 component 前缀的格式串，避免每个方法重复拼接。
func (l *hcmLoggerAdapter) prefix(format string) string {
	if l.component == "" {
		return format
	}
	return "[" + l.component + "] " + format
}

// Debug 写入 verbose level 2 日志。
func (l *hcmLoggerAdapter) Debug(args ...interface{}) {
	if logs.V(2) {
		logs.Infof(l.prefix("%s"), fmt.Sprint(args...))
	}
}

// Debugf 写入 verbose level 2 日志。
func (l *hcmLoggerAdapter) Debugf(format string, args ...interface{}) {
	if logs.V(2) {
		logs.Infof(l.prefix(format), args...)
	}
}

// Info 写入 info 日志。
func (l *hcmLoggerAdapter) Info(args ...interface{}) {
	logs.Infof(l.prefix("%s"), fmt.Sprint(args...))
}

// Infof 写入 info 日志。
func (l *hcmLoggerAdapter) Infof(format string, args ...interface{}) {
	logs.Infof(l.prefix(format), args...)
}

// Warn 写入 warn 日志。
func (l *hcmLoggerAdapter) Warn(args ...interface{}) {
	logs.Warnf(l.prefix("%s"), fmt.Sprint(args...))
}

// Warnf 写入 warn 日志。
func (l *hcmLoggerAdapter) Warnf(format string, args ...interface{}) {
	logs.Warnf(l.prefix(format), args...)
}

// Error 写入 error 日志。
func (l *hcmLoggerAdapter) Error(args ...interface{}) {
	logs.Errorf(l.prefix("%s"), fmt.Sprint(args...))
}

// Errorf 写入 error 日志。
func (l *hcmLoggerAdapter) Errorf(format string, args ...interface{}) {
	logs.Errorf(l.prefix(format), args...)
}

// Fatal 写入 fatal 日志（进程退出）。
//
// 注意：trpc-mcp-go 当前主流路径不调用 Fatal，但接口要求实现。
func (l *hcmLoggerAdapter) Fatal(args ...interface{}) {
	logs.Fatalf(l.prefix("%s"), fmt.Sprint(args...))
}

// Fatalf 写入 fatal 日志（进程退出）。
func (l *hcmLoggerAdapter) Fatalf(format string, args ...interface{}) {
	logs.Fatalf(l.prefix(format), args...)
}
