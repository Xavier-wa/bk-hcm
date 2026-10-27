/*
 * TencentBlueKing is pleased to support the open source community by making
 * 蓝鲸智云 - 混合云管理平台 (BlueKing - Hybrid Cloud Management System) available.
 * Copyright (C) 2022 THL A29 Limited,
 * a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing,
 * software distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the
 * specific language governing permissions and limitations under the License.
 *
 * We undertake not to change the open source license (MIT license) applicable
 *
 * to the current version of the project delivered to anyone in the future.
 */

package iacplugin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"hcm/cmd/hc-service/logics/iac-plugin/tf-plugin/pool"
	"hcm/pkg/api/core"
	protocore "hcm/pkg/api/core/cloud"
	protocloud "hcm/pkg/api/data-service/cloud"
	"hcm/pkg/criteria/enumor"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/dal/dao/tools"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
	"hcm/pkg/runtime/filter"
	cvt "hcm/pkg/tools/converter"
)

const (
	defaultPoolConcurrency = 32
	maxPoolConcurrency     = 128
	maxLeaseHoldMS         = 60000
	maxStaggerMS           = 1000
	providerRegion         = "ap-guangzhou"
)

// poolConfigureReq 对已启动的进程池并发 Acquire。
//   - AccountIDs 是云账号 ID（云侧主账号 ID 或 HCM 账号主键），认证字段从账号扩展里读取；
//   - Concurrency 是总并发数，默认 32，上限 128；
//   - FakeConfigs 是不同假配置的个数。传 nil 时兼容旧行为，超出 AccountIDs 的并发各自一份假配置；
//     传 0 表示全部并发只轮询 AccountIDs（测同配置复用与预约）；传 N > 0 表示后 N 个并发用假配置，其余轮询 AccountIDs；
//   - InvalidConfigs 是混入的坏配置个数（字段类型错误），用于确认坏配置不会拖死进程；
//   - HoldMS 是拿到 lease 后持有多久再归还，毫秒；
//   - AcquireTimeoutMS 是单个 Acquire 的超时时间，毫秒。传 > 0 时用来验证排队超时取消后不会泄漏名额；
//   - StaggerMS 是相邻两个请求启动的间隔，毫秒。传 > 0 时保证入队顺序严格按 index，不再靠协程随机抢锁；
//   - KillProcs 是在请求发出一小段时间后强制杀掉的池内进程数，用于验证进程崩溃后的自动重试与重建；
//   - KillDelayMS 是从发请求到执行 KillProcs 的等待时间，毫秒，默认 20ms（容易命中正在配置或刚交付的进程）。
type poolConfigureReq struct {
	PluginName       string   `json:"plugin_name"`
	PluginVersion    string   `json:"plugin_version"`
	AccountIDs       []string `json:"account_ids"`
	Concurrency      int      `json:"concurrency"`
	FakeConfigs      *int     `json:"fake_configs"`
	InvalidConfigs   int      `json:"invalid_configs"`
	HoldMS           int      `json:"hold_ms"`
	AcquireTimeoutMS int      `json:"acquire_timeout_ms"`
	StaggerMS        int      `json:"stagger_ms"`
	KillProcs        int      `json:"kill_procs"`
	KillDelayMS      int      `json:"kill_delay_ms"`
}

// poolConfigureResp 并发 Acquire 的汇总与明细。
type poolConfigureResp struct {
	Summary poolConfigureSummary  `json:"summary"`
	Details []poolConfigureResult `json:"details"`
}

// poolConfigureSummary 本轮并发的动作、重试与进程分布。
type poolConfigureSummary struct {
	Total       int            `json:"total"`
	Succeeded   int            `json:"succeeded"`
	Failed      int            `json:"failed"`
	Retried     int            `json:"retried"`
	KilledProcs []uint64       `json:"killed_procs,omitempty"`
	Actions     map[string]int `json:"actions"`
	Processes   map[uint64]int `json:"processes"`
}

