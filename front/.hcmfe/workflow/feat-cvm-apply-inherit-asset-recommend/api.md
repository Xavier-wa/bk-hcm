# API — 主机申领-滚服项目-继承固资号选择推荐-前端

> 工作流: feat-cvm-apply-inherit-asset-recommend
> 阶段: API 接口契约定义
> 说明: 本需求涉及 2 个**新增接口**（候选固资列表推荐，分别对应资源管理/服务请求两个入口）+ 1 个**已有接口**（固资校验，复用逻辑）。

---

## 一、已有接口（复用，不改动）

### 1.1 固资校验接口

> 已有实现，复用逻辑。`asset-match.vue` 中 `handleCheck` 调用 `cvmDeviceStore.getInheritCvm`。选择固资后自动调用此接口校验。

| 项 | 值 |
| --- | --- |
| **URL** | `POST /api/v1/woa/bizs/{bk_biz_id}/task/check/apply/order/host` |
| **定义位置** | `src/store/cvm/device.ts` L166-189 |
| **调用方** | `asset-match.vue` `handleCheck` |

**请求参数**：

```typescript
{
  require_type: number;     // 需求类型（RequirementType 枚举）
  bk_biz_id: number;       // 业务 ID
  bk_asset_id: string;     // 固资号
  region: string;          // 地域
}
```

**响应结构**：

```typescript
// IQueryResData<IInheritCvm>
{
  code: number;        // 0 表示成功
  message?: string;
  data: IInheritCvm;
}

// IInheritCvm（src/store/cvm/device.ts L55-65）
interface IInheritCvm {
  device_type: string;             // 机型
  device_group: string;            // 机型族
  instance_charge_type: string;    // 计费模式
  charge_months: number;           // 剩余计费月数
  billing_start_time: string;      // 计费起始时间
  old_billing_expire_time: string; // 原计费过期时间
  new_billing_expire_time: string; // 新计费过期时间
  bk_cloud_inst_id: string;        // 云主机实例 ID
  generation_type: string;          // 生成类型
}
```

**调用逻辑**：
- `res.code === 0` → 校验成功，取 `res.data` 赋值 `inheritCvm`，emit `checkSuccess`
- `res.code !== 0` 或异常 → 校验失败，emit `checkFail`，展示错误消息

---

## 二、新增接口（候选固资列表推荐）

> 后端已提供接口文档，分两个入口（资源管理/服务请求），响应结构一致。接口版本 v9.9.9+。

### 2.1 接口 A：资源管理入口（有 bk_biz_id 路径参数）

| 项 | 值 |
| --- | --- |
| **URL** | `POST /api/v1/woa/bizs/{bk_biz_id}/rolling_servers/inherited_hosts/list` |
| **方法** | POST |
| **权限** | 业务访问 |
| **用途** | 查询业务下滚服项目可继承的固资候选，按入参机型族分组返回，每族按已使用时长由长到短最多返回 5 条 |
| **版本** | v9.9.9+ |

**路径参数**：

| 参数名称 | 参数类型 | 必选 | 描述 |
| --- | --- | --- | --- |
| bk_biz_id | int64 | 是 | 业务 ID |

### 2.2 接口 B：服务请求入口（无 bk_biz_id 路径参数）

| 项 | 值 |
| --- | --- |
| **URL** | `POST /api/v1/woa/rolling_servers/inherited_hosts/list` |
| **方法** | POST |
| **权限** | 平台管理-自研云资源-主机申领 |
| **用途** | 同接口 A，服务请求入口 |
| **版本** | v9.9.9+ |

> **接口 A 与 B 差异**：接口 A 的 `bk_biz_id` 在路径参数中；接口 B 的 `bk_biz_id` 在 body 中传。响应结构完全一致。

### 2.3 请求参数（Body）

| 参数名称 | 参数类型 | 必选 | 描述 |
| --- | --- | --- | --- |
| bk_biz_id | int64 | 接口 B 必填 / 接口 A 不传 | 业务 ID（接口 B 在 body 传，接口 A 以路径参数为准） |
| region | string | 是 | 云地域标识，如 `ap-guangzhou`，最大长度 128 |
| device_families | []string | 是 | 机型族中文名列表，如 `["标准型", "高IO型", "大数据型", "计算型", "GPU型"]` |

**请求示例**：

