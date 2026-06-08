### 描述

- 该接口提供版本：v1.8.7+。
- 该接口所需权限：平台管理-机房裁撤。
- 该接口功能描述：添加或更新裁撤配置。

### URL

PUT /api/v1/woa/dissolve/config/upsert

### 输入参数

| 参数名称            | 参数类型              | 必选 | 描述                                                           |
|-----------------|-------------------|----|--------------------------------------------------------------|
| host_apply_time | string            | 否  | 统计机房裁撤主机的开始时间                                                |
| approval_limit  | string            | 否  | 主机裁撤项目申请自动审批配置，超过比例后，则申请单需要管理员人工审批，范围0-100                   |
| quota_coefficient | float64         | 否  | 配额系数（百分比），用于计算可申请额度，范围1-100，默认值65                           |
| quota_offsets   | []QuotaOffsetItem | 否  | 业务偏移配置数组。注意：此字段为全量覆盖，传空数组`[]`将清空所有业务的偏移配置                    |

#### QuotaOffsetItem

| 参数名称     | 参数类型   | 必选 | 描述                                    |
|----------|--------|----|-----------------------------------------|
| bk_biz_id | int64 | 是  | 业务ID，必须大于0，且不能重复                      |
| offset   | int64  | 是  | 偏移值，必须大于等于0                           |
| type     | string | 是  | 调整类型：increase=调增，decrease=调减          |
| memo     | string | 否  | 调整原因，最大512字符                          |

### 调用示例

#### 请求参数示例

```json
{
   "host_apply_time": "2024-09-01T12:00:00Z",
   "approval_limit": 100,
   "quota_coefficient": 65,
   "quota_offsets": [
      {"bk_biz_id": 100001, "offset": 50, "type": "increase", "memo": "特殊业务需求"},
      {"bk_biz_id": 100002, "offset": 30, "type": "decrease", "memo": "额度回收"}
   ]
}

```

### 响应示例

#### 获取详细信息返回结果示例

```json
{
  "code": 0,
  "message": "ok",
  "data": null
}
```

### 响应参数说明

| 参数名称  | 参数类型   | 描述   |
|---------|-----------|--------|
| code    | int       | 状态码  |
| message | string    | 请求信息 |
| data    | object    | 响应数据 |