// poolConfigureResult 一次 Acquire 的结果。
// Kind 取值：real（真实账号）、fake（假密钥）、invalid（字段类型错误）。
type poolConfigureResult struct {
	Index       int    `json:"index"`
	Kind        string `json:"kind"`
	AccountID   string `json:"account_id,omitempty"`
	Real        bool   `json:"real"`
	ProcID      uint64 `json:"proc_id,omitempty"`
	Action      string `json:"action,omitempty"`
	QueueWaitMS int64  `json:"queue_wait_ms"`
	CostMS      int64  `json:"cost_ms"`
	HoldMS      int64  `json:"hold_ms,omitempty"`
	Retries     int    `json:"retries,omitempty"`
	Error       string `json:"error,omitempty"`
}

// configureJob 一次 Acquire。secrets 只用于把错误信息里的认证字段替换掉，禁止写入日志。
type configureJob struct {
	index          int
	kind           string
	accountID      string
	real           bool
	hold           time.Duration
	acquireTimeout time.Duration
	config         map[string]any
	secrets        []string
}

// ConfigurePluginPool 并发 Acquire 已启动的进程池，按 HoldMS 持有后归还。ConfigureProvider 在 Acquire 内部完成。
func (s *svc) ConfigurePluginPool(cts *rest.Contexts) (any, error) {
	req := new(poolConfigureReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("configure plugin pool decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if err := validatePoolConfigureReq(req); err != nil {
		logs.Errorf("configure plugin pool invalid parameter, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}
	if s.pools == nil {
		logs.Errorf("configure plugin pool failed, pool is not started, plugin: %s, rid: %s",
			req.PluginName, cts.Kit.Rid)
		return nil, errf.New(errf.Aborted, "plugin pool is not started")
	}

	pluginPool, err := s.pools.GetPool(req.PluginName)
	if err != nil {
		logs.Errorf("get plugin pool failed, plugin: %s, version: %s, err: %v, rid: %s",
			req.PluginName, req.PluginVersion, err, cts.Kit.Rid)
		return nil, errf.Newf(errf.RecordNotFound, "plugin pool %s is not enabled", req.PluginName)
	}
	if pluginPool.Version() != req.PluginVersion {
		logs.Errorf("plugin pool version mismatch, plugin: %s, request: %s, running: %s, rid: %s",
			req.PluginName, req.PluginVersion, pluginPool.Version(), cts.Kit.Rid)
		return nil, errf.Newf(errf.InvalidParameter, "running pool version is %s", pluginPool.Version())
	}

	jobs, err := s.buildConfigureJobs(cts, req)
	if err != nil {
		return nil, err
	}

	logs.Infof("configure plugin pool start, plugin: %s, version: %s, account_count: %d, concurrency: %d, "+
		"hold_ms: %d, acquire_timeout_ms: %d, stagger_ms: %d, kill_procs: %d, rid: %s", req.PluginName,
		req.PluginVersion, len(req.AccountIDs), len(jobs), req.HoldMS, req.AcquireTimeoutMS, req.StaggerMS,
		req.KillProcs, cts.Kit.Rid)

	results, killed := runConfigureBatch(cts, pluginPool, jobs, req)

	summary := summarizeConfigureResults(results, killed)
	logs.Infof("configure plugin pool finish, plugin: %s, version: %s, total: %d, succeeded: %d, failed: %d, "+
		"retried: %d, killed_procs: %v, actions: %v, processes: %v, rid: %s", req.PluginName, req.PluginVersion,
		summary.Total, summary.Succeeded, summary.Failed, summary.Retried, summary.KilledProcs, summary.Actions,
		summary.Processes, cts.Kit.Rid)
	return &poolConfigureResp{Summary: summary, Details: results}, nil
}

func runConfigureBatch(cts *rest.Contexts, pluginPool *pool.PluginPool, jobs []configureJob,
	req *poolConfigureReq) ([]poolConfigureResult, []uint64) {

	results := make([]poolConfigureResult, len(jobs))
	var wg sync.WaitGroup
	var killed []uint64

	if req.KillProcs > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-time.After(time.Duration(req.KillDelayMS) * time.Millisecond):
				killed = pluginPool.KillProcesses(req.KillProcs)
				logs.Infof("killed plugin pool processes, plugin: %s, killed_procs: %v, rid: %s",
					pluginPool.Name(), killed, cts.Kit.Rid)
			case <-cts.Kit.Ctx.Done():
			}
		}()
	}

	stagger := time.Duration(req.StaggerMS) * time.Millisecond
	for i := range jobs {
		if i > 0 && stagger > 0 {
			time.Sleep(stagger)
		}
		wg.Add(1)
		go func(job configureJob) {
			defer wg.Done()
			results[job.index] = acquireOnce(cts, pluginPool, job)
		}(jobs[i])
	}
	wg.Wait()
	return results, killed
}

