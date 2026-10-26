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
)

const (
	defaultPoolConcurrency = 32
	maxPoolConcurrency     = 128
	providerRegion         = "ap-guangzhou"
)

// poolConfigureReq 对已启动的进程池并发 Acquire。
// AccountIDs 是云账号 ID，认证字段从账号扩展里读取。Concurrency 是总并发数，超出账号数的请求使用假配置。
type poolConfigureReq struct {
	PluginName    string   `json:"plugin_name"`
	PluginVersion string   `json:"plugin_version"`
	AccountIDs    []string `json:"account_ids"`
	Concurrency   int      `json:"concurrency"`
}

// poolConfigureResp 每个并发请求的 Acquire 结果。
type poolConfigureResp struct {
	Details []poolConfigureResult `json:"details"`
}

// poolConfigureResult 一次 Acquire 的结果。Real 为 true 表示使用了库里的认证配置。
type poolConfigureResult struct {
	Index     int    `json:"index"`
	AccountID string `json:"account_id,omitempty"`
	Real      bool   `json:"real"`
	CostMS    int64  `json:"cost_ms"`
	Error     string `json:"error,omitempty"`
}

// configureJob 一次 Acquire。secrets 只用于把错误信息里的认证字段替换掉，禁止写入日志。
type configureJob struct {
	index     int
	accountID string
	real      bool
	config    map[string]any
	secrets   []string
}

// ConfigurePluginPool 并发 Acquire 已启动的进程池，成功后立即归还。ConfigureProvider 在 Acquire 内部完成。
func (s *svc) ConfigurePluginPool(cts *rest.Contexts) (any, error) {
	req := new(poolConfigureReq)
	if err := cts.DecodeInto(req); err != nil {
		logs.Errorf("configure plugin pool decode request failed, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}
	if req.PluginName == "" || req.PluginVersion == "" || len(req.AccountIDs) == 0 {
		logs.Errorf("configure plugin pool invalid parameter, plugin: %s, version: %s, account_count: %d, rid: %s",
			req.PluginName, req.PluginVersion, len(req.AccountIDs), cts.Kit.Rid)
		return nil, errf.New(errf.InvalidParameter, "plugin_name, plugin_version and account_ids are required")
	}
	concurrency := req.Concurrency
	if concurrency == 0 {
		concurrency = defaultPoolConcurrency
	}
	if concurrency < len(req.AccountIDs) {
		concurrency = len(req.AccountIDs)
	}
	if concurrency > maxPoolConcurrency {
		logs.Errorf("configure plugin pool invalid concurrency: %d, rid: %s", concurrency, cts.Kit.Rid)
		return nil, errf.Newf(errf.InvalidParameter, "concurrency should be between 1 and %d", maxPoolConcurrency)
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

	jobs := make([]configureJob, concurrency)
	for i, accountID := range req.AccountIDs {
		job, err := s.realAccountJob(cts, i, accountID)
		if err != nil {
			logs.Errorf("load account secret failed, account_id: %s, err: %v, rid: %s", accountID, err, cts.Kit.Rid)
			return nil, err
		}
		jobs[i] = job
		logs.Infof("load account secret success, account_id: %s, rid: %s", accountID, cts.Kit.Rid)
	}
	for i := len(req.AccountIDs); i < concurrency; i++ {
		jobs[i] = fakeJob(i)
	}

	logs.Infof("configure plugin pool start, plugin: %s, version: %s, account_count: %d, concurrency: %d, rid: %s",
		req.PluginName, req.PluginVersion, len(req.AccountIDs), concurrency, cts.Kit.Rid)

	results := make([]poolConfigureResult, concurrency)
	var wg sync.WaitGroup
	for i := range jobs {
		wg.Add(1)
		go func(job configureJob) {
			defer wg.Done()
			results[job.index] = acquireOnce(cts, pluginPool, job)
		}(jobs[i])
	}
	wg.Wait()

	logs.Infof("configure plugin pool finish, plugin: %s, version: %s, concurrency: %d, rid: %s",
		req.PluginName, req.PluginVersion, concurrency, cts.Kit.Rid)
	return &poolConfigureResp{Details: results}, nil
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

func fakeJob(index int) configureJob {
	secretID := fmt.Sprintf("fake-secret-id-%d", index)
	secretKey := "fake-secret-key"
	return configureJob{
		index:   index,
		real:    false,
		config:  providerConfig(secretID, secretKey),
		secrets: []string{secretID, secretKey},
	}
}

func acquireOnce(cts *rest.Contexts, pluginPool *pool.PluginPool, job configureJob) poolConfigureResult {
	start := time.Now()
	logs.Infof("acquire plugin pool start, plugin: %s, index: %d, account_id: %s, real: %t, rid: %s",
		pluginPool.Name(), job.index, job.accountID, job.real, cts.Kit.Rid)

	lease, err := pluginPool.Acquire(cts.Kit, job.config)
	cost := time.Since(start).Milliseconds()
	if err != nil {
		safeErr := redact(err.Error(), job.secrets)
		logs.Errorf("acquire plugin pool failed, plugin: %s, index: %d, account_id: %s, real: %t, err: %s, "+
			"cost_ms: %d, rid: %s", pluginPool.Name(), job.index, job.accountID, job.real, safeErr, cost, cts.Kit.Rid)
		return poolConfigureResult{Index: job.index, AccountID: job.accountID, Real: job.real, CostMS: cost, Error: safeErr}
	}

	logs.Infof("acquire plugin pool success, plugin: %s, index: %d, account_id: %s, real: %t, cost_ms: %d, rid: %s",
		pluginPool.Name(), job.index, job.accountID, job.real, cost, cts.Kit.Rid)
	lease.Release()
	logs.Infof("release plugin pool lease, plugin: %s, index: %d, account_id: %s, real: %t, rid: %s",
		pluginPool.Name(), job.index, job.accountID, job.real, cts.Kit.Rid)
	return poolConfigureResult{Index: job.index, AccountID: job.accountID, Real: job.real, CostMS: cost}
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
