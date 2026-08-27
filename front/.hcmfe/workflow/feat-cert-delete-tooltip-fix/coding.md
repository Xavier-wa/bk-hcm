# coding - 证书-删除按钮tips错误

## 现象
证书管理页（资源运营视角）删除按钮的 tooltip 错误：
- 资源页下、**已分配业务**的证书（`bk_biz_id !== -1`，按钮被禁用）→ 悬停看不到任何解释。
- 资源页下、**未分配业务**的证书（`bk_biz_id === -1`，按钮可点）→ 悬停却显示「该证书已分配业务, 仅可在业务下操作」→ 误导性错误提示。

## 根因
`front/src/views/business/cert-manager/index.tsx` 操作列删除按钮：
- tooltip 的 `disabled` 与按钮 `disabled` 用了**同一条件** `isResourcePage && data.bk_biz_id !== -1`。
- `v-bk-tooltips` 通过 `el.addEventListener('mouseenter', ...)` 绑定（bkui-vue `lib/directives/index.js:1107`）。
- 按钮渲染为原生 `<button disabled>`，原生 disabled 元素**不触发 mouseenter**，故禁用按钮上 tooltip 永远弹不出。

## 修复
将 `v-bk-tooltips` 移到外层 `<span>`（span 能接收鼠标事件），并反转 tooltip 的 `disabled` 条件：

```tsx
<span
  v-bk-tooltips={{
    content: '该证书已分配业务, 仅可在业务下操作',
    disabled: !(isResourcePage && data.bk_biz_id !== -1),
  }}>
  <Button
    text
    theme='primary'
    onClick={() => handleDeleteCert(data)}
    disabled={noPerm || (isResourcePage && data.bk_biz_id !== -1)}>
    删除
  </Button>
</span>
```

效果：
- 资源页 + 已分配业务（按钮禁用）→ tooltip 显示说明，解释为何不可删。
- 资源页 + 未分配业务（按钮可点）→ tooltip 隐藏，不再显示误导性文案。
- `noPerm` 禁用场景 tooltip 仍隐藏（该文案仅描述业务归属，与权限无关）。

## 涉及文件
- `front/src/views/business/cert-manager/index.tsx`（操作列 render，L72-92）