func validatePoolConfigureReq(req *poolConfigureReq) error {
	if req.PluginName == "" || req.PluginVersion == "" || len(req.AccountIDs) == 0 {
		return errf.New(errf.InvalidParameter, "plugin_name, plugin_version and account_ids are required")
	}
	if req.Concurrency == 0 {
		req.Concurrency = defaultPoolConcurrency
	}
	if req.Concurrency < len(req.AccountIDs) {
		req.Concurrency = len(req.AccountIDs)
	}
	if req.Concurrency > maxPoolConcurrency {
		return errf.Newf(errf.InvalidParameter, "concurrency should be between 1 and %d", maxPoolConcurrency)
	}
	if req.HoldMS < 0 || req.HoldMS > maxLeaseHoldMS {
		return errf.Newf(errf.InvalidParameter, "hold_ms should be between 0 and %d", maxLeaseHoldMS)
	}
	if req.AcquireTimeoutMS < 0 || req.AcquireTimeoutMS > maxLeaseHoldMS {
		return errf.Newf(errf.InvalidParameter, "acquire_timeout_ms should be between 0 and %d", maxLeaseHoldMS)
	}
	if req.StaggerMS < 0 || req.StaggerMS > maxStaggerMS {
		return errf.Newf(errf.InvalidParameter, "stagger_ms should be between 0 and %d", maxStaggerMS)
	}
	if req.KillProcs < 0 || req.KillProcs > maxPoolConcurrency {
		return errf.New(errf.InvalidParameter, "kill_procs should not be negative")
	}
	if req.KillDelayMS < 0 || req.KillDelayMS > maxLeaseHoldMS {
		return errf.Newf(errf.InvalidParameter, "kill_delay_ms should be between 0 and %d", maxLeaseHoldMS)
	}
	if req.KillProcs > 0 && req.KillDelayMS == 0 {
		req.KillDelayMS = 20
	}
	if req.InvalidConfigs < 0 {
		return errf.New(errf.InvalidParameter, "invalid_configs should not be negative")
	}
	if req.FakeConfigs == nil {
		defaultFakes := max(req.Concurrency-len(req.AccountIDs)-req.InvalidConfigs, 0)
		req.FakeConfigs = cvt.ValToPtr(defaultFakes)
	}
	if *req.FakeConfigs < 0 {
		return errf.New(errf.InvalidParameter, "fake_configs should not be negative")
	}
	if *req.FakeConfigs+req.InvalidConfigs > req.Concurrency {
		return errf.New(errf.InvalidParameter, "fake_configs + invalid_configs should not exceed concurrency")
	}
	return nil
}

