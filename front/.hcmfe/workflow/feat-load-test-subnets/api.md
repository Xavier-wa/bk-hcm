# API: 自研云-主机申领-网络信息增加压测子网配置类型

> 对应 Design: `.hcmfe/workflow/feat-load-test-subnets/design.md`

---

## 1. 新增接口

### 1.1 查询压测子网配置

```
GET /api/v1/woa/config/load_test_subnets
```

- **版本要求**: v9.9.9+
- **权限**: 无
- **入参**: 无
- **描述**: 返回所有地域的压测 VPC/子网索引映射

#### 响应

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {
    "ap-nanjing": {
      "vpc-aaa111": ["subnet-s22y2418", "subnet-pqnrqykq"],
      "vpc-bbb222": ["subnet-7g6ctd9i", "subnet-ncdnis5q"]
    },
    "ap-shanghai": {
      "vpc-ccc333": ["subnet-99t4p51x", "subnet-3cv8ulcl"]
    }
  }
}
```

#### 响应参数

| 参数 | 类型 | 描述 |
|------|------|------|
| `data` | `object` | 压测子网映射，外层 key = region，中层 key = vpc_id，内层 value = subnet_id 列表 |
| `data.{region}` | `object` | 该地域下压测 VPC → 子网列表的映射 |
| `data.{region}.{vpc_id}` | `string[]` | 该 VPC 下的压测子网 ID 列表 |

#### 配置不存在时

```json
{
  "result": true,
  "code": 0,
  "message": "success",
  "data": {}
}
```

`data` 为空对象 `{}`，此时判定当前无任何压测子网配置。

---

## 2. 与现有接口的关系

| 接口 | 改动 | 说明 |
|------|------|------|
| `POST /api/v1/woa/config/findmany/config/cvm/vpc` | **无改动** | 保持原有请求逻辑，由 VpcSelector 内部调用 |
| `POST /api/v1/woa/config/findmany/config/cvm/subnet` | **无改动** | 保持原有请求逻辑，由 SubnetSelector 内部调用 |
| `GET /api/v1/woa/config/load_test_subnets` | **新增** | 提供压测索引，由 Panel 调用 |

---

## 3. 调用时机

| 时机 | 调用 |
|------|------|
| 组件挂载 | `GET /load_test_subnets`（一次获取全部索引，缓存在内存） |
| vpc 选中后 | `POST /cvm/subnet`（由 SubnetSelector 内部调用） |
| 切换 configType | 不重新请求，仅切换过滤逻辑 |

---

## 4. 压测可用性判定

索引中当前 region 对应的 object 为空（`!index[region]`）或所有 vpc 下的子网列表都为空 → 禁用「用于压测」选项。

---

## 5. 错误处理

| 场景 | 行为 |
|------|------|
| `GET /load_test_subnets` 请求失败 | 降级：pressureIndex 保持空，filter 返回 undefined，VPC/子网展示全部原始数据，压测选项禁用 |
| `GET /load_test_subnets` 返回 `data: {}` | 判定无压测配置，压测选项禁用 |
