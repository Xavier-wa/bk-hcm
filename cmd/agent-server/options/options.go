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

// Package options defines the startup flags and environment-variable defaults for agent-server.
package options

import (
	"github.com/spf13/pflag"

	"hcm/pkg/cc"
	"hcm/pkg/runtime/flags"
)

// Option holds all startup parameters for agent-server.
type Option struct {
	// Sys is the standard cc system option (config-file, bind-ip, env, labels, disable-election).
	// Populated by flags.SysFlags; used directly by cc.LoadSettings and serviced.NewServiceOption.
	Sys *cc.SysOption
}

// InitOptions init data service's options from command flags.
func InitOptions() *Option {
	fs := pflag.CommandLine
	sysOpt := flags.SysFlags(fs)
	opt := &Option{Sys: sysOpt}

	// parses the command-line flags from os.Args[1:]. must be called after all flags are defined
	// and before flags are accessed by the program.
	pflag.Parse()

	// check if the command-line flag is show current version info cmd.
	sysOpt.CheckV()

	return opt
}