```json
{
  "bk_biz_id": 100148,
  "region": "ap-guangzhou",
  "device_families": ["标准型", "高IO型", "大数据型", "计算型", "GPU型"]
}
```

### 2.4 响应结构

```typescript
{
  result: boolean;      // 请求成功与否
  code: number;        // 0 表示 success，>0 表示失败
  message: string;     // 请求失败返回的错误信息
  data: {
    info: IInheritedHostGroup[];  // 按机型族分组的候选列表
  };
}

// 机型族分组
interface IInheritedHostGroup {
  device_family: string;       // 机型族中文名，回显入参值
  hosts: IInheritedHost[];     // 该机型族下的候选，最多 5 条。该族无可继承机器时为空数组 []
}

// 候选固资项
interface IInheritedHost {
  bk_asset_id: string;          // 固资号
  bk_host_innerip: string;      // 内网 IP
  bk_cloud_inst_id: string;    // 云主机实例 ID
  device_type: string;          // 机型
  instance_charge_type: string; // 实例计费模式。PREPAID：包年包月；POSTPAID_BY_HOUR：按量计费
  billing_start_time: string;   // 套餐计费起始时间，RFC3339 格式
  billing_expire_time: string;  // 套餐计费到期时间，RFC3339 格式。按量计费固资无到期时间
  charge_months: number;        // 剩余月数（当前时间到套餐到期时间）
  is_recommended: boolean;      // 是否为推荐项
}
```

**响应示例**：

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {
    "info": [
      {
        "device_family": "标准型",
        "hosts": [
          {
            "bk_asset_id": "TC241120001357",
            "bk_host_innerip": "1.1.1.1",
            "bk_cloud_inst_id": "ins-0a1b2c3d",
            "device_type": "S5.LARGE8",
            "instance_charge_type": "PREPAID",
            "billing_start_time": "2024-11-20T10:15:30+08:00",
            "billing_expire_time": "2027-05-20T10:15:30+08:00",
            "charge_months": 10,
            "is_recommended": true
          },
          {
            "bk_asset_id": "TC250305002468",
            "bk_host_innerip": "2.2.2.2",
            "bk_cloud_inst_id": "ins-4e5f6a7b",
            "device_type": "S5.2XLARGE16",
            "instance_charge_type": "PREPAID",
            "billing_start_time": "2025-03-05T09:00:00+08:00",
            "billing_expire_time": "2027-03-05T09:00:00+08:00",
            "charge_months": 7,
            "is_recommended": false
          }
        ]
      },
      {
        "device_family": "GPU型",
        "hosts": []
      }
    ]
  }
}
```

### 2.5 响应字段说明

**顶层**：

| 参数名称 | 参数类型 | 描述 |
| --- | --- | --- |
| result | bool | 请求成功与否。true:请求成功；false:请求失败 |
| code | int | 错误编码。0 表示 success，>0 表示失败错误 |
| message | string | 请求失败返回的错误信息 |
| data | object | 响应数据 |

**data.info[]**：

| 参数名称 | 参数类型 | 描述 |
| --- | --- | --- |
| device_family | string | 机型族中文名，回显入参值 |
| hosts | object array | 该机型族下的候选，最多 5 条。该族无可继承机器时为空数组 `[]`（不为 `null`、不省略该分组） |

**data.info[].hosts[]**：

| 参数名称 | 参数类型 | 描述 | 展示位置 |
| --- | --- | --- | --- |
| bk_asset_id | string | 固资号 | 表格「固资号」列 |
| bk_host_innerip | string | 内网 IP | 表格「IP」列 |
| bk_cloud_inst_id | string | 云主机实例 ID | — |
| device_type | string | 机型 | 表格「机型」列 |
| instance_charge_type | string | 实例计费模式。PREPAID：包年包月；POSTPAID_BY_HOUR：按量计费 | 表格「计费模式」列 |
| billing_start_time | string | 套餐计费起始时间，RFC3339 格式 | 表格「开始时间」列 |
| billing_expire_time | string | 套餐计费到期时间，RFC3339 格式。按量计费固资无到期时间 | 表格「结束时间」列 |
| charge_months | int | 剩余月数（当前时间到套餐到期时间） | 表格「计费模式（剩余时长）」列 |
| is_recommended | bool | 是否为推荐项 | 表格「推荐」badge |

### 2.6 前端处理逻辑

1. **接口调用时机**：下拉面板展开 + 可用区已选中（非"全部"）
2. **入参 device_families**：固定传 5 个机型族 `["标准型", "高IO型", "大数据型", "计算型", "GPU型"]`
3. **后端已分组**：响应 `data.info` 按 `device_family` 分组，顺序与入参一致
4. **后端已排序**：每族按已使用时长由长到短最多返回 5 条
5. **后端已标记推荐**：`is_recommended` 字段标记推荐项
6. **Tab 候选数量**：每个分组 `hosts.length`
7. **候选数为 0 的 Tab**：直接隐藏（`hosts` 为空数组）
8. **Tab 无设备**：展示文案「该机型族没有符合要求的CVM实例」
9. **自动回填**：有标准型优先标准型第一个（`info[0].hosts[0]`），没有标准型取第一个非空分组的第一个数据回填
10. **回填后自动校验**：调用已有 `getInheritCvm` 接口
11. **接口选择**：根据入口选择接口 A（有 bk_biz_id 路径参数）或接口 B（无路径参数，body 传 bk_biz_id）

### 2.7 错误码

| code | 含义 | 前端处理 |
| --- | --- | --- |
| 0 | 成功 | 正常展示候选列表 |
| >0 | 失败 | 展示错误提示，下拉面板展示空态 |

### 2.8 边界场景

| 场景 | 处理 |
| --- | --- |
| 可用区选到「全部」 | 不调用此接口，清空固资号 + 解锁 |
| 接口返回所有分组 hosts 为空 | 所有 Tab 隐藏，展示空态 |
| 接口加载中 | 展示 loading 状态 |
| 接口失败 | 展示错误提示 |
| 切换可用区 | 重新调用，覆盖之前选择 |
| 切换机型族 Tab | 不重新调用接口（前端过滤已有数据） |
| 按量计费固资无到期时间 | `billing_expire_time` 为空，展示 `--` |

---

## 三、接口调用流程

```
可用区选中（非"全部")
  └─> 根据入口选择接口 A 或 B
      └─> 调用 inherited_hosts/list(bk_biz_id, region, device_families)
          └─> 返回按机型族分组的候选列表（每组最多 5 条，后端已排序+标记推荐）
              └─> 前端按 device_family 映射 Tab
                  └─> Tab 展示候选数量（0 隐藏）
                      └─> 自动回填（有标准型优先，否则取第一个非空分组）
                          └─> 回填后自动调用 getInheritCvm 校验
                              └─> 校验成功 → 锁定计费+时长
                              └─> 校验失败 → 提示原因