func (s *svc) buildConfigureJobs(cts *rest.Contexts, req *poolConfigureReq) ([]configureJob, error) {
	realTemplates := make([]configureJob, len(req.AccountIDs))
	for i, accountID := range req.AccountIDs {
		job, err := s.realAccountJob(cts, i, accountID)
		if err != nil {
			logs.Errorf("load account secret failed, account_id: %s, err: %v, rid: %s", accountID, err, cts.Kit.Rid)
			return nil, err
		}
		realTemplates[i] = job
		logs.Infof("load account secret success, account_id: %s, rid: %s", accountID, cts.Kit.Rid)
	}

	hold := time.Duration(req.HoldMS) * time.Millisecond
	acquireTimeout := time.Duration(req.AcquireTimeoutMS) * time.Millisecond
	realSlots := req.Concurrency - *req.FakeConfigs - req.InvalidConfigs
	jobs := make([]configureJob, req.Concurrency)
	for i := 0; i < realSlots; i++ {
		tpl := realTemplates[i%len(realTemplates)]
		tpl.index = i
		tpl.hold = hold
		tpl.acquireTimeout = acquireTimeout
		jobs[i] = tpl
	}
	for i := 0; i < *req.FakeConfigs; i++ {
		idx := realSlots + i
		job := fakeJob(idx, hold)
		job.acquireTimeout = acquireTimeout
		jobs[idx] = job
	}
	for i := 0; i < req.InvalidConfigs; i++ {
		idx := realSlots + *req.FakeConfigs + i
		job := invalidJob(idx)
		job.acquireTimeout = acquireTimeout
		jobs[idx] = job
	}
	return jobs, nil
}

func summarizeConfigureResults(results []poolConfigureResult, killed []uint64) poolConfigureSummary {
	s := poolConfigureSummary{
		Total:       len(results),
		KilledProcs: killed,
		Actions:     make(map[string]int),
		Processes:   make(map[uint64]int),
	}
	for _, r := range results {
		if r.Retries > 0 {
			s.Retried++
		}
		if r.Error != "" {
			s.Failed++
			continue
		}
		s.Succeeded++
		if r.Action != "" {
			s.Actions[r.Action]++
		}
		if r.ProcID > 0 {
			s.Processes[r.ProcID]++
		}
	}
	return s
}

func (s *svc) realAccountJob(cts *rest.Contexts, index int, accountID string) (configureJob, error) {
	ext, err := s.loadTCloudAccountExtension(cts, accountID)
	if err != nil {
		return configureJob{}, err
	}
	secretID, secretKey := ext.CloudSecretID, ext.CloudSecretKey
	if secretID == "" || secretKey == "" {
		return configureJob{}, errf.Newf(errf.InvalidParameter, "account %s secret is empty", accountID)
	}
	return configureJob{
		index:     index,
		kind:      "real",
		accountID: accountID,
		real:      true,
		config:    providerConfig(secretID, secretKey),
		secrets:   secretValues(ext),
	}, nil
}

// loadTCloudAccountExtension 先按云侧主账号 ID 查，没有命中再按 HCM 账号主键查。
func (s *svc) loadTCloudAccountExtension(cts *rest.Contexts, accountID string) (*protocore.TCloudAccountExtension,
	error) {

	listResp, err := s.dataCli.Global.Account.List(cts.Kit.Ctx, cts.Kit.Header(), &protocloud.AccountListReq{
		Filter: tools.ExpressionAnd(
			tools.RuleEqual("vendor", enumor.TCloud),
			&filter.AtomRule{Field: "extension.cloud_main_account_id", Op: filter.JSONEqual.Factory(), Value: accountID},
		),
		Page: &core.BasePage{Limit: 1},
	})
	if err != nil {
		return nil, errf.NewFromErr(errf.Aborted, err)
	}
	resolvedID := accountID
	if listResp != nil && len(listResp.Details) > 0 {
		resolvedID = listResp.Details[0].ID
	}

	account, err := s.dataCli.TCloud.Account.Get(cts.Kit.Ctx, cts.Kit.Header(), resolvedID)
	if err != nil {
		return nil, errf.NewFromErr(errf.Aborted, err)
	}
	if account == nil || account.Extension == nil {
		return nil, errf.Newf(errf.RecordNotFound, "account %s extension is empty", accountID)
	}
	return account.Extension, nil
}

func fakeJob(index int, hold time.Duration) configureJob {
	secretID := fmt.Sprintf("fake-secret-id-%d", index)
	secretKey := "fake-secret-key"
	return configureJob{
		index:   index,
		kind:    "fake",
		real:    false,
		hold:    hold,
		config:  providerConfig(secretID, secretKey),
		secrets: []string{secretID, secretKey},
	}
}

