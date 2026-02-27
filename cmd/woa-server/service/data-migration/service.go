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

// Package datamigration provides data migration service
package datamigration

import (
	"net/http"

	"hcm/cmd/woa-server/service/capability"
	"hcm/cmd/woa-server/service/data-migration/config"
	"hcm/cmd/woa-server/service/data-migration/framework"
	"hcm/cmd/woa-server/storage/dal"
	"hcm/pkg/api/core"
	"hcm/pkg/criteria/errf"
	"hcm/pkg/iam/auth"
	"hcm/pkg/iam/meta"
	"hcm/pkg/logs"
	"hcm/pkg/rest"
)

// InitService initial the service
func InitService(c *capability.Capability) {
	// 获取ORM实例用于数据迁移时更新时间字段
	ormInst := c.Dao.GetOrm()

	// 创建API路由器
	apiRouter := NewClientAPIRouter(c.Client.DataService().TCloudZiyan, ormInst)

	// 创建迁移引擎
	engine := framework.NewMigrationEngine(c.MongoDB, c.Client.DataService().TCloudZiyan, apiRouter)

	s := &service{
		authorizer: c.Authorizer,
		mongo:      c.MongoDB,
		apiRouter:  apiRouter,
		engine:     engine,
	}

	h := rest.NewHandler()
	h.Path("/data_migration")

	s.initService(h)
	h.Load(c.WebService)
}

type service struct {
	authorizer auth.Authorizer
	mongo      dal.DB
	apiRouter  *ClientAPIRouter
	engine     *framework.MigrationEngine
}

// initService 数据迁移服务接口
func (s *service) initService(h *rest.Handler) {
	h.Add("MigrateData", http.MethodPost, "/migrate", s.MigrateData)
	h.Add("ListConfigs", http.MethodGet, "/configs", s.ListConfigs)
}

// MigrateData 执行数据迁移
func (s *service) MigrateData(cts *rest.Contexts) (interface{}, error) {
	req := &framework.MigrationRequest{}
	if err := cts.DecodeInto(req); err != nil {
		return nil, errf.NewFromErr(errf.DecodeRequestFailed, err)
	}

	// 验证请求
	if err := s.validateMigrationRequest(req); err != nil {
		return nil, err
	}

	if err := s.authorizer.AuthorizeWithPerm(cts.Kit, meta.ResourceAttribute{
		Basic: &meta.Basic{Type: meta.ZiyanResDeliverAnalyze, Action: meta.Update},
	}); err != nil {
		logs.Errorf("[MigrationEngine:MigrateData] no permission to migrate data, err: %v, rid: %s", err, cts.Kit.Rid)
		return nil, err
	}

	logs.Infof("[MigrationEngine:MigrateData] Start migration: table=%s, mode=%s, dry_run=%v, "+
		"include_dependencies=%v, rid=%s", req.TableName, req.Mode, req.DryRun, req.IncludeDependencies, cts.Kit.Rid)

	// 执行迁移
	result, err := s.engine.Migrate(cts.Kit, req)
	if err != nil {
		logs.Errorf("[MigrationEngine:MigrateData] Migration failed: %v, rid: %s", err, cts.Kit.Rid)
		return nil, errf.Newf(errf.Unknown, "migration failed: %v", err)
	}

	logs.Infof("[MigrationEngine:MigrateData] Migration completed: total=%d, created=%d, updated=%d, skipped=%d, "+
		"failed=%d, rid: %s", result.Total, result.Created, result.Updated, result.Skipped, result.Failed, cts.Kit.Rid)

	return result, nil
}

// ListConfigs 列出所有配置
func (s *service) ListConfigs(cts *rest.Contexts) (interface{}, error) {
	configs := config.List()

	result := &core.ListResultT[*config.TableMigrationConfig]{
		Count:   uint64(len(configs)),
		Details: configs,
	}

	return result, nil
}

// validateMigrationRequest 验证迁移请求
func (s *service) validateMigrationRequest(req *framework.MigrationRequest) error {
	if req.TableName == "" {
		return errf.New(errf.InvalidParameter, "table_name is required")
	}

	if req.Mode == "" {
		req.Mode = framework.MigrationModeSync
	}

	// 验证模式
	switch req.Mode {
	case framework.MigrationModeCreateOnly,
		framework.MigrationModeUpdateOnly,
		framework.MigrationModeSync:
		// 合法模式
	default:
		return errf.Newf(errf.InvalidParameter, "invalid mode: %s", req.Mode)
	}

	return nil
}
