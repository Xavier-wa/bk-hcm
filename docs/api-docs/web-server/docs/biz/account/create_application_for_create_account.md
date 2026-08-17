### 描述

- 该接口提供版本：v9.9.9+。
- 该接口所需权限：业务访问。
- 该接口功能描述：创建用于创建账号的申请。
- 说明：当 `type=registration`（登记账号）时走免审直连交付，不创建 ITSM 审批单、不落 HCM 申请单，同步创建账号；当 `type=resource` / `security_audit` 时仍走 ITSM 审批并创建申请单。
- path 中的 `{bk_biz_id}` 是调用方业务上下文，用于「业务访问」鉴权，须为真实业务 ID（大于 0）。body 中的 `bk_biz_id` 是账号管理业务：仅 `type=resource` 时必填，且必须与 path 一致；`type=registration` / `security_audit` 时不允许传递（缺省或为 0）。

### URL

POST /api/v1/cloud/bizs/{bk_biz_id}/applications/types/add_account

### 输入参数

| 参数名称          | 参数类型         | 必选 | 描述                                                        |
|---------------|--------------|----|-----------------------------------------------------------|
| vendor        | string       | 是  | 云厂商（枚举值：tcloud、aws、huawei、gcp、azure）                      |
| name          | string       | 是  | 名称                                                        |
| managers      | string array | 是  | 账号管理者                                                     |
| security_managers | string array | 否  | 账号安全管理者，当云厂商为tcloud时，该字段必填                                |
| type          | string       | 是  | 账号类型 (枚举值：resource:资源账号、registration:登记账号、security_audit:安全审计账号) |
| site          | string       | 是  | 站点（枚举值：china:中国站、international:国际站）                       |
| memo          | string       | 否  | 备注                                                        |
| bk_biz_id     | int64        | 否  | 管理业务。仅 type=resource 时必填，且须与 path 中的 bk_biz_id 一致、为真实管理业务；type=registration / security_audit 时不允许传递（缺省或为 0） |
| usage_biz_ids | int64 array  | 是  | 使用业务，非资源账号的该字段长度必须为1                                      |
| extension     | object       | 是  | 混合云差异字段                                                   |
| remark        | string       | 否  | 单据备注                                                      |

##### extension[tcloud]

| 参数名称                  | 参数类型   | 描述     |
|-----------------------|--------|--------|
| cloud_main_account_id | string | 云主账户ID |

##### extension[aws]

| 参数名称               | 参数类型   | 必选 | 描述      |
|--------------------|--------|----|---------|
| cloud_account_id   | string | 是  | 云账户ID   |
| cloud_iam_username | string | 是  | 云iam用户名 |
| cloud_secret_id    | string | 否  | 云加密ID   |
| cloud_secret_key   | string | 否  | 云密钥     |

##### extension[huawei]

| 参数名称                   | 参数类型   | 必选 | 描述       |
|------------------------|--------|----|----------|
| cloud_sub_account_id   | string | 是  | 云子账户ID   |
| cloud_sub_account_name | string | 是  | 云子账户名称   |
| cloud_iam_user_id      | string | 是  | 云iam用户ID |
| cloud_iam_username     | string | 是  | 云iam用户名  |
| cloud_secret_id        | string | 否  | 云加密ID    |
| cloud_secret_key       | string | 否  | 云密钥      |

##### extension[gcp]

| 参数名称                       | 参数类型   | 必选 | 描述      |
|----------------------------|--------|----|---------|
| email                      | string | 否  | 邮箱地址    |
| cloud_project_id           | string | 是  | 云项目ID   |
| cloud_project_name         | string | 是  | 云项目名称   |
| cloud_service_account_id   | string | 否  | 云服务账户ID |
| cloud_service_account_name | string | 否  | 云服务账户名称 |
| cloud_service_secret_id    | string | 否  | 云服务加密ID |
| cloud_service_secret_key   | string | 否  | 云服务密钥   |

##### extension[azure]

| 参数名称                    | 参数类型   | 必选 | 描述     |
|-------------------------|--------|----|--------|
| display_name_name       | string | 否  | 展示名称   |
| cloud_tenant_id         | string | 是  | 云租户ID  |
| cloud_subscription_id   | string | 是  | 云订阅ID  |
| cloud_subscription_name | string | 是  | 云订阅名称  |
| cloud_application_id    | string | 否  | 云应用ID  |
| cloud_application_name  | string | 否  | 云应用名称  |
| cloud_client_secret_key | string | 否  | 云客户端密钥 |

### 调用示例

#### TCloud 资源账号（resource）

```json
{
  "vendor": "tcloud",
  "name": "jim",
  "managers": [
    "hcm"
  ],
  "security_managers": [
    "hcm"
  ],
  "type": "resource",
  "site": "china",
  "bk_biz_id": 1010011010,
  "usage_biz_ids": [
    1010011010
  ],
  "extension": {
    "cloud_main_account_id": "main-xxxxxx"
  },
  "memo": ""
}
```