```

---

## 四、接口选择策略

> 根据调用入口决定使用接口 A 还是 B。

| 入口 | 接口 | URL | bk_biz_id 传递方式 |
| --- | --- | --- | --- |
| 资源管理（业务视角） | 接口 A | `POST /api/v1/woa/bizs/{bk_biz_id}/rolling_servers/inherited_hosts/list` | 路径参数 |
| 服务请求（自研云） | 接口 B | `POST /api/v1/woa/rolling_servers/inherited_hosts/list` | body 参数 |

> **判断依据**：与已有 `getInheritCvm` 的 `resolveBizApiPath` 逻辑一致——有 `bk_biz_id` 时用接口 A，无 `bk_biz_id` 时用接口 B。

---

## 五、类型定义（前端）

> 新增类型定义位置：`src/store/cvm/device.ts`

```typescript
// 候选固资列表请求参数
export interface IInheritedHostListReq {
  bk_biz_id?: number;      // 接口 B 在 body 传；接口 A 不传（路径参数）
  region: string;          // 云地域标识
  device_families: string[]; // 机型族中文名列表
}

// 机型族分组
export interface IInheritedHostGroup {
  device_family: string;       // 机型族中文名
  hosts: IInheritedHost[];     // 该族候选列表（最多 5 条）
}

// 候选固资项
export interface IInheritedHost {
  bk_asset_id: string;          // 固资号
  bk_host_innerip: string;      // 内网 IP
  bk_cloud_inst_id: string;    // 云主机实例 ID
  device_type: string;          // 机型
  instance_charge_type: string; // 计费模式 PREPAID/POSTPAID_BY_HOUR
  billing_start_time: string;   // 计费起始时间 RFC3339
  billing_expire_time: string;  // 计费到期时间 RFC3339（按量计费无）
  charge_months: number;        // 剩余月数
  is_recommended: boolean;      // 是否推荐
}
```