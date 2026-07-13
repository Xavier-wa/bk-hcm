# API — 安全组-关联实例-增加复制按钮

> **本功能无新增后端 API**。复制操作均为前端行为，依赖已有接口获取数据。

## 1. 接口总览

| 接口 | 方法 | 用途 | 使用场景 |
|------|------|------|----------|
| 查询安全组关联资源（资源视图） | `POST` | 账号下/资源运营列表数据 | 功能1 |
| 查询安全组关联资源（业务视图） | `POST` | 业务下折叠面板列表数据 | 功能2 |
| rollRequest 全量拉取 | — | 拉取全部页数据 | 功能2 |

---

## 2. 接口1：查询关联资源（资源视图）

**platform.vue** 调用 `securityGroupStore.queryRelatedResourcesBySgId()`。

### 2.1 请求

```
POST /api/v1/cloud/security_groups/{sgId}/related_resources/{resType}s/list
```

| 参数 | 位置 | 类型 | 说明 |
|------|------|------|------|
| `sgId` | path | `string` | 安全组 ID |
| `resType` | path | `string` | 资源类型：`cvm` / `load_balancer` |

**请求体**（`QueryBuilderType`）：

```json
{
  "filter": { "op": "and", "rules": [...] },
  "page": { "start": 0, "limit": 20, "count": false }
}
```

分页参数：

| 字段 | 类型 | 说明 |
|------|------|------|
| `page.start` | `number` | 偏移量，从 0 开始 |
| `page.limit` | `number` | 每页条数 |
| `page.count` | `boolean` | 是否返回总数 |

> 注：现有调用通过 `enableCount()` 将 count 拆为两个并行请求（`count: false` 取列表 + `count: true` 取总数）。

### 2.2 响应

```json
{
  "data": {
    "details": [
      {
        "id": "xxx",
        "cloud_id": "ins-xxxxx",
        "name": "my-cvm",
        "vendor": "tencent",
        "bk_biz_id": 100,
        "account_id": "xxx",
        "region": "ap-guangzhou",
        "status": "RUNNING",
        "private_ipv4_addresses": ["10.0.0.1"],
        "private_ipv6_addresses": ["fe80::1"],
        "public_ipv4_addresses": ["1.2.3.4"],
        "public_ipv6_addresses": [],
        "zone": "ap-guangzhou-3",
        "cloud_vpc_ids": ["vpc-xxx"],
        "cloud_subnet_ids": ["subnet-xxx"]
      }
    ],
    "count": 100
  }
}
```

### 2.3 本功能用到的字段

| 字段 | 类型 | 用途 |
|------|------|------|
| `private_ipv4_addresses` | `string[]` | 复制内网 IP（IPv4 部分） |
| `private_ipv6_addresses` | `string[]` | 复制内网 IP（IPv6 部分） |
| `cloud_id` | `string` | 复制实例 ID |

---

## 3. 接口2：查询关联资源（业务视图）

**collapse-data-list.vue** 调用 `securityGroupStore.queryRelatedResourcesByBiz()`。

### 3.1 请求

```
POST /api/v1/cloud/{bizs/{bizId}/}security_groups/{sgId}/related_resources/biz_resources/{resBizId}/{resType}s/list
```

| 参数 | 位置 | 类型 | 说明 |
|------|------|------|------|
| `bizId` | path | `number` | 业务 ID（仅业务页，`getBusinessApiPath()` 决定是否拼接） |
| `sgId` | path | `string` | 安全组 ID |
| `resBizId` | path | `number` | 资源所属业务 ID |
| `resType` | path | `string` | 资源类型 |

请求体与接口1相同。

### 3.2 响应

响应结构与接口1相同，字段一致。

### 3.3 功能2 全量拉取

功能2 需复制当前业务下**全部**关联实例数据，若接口分页则通过 `rollRequest` 拉取全量：

```ts
import rollRequest from '@blueking/roll-request';
import http from '@/http';

// 拉取当前业务下全部关联资源
const allList = await rollRequest({
  httpClient: http,
  pageEnableCountKey: 'count',
}).rollReqUseCount<SecurityGroupRelatedResourceItem>(
  apiPath,
  { filter: ... },
  {
    limit: 500,
    countGetter: (res) => res.data.count,
    listGetter: (res) => res.data.details,
  },
);
```

---

## 4. 错误处理

| 场景 | 处理方式 |
|------|----------|
| 接口请求失败 | 不复制，提示"获取数据失败" |
| 数据为空 | 按钮置灰 |
| 字段缺失（如无 `cloud_id`） | 跳过该行，不中断复制 |
| 剪贴板 API 不可用 | `CopyToClipboard` 组件内置降级（`document.execCommand('copy')`） |

---

## 5. 安全说明

- 本功能不涉及权限接口变更
- 复制数据仅从已加载到前端的数据中提取，不会越权获取未授权的数据
- 功能1 的跨页选择需跟进权限校验一致性
