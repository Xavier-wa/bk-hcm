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

package dao

import daoiac "hcm/pkg/dal/dao/iac"

// IacRepository 返回 IacRepository DAO。
func (s *set) IacRepository() daoiac.IacRepository {
	return &daoiac.IacRepositoryDao{
		Orm:   s.orm,
		IDGen: s.idGen,
	}
}

// IacSpec 返回 IacSpec DAO。
func (s *set) IacSpec() daoiac.IacSpec {
	return &daoiac.IacSpecDao{
		Orm:   s.orm,
		IDGen: s.idGen,
	}
}

// IacSpecNode 返回 IacSpecNode DAO。
func (s *set) IacSpecNode() daoiac.IacSpecNode {
	return &daoiac.IacSpecNodeDao{
		Orm:   s.orm,
		IDGen: s.idGen,
	}
}

// IacState 返回 IacState DAO。
func (s *set) IacState() daoiac.IacState {
	return &daoiac.IacStateDao{
		Orm:   s.orm,
		IDGen: s.idGen,
	}
}