// invalidJob 构造字段类型错误的配置：secret_id 传整数，编码阶段即被拒绝，用来验证进程不会被坏配置拖死。
func invalidJob(index int) configureJob {
	return configureJob{
		index: index,
		kind:  "invalid",
		real:  false,
		config: map[string]any{
			"secret_id":  index,
			"secret_key": "invalid-type-test",
		},
	}
}

func acquireOnce(cts *rest.Contexts, pluginPool *pool.PluginPool, job configureJob) poolConfigureResult {
	start := time.Now()
	logs.Infof("acquire plugin pool start, plugin: %s, index: %d, kind: %s, account_id: %s, hold_ms: %d, rid: %s",
		pluginPool.Name(), job.index, job.kind, job.accountID, job.hold.Milliseconds(), cts.Kit.Rid)

	kt := *cts.Kit
	if job.acquireTimeout > 0 {
		ctx, cancel := context.WithTimeout(cts.Kit.Ctx, job.acquireTimeout)
		defer cancel()
		kt.Ctx = ctx
	}

	lease, err := pluginPool.Acquire(&kt, job.config)
	cost := time.Since(start).Milliseconds()
	if err != nil {
		safeErr := redact(err.Error(), job.secrets)
		logs.Errorf("acquire plugin pool failed, plugin: %s, index: %d, kind: %s, account_id: %s, err: %s, "+
			"cost_ms: %d, rid: %s", pluginPool.Name(), job.index, job.kind, job.accountID, safeErr, cost, cts.Kit.Rid)
		return poolConfigureResult{
			Index: job.index, Kind: job.kind, AccountID: job.accountID, Real: job.real, CostMS: cost, Error: safeErr,
		}
	}

	res := poolConfigureResult{
		Index:       job.index,
		Kind:        job.kind,
		AccountID:   job.accountID,
		Real:        job.real,
		ProcID:      lease.ProcID(),
		Action:      lease.Action(),
		QueueWaitMS: lease.QueueWaitMS(),
		CostMS:      cost,
		Retries:     lease.Retries(),
	}
	logs.Infof("acquire plugin pool success, plugin: %s, index: %d, kind: %s, account_id: %s, proc_id: %d, "+
		"action: %s, queue_wait_ms: %d, cost_ms: %d, retries: %d, rid: %s", pluginPool.Name(), job.index, job.kind,
		job.accountID, res.ProcID, res.Action, res.QueueWaitMS, res.CostMS, res.Retries, cts.Kit.Rid)

	if job.hold > 0 {
		holdStart := time.Now()
		select {
		case <-time.After(job.hold):
		case <-cts.Kit.Ctx.Done():
		}
		res.HoldMS = time.Since(holdStart).Milliseconds()
	}

	lease.Release()
	logs.Infof("release plugin pool lease, plugin: %s, index: %d, kind: %s, account_id: %s, proc_id: %d, "+
		"hold_ms: %d, rid: %s", pluginPool.Name(), job.index, job.kind, job.accountID, res.ProcID, res.HoldMS,
		cts.Kit.Rid)
	return res
}

// providerConfig 是 ConfigureProvider 的 Provider 配置。gRPC Configure 只消费 schema 里的字段，
// 认证用 secret_id 与 secret_key，region 不是密钥。
func providerConfig(secretID, secretKey string) map[string]any {
	return map[string]any{
		"secret_id":  secretID,
		"secret_key": secretKey,
		"region":     providerRegion,
	}
}

func secretValues(ext *protocore.TCloudAccountExtension) []string {
	if ext == nil {
		return nil
	}
	return []string{ext.CloudSecretID, ext.CloudSecretKey}
}

func redact(msg string, secrets []string) string {
	for _, secret := range secrets {
		if secret == "" {
			continue
		}
		msg = strings.ReplaceAll(msg, secret, "***")
	}
	return msg
}
