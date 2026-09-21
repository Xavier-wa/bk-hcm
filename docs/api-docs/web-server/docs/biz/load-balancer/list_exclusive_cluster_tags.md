### 描述

- 该接口提供版本：v9.9.9.9+。
- 该接口所需权限：业务访问。
- 该接口功能描述：查询业务下可用的负载均衡独占集群标签及标签下的集群，用于负载均衡购买页「独占型」规格选择。

### URL

POST /api/v1/cloud/bizs/{bk_biz_id}/load_balancers/exclusive_clusters/tags/list

### 输入参数

| 参数名称         | 参数类型         | 必选 | 描述                                                |
|--------------|--------------|----|---------------------------------------------------|
| bk_biz_id    | int64        | 是  | 业务ID，路径参数                                         |
| account_id   | string       | 是  | 账号ID                                              |
| region       | string       | 是  | 地域                                                |
| isp          | string       | 是  | 运营商（枚举值：CMCC、CUCC、CTCC） |
| zone         | string       | 否  | 根据可用区过滤独占集群列表               |
| cluster_type | string       | 否  | 集群类型（枚举值：TGW、STGW），为空则同时返回四层与七层             |

说明：

- 仅返回已分配给当前业务（`bk_biz_id` 等于路径业务）的公网独占集群。
- 服务端已过滤集群标签为空的集群，此类集群不可用于购买。
- 传入 `zone` 时只返回主可用区命中列表的集群；某标签下所有集群都不命中时，该标签不出现在 `details` 中。
- `details` 为空数组时，购买页应隐藏或禁用「独占型」规格。
- 四层（TGW）返回标签及标签下的集群列表；七层（STGW）云侧不支持指定集群，只返回标签，`clusters` 固定为空数组。
- 集群内的闲置 VIP 不在本接口返回，选定具体四层集群后请用该集群的 `cloud_cluster_id` 调用 `/load_balancers/exclusive_clusters/idle_vips/list` 实时查询。

### 调用示例

查询业务 213 在 ap-guangzhou 地域、ap-guangzhou-3 可用区下 BGP 线路的独占集群标签。

```json
{
  "account_id": "00000001",
  "region": "ap-guangzhou",
  "isp": "BGP",
  "zone": "ap-guangzhou-3",
  "cluster_type": "",
}
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": {
    "details": [
      {
        "cluster_tag": "ziyan_chiji",
        "cluster_type": "TGW",
        "clusters": [
          {
            "cloud_cluster_id": "tgw-38feq8c6",
            "cluster_id": "00000001",
            "cluster_name": "ziyan-l4-1",
            "egress": "center_egress1",
            "isp": "BGP",
            "zone": "ap-guangzhou-3",
          },
          {
            "cloud_cluster_id": "tgw-9k2lq7b1",
            "cluster_id": "00000003",
            "cluster_name": "ziyan-l4-2",
            "egress": "center_egress1",
            "isp": "BGP",
            "zone": "ap-guangzhou-4",
          }
        ]
      },
      {
        "cluster_tag": "ziyan_chiji",
        "cluster_type": "STGW",
        "clusters": [
          {
            "cloud_cluster_id": "stgw-38feq8c6",
            "cluster_id": "00000001",
            "cluster_name": "ziyan-l4-1",
            "egress": "center_egress1",
            "isp": "BGP",
            "zone": "ap-guangzhou-3",
          },
          {
            "cloud_cluster_id": "stgw-9k2lq7b1",
            "cluster_id": "00000003",
            "cluster_name": "ziyan-l4-2",
            "egress": "center_egress1",
            "isp": "BGP",
            "zone": "ap-guangzhou-4",
          }
        ]
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

| 参数名称    | 参数类型  | 描述      |
|---------|-------|---------|
| details | array | 标签聚合结果数组 |

#### data.details[n]

| 参数名称         | 参数类型          | 描述                            |
|--------------|---------------|-------------------------------|
| cluster_tag  | string        | 集群标签，购买时作为 `cluster_tag` 传入   |
| cluster_type | string        | 集群类型（TGW四层、STGW七层），前端映射为四层/七层 |
| clusters     | cluster array | 该标签下的集群列表，七层固定为空数组            |

##### cluster

| 参数名称                | 参数类型   | 描述                                                   |
|---------------------|--------|------------------------------------------------------|
| cloud_cluster_id    | string | 集群云上ID，购买时作为 `cloud_cluster_ids` 的元素传入               |
| cluster_id          | string | 集群本地ID                                               |
| cluster_name        | string | 集群名称                                                 |
| egress              | string | 网络出口，用于查询共享带宽包时作为 `egresses` 传入                      |
| isp                 | string | 运营商                                                  |
| zone                | string | 可用区                                                  |