> path `{bk_biz_id}` 与 body `bk_biz_id` 必须一致，且为真实管理业务。

#### TCloud 登记账号（registration）

```json
{
  "vendor": "tcloud",
  "name": "hcm-reg-demo",
  "managers": [
    "hcm"
  ],
  "type": "registration",
  "site": "china",
  "usage_biz_ids": [
    1010011010
  ],
  "extension": {
    "cloud_main_account_id": "main-xxxxxx",
    "cloud_sub_account_id": "sub-xxxxxx"
  },
  "memo": ""
}
```

> path 使用真实业务 ID 做「业务访问」鉴权；body **不传** `bk_biz_id`（或为 0）。`type=security_audit` 同样不传 body `bk_biz_id`。

#### Aws

```json
{
  "vendor": "tcloud",
  "name": "jim",
  "managers": [
    "hcm"
  ],
  "type": "resource",
  "site": "china",
  "bk_biz_id": 1010011010,
  "usage_biz_ids": [
    1010011010
  ],
  "extension": {
    "cloud_account_id": "main-xxxxxx",
    "cloud_iam_username": "sub-xxxxxx",
    "cloud_secret_id": "xxxxx",
    "cloud_secret_key": "xxxxxxxx"
  },
  "memo": ""
}
```

#### HuaWei

```json
{
  "vendor": "tcloud",
  "name": "jim",
  "managers": [
    "hcm"
  ],
  "type": "resource",
  "site": "china",
  "bk_biz_id": 1010011010,
  "usage_biz_ids": [
    1010011010
  ],
  "extension": {
    "cloud_main_account_name": "main-xxxxxx",
    "cloud_sub_account_id": "sub-xxxxxx",
    "cloud_sub_account_name": "xxxxx",
    "cloud_iam_user_id": "xxxxxxxx",
    "cloud_iam_username": "xxxxxxxx",
    "cloud_secret_id": "xxxxxxxx",
    "cloud_secret_key": "xxxxxxxx"
  },
  "memo": ""
}
```

#### Gcp

```json
{
  "vendor": "tcloud",
  "name": "jim",
  "managers": [
    "hcm"
  ],
  "type": "resource",
  "site": "china",
  "bk_biz_id": 1010011010,
  "usage_biz_ids": [
    1010011010
  ],
  "extension": {
    "cloud_project_id": "main-xxxxxx",
    "cloud_project_name": "sub-xxxxxx",
    "cloud_service_account_id": "xxxxx",
    "cloud_service_account_name": "xxxxxxxx",
    "cloud_service_secret_id": "xxxxxxxx",
    "cloud_service_secret_key": "xxxxxxxx"
  },
  "memo": ""
}
```

#### Azure

```json
{
  "vendor": "tcloud",
  "name": "jim",
  "managers": [
    "hcm"
  ],
  "type": "resource",
  "site": "china",
  "bk_biz_id": 1010011010,
  "usage_biz_ids": [
    1010011010
  ],
  "extension": {
    "cloud_tenant_id": "main-xxxxxx",
    "cloud_subscription_id": "sub-xxxxxx",
    "cloud_subscription_name": "xxxxx",
    "cloud_application_id": "xxxxxxxx",
    "cloud_application_name": "xxxxxxxx",
    "cloud_client_secret_id": "xxxxxxxx",
    "cloud_client_secret_key": "xxxxxxxx"
  },
  "memo": ""
}
```

### 响应示例

#### 需审批账号（resource / security_audit）

```json
{
  "result": true,
  "code": 0,
  "message": "",
  "data": {
    "id": "00000001"
  }
}
```

> 此时 `data.id` 为**申请单 ID**，可据此查询单据详情 / 跳转「我的申请」。

#### 登记账号免审（registration）

```json
{
  "result": true,
  "code": 0,
  "message": "",
  "data": {
    "id": "00000b05"
  }
}
```

> 此时 `data.id` 为**账号 ID**（不是申请单 ID）。登记账号不会创建 HCM 申请单，请勿将该 ID 当作单据 ID 使用。

#### 失败示例

```json
{
  "result": false,
  "code": 2000006,
  "message": "create account success, but add create action associate permissions failed, err: xxx"
}
```

> 常见错误码：`2000001`（InvalidParameter，参数校验失败）、`2000006`（Aborted，业务处理/交付失败）。登记账号免审失败时同步返回错误，不会生成申请单。

### 响应参数说明

| 参数名称    | 参数类型   | 描述   |
|---------|--------|------|
| result  | bool   | 请求是否成功 |
| code    | int32  | 状态码  |
| message | string | 请求信息 |
| data    | object | 响应数据；失败时通常为空或不返回业务数据 |

#### data

| 参数名称 | 参数类型   | 描述   |
|------|--------|------|
| id   | string | 业务结果 ID。`type=resource` / `security_audit` 时为申请单 ID；`type=registration` 时为新创建的账号 ID |
