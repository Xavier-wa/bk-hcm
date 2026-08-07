### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：服务-机房裁撤。
- 该接口功能描述：查询导出的裁撤主机明细（分页 JSON，单页上限 5000）。传入 snapshot_date 时按该日期 ES 快照补充性能/属性扩展字段。

### URL

POST /api/v1/woa/dissolve/host/detail/export/list

### 输入参数

| 参数名称        | 参数类型         | 必选 | 描述                                                  |
|-------------|--------------|----|-----------------------------------------------------|
| bk_biz_ids  | int64 array  | 否  | 业务ID列表                                              |
| project_ids | int array    | 否  | 裁撤项目ID列表                                            |
| group_ids   | int64 array  | 否  | 运维小组ID列表                                            |
| operators   | string array | 否  | 负责人列表                                       |
| modules     | string array | 否  | 裁撤模块名称列表                                            |
| inner_ips   | string array | 否  | 内网IP列表                                  |
| asset_ids   | string array | 否  | 主机固资号列表                                  |
| status      | string       | 否  | 裁撤状态，枚举值：complete（已裁撤）/incomplete（未裁撤），不传查全部       |
| expect_abolish_times | string array | 否  | 裁撤截止时间列表，格式 yyyy-MM-dd                     |
| snapshot_date | string     | 否  | ES 快照日期（格式 yyyyMMdd），传入时补充扩展字段                     |
| page        | object       | 是  | 分页设置，limit 最大 5000                                  |

#### page

| 参数名称  | 参数类型   | 必选 | 描述                  |
|-------|--------|----|---------------------|
| count | bool   | 是  | 是否返回总记录条数           |
| start | uint   | 否  | 记录开始位置              |
| limit | uint   | 否  | 每页限制条数，最大 5000      |
| sort  | string | 否  | 排序字段                |

### 调用示例

```json
{
  "bk_biz_ids": [100],
  "status": "incomplete",
  "snapshot_date": "20260601",
  "page": {
    "count": false,
    "start": 0,
    "limit": 5000
  }
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "count": 1,
    "details": [
      {
        "id": "00000001",
        "asset_id": "ABC123",
        "inner_ip": "1.1.1.1",
        "device_type": "S5.LARGE",
        "module": "module-a",
        "status": "incomplete",
        "project_id": 100,
        "project_name": "2026年第一批裁撤",
        "region": "ap-guangzhou",
        "bk_biz_id": 100,
        "group_id": 200,
        "operators": ["zhangsan"],
        "cpu_core": 64,
        "expect_abolish_time": "2026-12-31",
        "extension": {
          "outer_ip": "",
          "device_type": "S5.LARGE",
          "module_name": "module-a",
          "idc_unit_name": "广州-1",
          "sfw_name_version": "TencentOS Server 3.1",
          "go_up_date": "2023-01-01",
          "raid_name": "RAID1",
          "logic_area": "logic-area-a",
          "device_layer": "应用层",
          "cpu_score": 0.12,
          "mem_score": 0.34,
          "inner_net_traffic_score": 0.1,
          "disk_io_score": 0.2,
          "disk_util_score": 0.3,
          "is_pass": true,
          "mem4linux": 16,
          "inner_net_traffic": 1.2,
          "outer_net_traffic": 0.5,
          "disk_io": 100,
          "disk_util": 0.3,
          "disk_total": 500,
          "group_name": "运维一组",
          "center": "业务中心A"
        }
      }
    ]
  }
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述   |
|---------|--------|------|
| code    | int32  | 状态码  |
| message | string | 请求信息 |
| data    | object | 响应数据 |

#### data

| 参数名称    | 参数类型  | 描述     |
|---------|-------|--------|
| count   | int64 | 总记录条数  |
| details | array | 裁撤主机明细 |

#### data.details[n]

字段同 `POST /api/v1/woa/dissolve/host/detail/list`，额外包含：

| 参数名称      | 参数类型   | 描述                                       |
|-----------|--------|------------------------------------------|
| extension | object | ES 快照扩展字段（仅当传入 snapshot_date 且快照命中时返回）  |

#### data.details[n].extension

| 参数名称                    | 参数类型   | 描述            |
|-------------------------|--------|---------------|
| outer_ip                | string | 公网IP          |
| device_type             | string | SCM设备类型       |
| module_name             | string | 裁撤模块名称        |
| idc_unit_name           | string | 存放机房管理单元      |
| sfw_name_version        | string | 操作系统          |
| go_up_date              | string | 上架时间          |
| raid_name               | string | RAID结构        |
| logic_area              | string | 逻辑区域          |
| device_layer            | string | 设备技术分类        |
| cpu_score               | float  | CPU得分         |
| mem_score               | float  | 内存得分          |
| inner_net_traffic_score | float  | 内网流量得分        |
| disk_io_score           | float  | 磁盘IO得分        |
| disk_util_score         | float  | 磁盘IO使用率得分     |
| is_pass                 | bool   | 是否达标          |
| mem4linux               | float  | 内存使用量(G)      |
| inner_net_traffic       | float  | 内网流量(Mb/s)    |
| outer_net_traffic       | float  | 外网流量(Mb/s)    |
| disk_io                 | float  | 磁盘IO(Blocks/s)|
| disk_util               | float  | 磁盘IO使用率       |
| disk_total              | float  | 磁盘总量(G)       |
| group_name              | string | 运维小组          |
| center                  | string | 业务中心          |
