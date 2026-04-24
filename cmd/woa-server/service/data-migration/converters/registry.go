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

package converters

import (
	"fmt"
	"sync"
)

// Registry 转换器注册表
type Registry struct {
	converters map[string]DataConverter
	mu         sync.RWMutex
}

// globalRegistry 全局转换器注册表
var globalRegistry = &Registry{
	converters: make(map[string]DataConverter),
}

// Register 注册转换器
func Register(converter DataConverter) {
	globalRegistry.mu.Lock()
	defer globalRegistry.mu.Unlock()
	globalRegistry.converters[converter.GetName()] = converter
}

// Get 获取转换器
func Get(name string) (DataConverter, error) {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	converter, ok := globalRegistry.converters[name]
	if !ok {
		return nil, fmt.Errorf("converter %s not found", name)
	}

	return converter, nil
}

// List 列出所有转换器
func List() map[string]DataConverter {
	globalRegistry.mu.RLock()
	defer globalRegistry.mu.RUnlock()

	result := make(map[string]DataConverter, len(globalRegistry.converters))
	for name, converter := range globalRegistry.converters {
		result[name] = converter
	}

	return result
}
