## 1. CLB extension 同步

- [x] 1.1 在 `TCloudClbExtension` 增加 `Exclusive`、`ClusterIds`、`ClusterTag`
- [x] 1.2 `convertTCloudExtension` 赋值 `Exclusive`（null 收成 0）、`ClusterIds`、`ClusterTag`，并补写 `Egress`
- [x] 1.3 `isLBExtensionChange` 比较 `exclusive`（null 视为 0）、`cluster_ids`、`cluster_tag`
