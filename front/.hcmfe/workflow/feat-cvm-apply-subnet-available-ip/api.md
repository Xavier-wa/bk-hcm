# API — 海垒申领界面及修改界面增加子网网段"可用IP"数量显示

> 参考后端接口文档 MR：`https://<GIT_HOST>/bcc/hcm/-/merge_requests/3341`（story 132811653，源分支 `ziyan_subnet_ip_count`）
> 本迭代仅前端适配，接口由后端已改造 / 改造中，前端按契约对齐字段即可。

## 1. 涉及接口

| 接口 | 方法 | 说明 |
|------|------|------|
| `/api/v1/woa/config/findmany/config/cvm/subnet` | POST | 私有子网列表查询（CvmSubnetSelector 数据源） |

## 2. 请求参数（不变）

```json
{
  "region": "ap-shanghai",
  "zone": "ap-shanghai-2",
  "vpc": "vpc-2x7lhtse"
}
```

| 参数 | 类型 | 说明 |
|------|------|------|
| region | string | 地域 |
| zone | string | 可用区 |
| vpc | string | VPC ID |

## 3. 响应结构

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {
    "count": 1,
    "info": [
      {
        "id": "00000001",
        "region": "ap-shanghai",
        "zone": "ap-shanghai-2",
        "vpc_id": "vpc-2x7lhtse",
        "vpc_name": "VPC-IEG-SH",
        "subnet_id": "subnet-ax907buf",
        "subnet_name": "cvm_use_199",
        "enable": true,
        "comment": "",
        "available_ip_count": 123
      }
    ]
  }
}
```

### `data.info[]` 字段

| 参数 | 类型 | 描述 |
|------|------|------|
| id | string | 子网自增 ID |
| region | string | 地域 |
| zone | string | 可用区 |
| vpc_id | string | VPC ID |
| vpc_name | string | VPC 名 |
| subnet_id | string | 私有子网云 ID |
| subnet_name | string | 私有子网名称 |
| enable | bool | 是否启用 |
| comment | string | 备注 |
| **available_ip_count** | **int \| null** | **剩余可用 IP 数，取自 CRP `leftIpNum`；查询失败/未取到时为 `null`，真实剩余 0 为 `0`** |

## 4. `available_ip_count` 语义与降级

- 数据源：CRP `getRealSubnetInfo.leftIpNum`，与生产调度选子网同一口径。
- 写入规则（后端约定，2026-08-19 更新）：

| 情况 | `available_ip_count` |
|------|---------------------|
| DB 有、CRP 命中 | CRP `leftIpNum` 转 `uint64`（含真实 `0`） |
| DB 有、CRP 未命中 | `null` |
| CRP 调用失败 / region·zone·vpc 不全 | 全部 `null` |
| CRP 有、DB 无 | 不进清单（忽略） |

- 清单条数始终由 `enable_cvm = true` 的 DB 清单决定，CRP 结果不新增/删除清单项。
- 失败时按项降级，不影响清单返回（HTTP 仍成功）。

## 5. 前端字段适配点

`front/src/views/ziyanScr/components/cvm-subnet-selector/index.vue` 中：

- `ICvmSubnet` 接口字段类型为 `available_ip_count: number | null;`
- Option 渲染 name 追加 ` | 可用IP: X`；`null` 时显示 ` | 可用IP: ?`

## 6. 前后端契约对齐结论（已决策）

**2026-08-19 更新**：后端将查询失败/未取到的 `available_ip_count` 由 `0` 改为返回 `null`，与真实剩余 `0` 区分。

**决策**（用户确认）：前端 `null` 显示 `可用IP: ?`（问号），仅真实 `0` 才显示 `可用IP: 0`。
