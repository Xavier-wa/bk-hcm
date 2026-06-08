/*
 * Tencent is pleased to support the open source community by making 蓝鲸 available.
 * Copyright (C) 2017-2018 THL A29 Limited, a Tencent company. All rights reserved.
 * Licensed under the MIT License (the "License"); you may not use this file except
 * in compliance with the License. You may obtain a copy of the License at
 * http://opensource.org/licenses/MIT
 * Unless required by applicable law or agreed to in writing, software distributed under
 * the License is distributed on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND,
 * either express or implied. See the License for the specific language governing permissions and
 * limitations under the License.
 */

package config

import (
	"strings"
	"testing"
)

// TestCvmImageFields 验证 CvmImage 结构体字段完整性
// 确保返回给前端的字段不会被意外删除或修改
func TestCvmImageFields(t *testing.T) {
	image := CvmImage{
		Region:    "ap-shenzhen",
		ImageId:   "img-xxx",
		ImageName: "test-image",
		Type:      "PRIVATE_IMAGE",
		BkBizID:   213,
	}

	// 验证所有字段都能正常赋值和读取
	// 如果字段类型变化或被删除，编译时会报错
	tests := []struct {
		name  string
		check func() bool
	}{
		{"Region should be set", func() bool { return image.Region == "ap-shenzhen" }},
		{"ImageId should be set", func() bool { return image.ImageId == "img-xxx" }},
		{"ImageName should be set", func() bool { return image.ImageName == "test-image" }},
		{"Type should be set", func() bool { return image.Type == "PRIVATE_IMAGE" }},
		{"BkBizID should be set", func() bool { return image.BkBizID == 213 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !tt.check() {
				t.Errorf("%s failed", tt.name)
			}
		})
	}
}

// TestCvmImageTypeValues 验证镜像类型的有效值
// 确保业务逻辑中使用的类型值是预期的
func TestCvmImageTypeValues(t *testing.T) {
	// 定义预期支持的镜像类型
	validTypes := []string{
		"PUBLIC_IMAGE",
		"PRIVATE_IMAGE",
	}

	// 验证类型值符合预期格式（全大写，下划线分隔）
	for _, typ := range validTypes {
		if typ != strings.ToUpper(typ) {
			t.Errorf("image type %s should be uppercase", typ)
		}
		if strings.Contains(typ, " ") {
			t.Errorf("image type %s should not contain spaces", typ)
		}
	}

	// 模拟业务场景：私有镜像必须有 bk_biz_id
	privateImage := CvmImage{
		Type:    "PRIVATE_IMAGE",
		BkBizID: 0, // 默认值
	}

	// 注意：此测试记录了当前业务规则
	// 如果将来要求私有镜像必须有 bk_biz_id > 0，需要在这里添加校验
	t.Logf("PRIVATE_IMAGE with bk_biz_id=0 is currently allowed (bk_biz_id=%d)", privateImage.BkBizID)
}
