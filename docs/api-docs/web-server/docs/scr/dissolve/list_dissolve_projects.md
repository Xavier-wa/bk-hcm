### 描述

- 该接口提供版本：v1.9.2.2+。
- 该接口所需权限：无。
- 该接口功能描述：查询裁撤系统的裁撤项目列表，供配置裁撤项目时选择。

### URL

GET /api/v1/woa/dissolve/projects

### 输入参数

无

### 调用示例

```json
{}
```

### 响应示例

```json
{
  "code": 0,
  "message": "",
  "data": [
    {
      "id": 100,
      "projectName": "2026年第一批裁撤",
      "projectType": "machine"
    }
  ]
}
```

### 响应参数说明

| 参数名称    | 参数类型   | 描述   |
|---------|--------|------|
| code    | int32  | 状态码  |
| message | string | 请求信息 |
| data    | array  | 裁撤项目列表 |

#### data[n]

| 参数名称        | 参数类型   | 描述                                  |
|-------------|--------|-------------------------------------|
| id          | int    | 裁撤项目ID                              |
| projectName | string | 项目名称                                |
| projectType | string | 项目类型，枚举值：machine/yunxi/lingxing     |
