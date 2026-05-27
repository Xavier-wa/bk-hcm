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

// Package app is the top-level service lifecycle manager for agent-server.
// Run() is the single entry-point: it loads configuration, initialises all
// subsystems, starts the HTTP server and channels, then blocks until the
// process receives a shutdown signal.
package app

import (
	"fmt"
	"net"
	"strconv"

	"hcm/cmd/agent-server/options"
	"hcm/cmd/agent-server/service"
	"hcm/pkg/cc"
	"hcm/pkg/logs"
	"hcm/pkg/metrics"
	"hcm/pkg/runtime/ctl"
	"hcm/pkg/runtime/shutdown"
	"hcm/pkg/serviced"

	"go.opentelemetry.io/otel"
	trpcmetric "trpc.group/trpc-go/trpc-agent-go/telemetry/metric"
)

// Run start the agent server
func Run(opt *options.Option) error {
	s := new(agentServer)
	if err := s.prepare(opt); err != nil {
		logs.Errorf("run prepare failed, err: %v", err)
		return err
	}

	if err := s.svc.ListenAndServeRest(); err != nil {
		logs.Errorf("run listen and serve rest failed, err: %v", err)
		return err
	}

	if err := s.register(); err != nil {
		logs.Errorf("run register failed, err: %v", err)
		return err
	}

	shutdown.RegisterFirstShutdown(s.finalizer)
	shutdown.WaitShutdown(20)
	return nil
}

type agentServer struct {
	svc *service.Service
	sd  serviced.Service
}

// prepare do prepare jobs before run agent server.
func (s *agentServer) prepare(opt *options.Option) error {
	// load settings from config file.
	if err := cc.LoadSettings(opt.Sys); err != nil {
		return fmt.Errorf("load settings from config files failed, err: %v", err)
	}

	logs.InitLogger(cc.AgentServer().Log.Logs())

	logs.Infof("load settings from config file success.")

	// init metrics
	network := cc.AgentServer().Network
	metrics.InitMetrics(net.JoinHostPort(network.BindIP, strconv.Itoa(int(network.Port))))

	// 桥接 OTel metrics 到 Prometheus
	if err := metrics.InitOTelMetrics(metrics.Register()); err != nil {
		return fmt.Errorf("init otel metrics failed, err: %v", err)
	}
	logs.Infof("otel metrics initialized and bridged to prometheus")

	// initialize trpc-agent-go built-in metrics using the global OTel provider
	if err := trpcmetric.InitMeterProvider(otel.GetMeterProvider()); err != nil {
		return fmt.Errorf("init trpc-agent-go metrics failed, err: %v", err)
	}
	logs.Infof("trpc-agent-go metrics initialized")

	// new api server discovery client.
	svcOpt := serviced.NewServiceOption(cc.AgentServerName, cc.AgentServer().Network, opt.Sys)
	discOpt := serviced.DiscoveryOption{Services: []cc.Name{cc.DataServiceName, cc.AuthServerName}}
	sd, err := serviced.NewServiceD(cc.AgentServer().Service, svcOpt, discOpt)
	if err != nil {
		return fmt.Errorf("new serviced discovery failed, err: %v", err)
	}
	s.sd = sd

	// init service.
	svc, err := service.NewService(sd)
	if err != nil {
		logs.Errorf("initialize service failed, err: %v", err)
		return fmt.Errorf("initialize service failed, err: %v", err)
	}
	s.svc = svc

	// init hcm control tool
	if err := ctl.LoadCtl(ctl.WithBasics(sd)...); err != nil {
		return fmt.Errorf("load control tool failed, err: %v", err)
	}

	logs.Infof("initialize service success.")
	return nil
}

// register agent-server to etcd.
func (s *agentServer) register() error {
	if err := s.sd.Register(); err != nil {
		return fmt.Errorf("register agent server failed, err: %v", err)
	}

	logs.Infof("register agent server to etcd success.")
	return nil
}

func (s *agentServer) finalizer() {
	if err := s.sd.Deregister(); err != nil {
		logs.Errorf("process service shutdown, but deregister failed, err: %v", err)
		return
	}

	logs.Infof("shutting down service, deregister service success.")
	return
}
