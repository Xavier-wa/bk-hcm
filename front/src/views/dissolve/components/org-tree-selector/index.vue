<script setup lang="ts">
import { ref, shallowRef, computed, watch, nextTick } from 'vue';
import { TreeSelect, Tag } from 'tdesign-vue-next';
import http from '@/http';

interface ITreeItem {
  id: number;
  name: string;
  full_name?: string;
  tof_dept_id?: number;
  has_children?: boolean;
  children?: ITreeItem[];
}

const model = defineModel<number[]>();
const loading = ref(false);

/** TreeSelect data：直接使用 API 返回的原始数据，通过 keys 映射字段 */
const treeData = shallowRef<ITreeItem[]>([]);

/** keys：告知 TreeSelect API 数据字段映射 */
const keys = { value: 'id', label: 'name', children: 'children' };

/** tof_dept_id → ITreeItem[] 叶子节点（支持同名 tof_dept_id 多节点） */
const leafMap = shallowRef<Map<number, ITreeItem[]>>(new Map());
/** id → ITreeItem 全量节点查询 */
const nodeMap = shallowRef<Map<number, ITreeItem>>(new Map());
/** label(name) → full_name：用于 tooltip 显示全路径 */
const nameToFullName = shallowRef<Map<string, string>>(new Map());

/** TreeSelect v-model（number[]，对应 API 的 id 字段） */
const treeValue = ref<number[]>([]);
let syncingFromTree = false;

/** treeProps：透传 Tree 组件的属性 */
const treeProps = computed(() => ({
  valueMode: 'parentFirst' as const,
  expandLevel: 1,
  maxHeight: 320,
  // tof_dept_id 为 0 的节点仅做展示，禁用选择
  disableCheck: (node: any) => node.data?.tof_dept_id === 0,
}));

// ==================== 数据加载 ====================

/** 构建 Map：leafMap（tof_dept_id→ITreeItem[]，支持同名 tof）、nodeMap（id→节点，全量）、nameToFullName */
const buildMaps = (nodes: ITreeItem[]) => {
  const lMap = new Map<number, ITreeItem[]>();
  const nMap = new Map<number, ITreeItem>();
  const nameMap = new Map<string, string>();
  const walk = (items: ITreeItem[]) => {
    for (const node of items) {
      nMap.set(node.id, node);
      if (!node.children?.length && node.tof_dept_id) {
        if (!lMap.has(node.tof_dept_id)) lMap.set(node.tof_dept_id, []);
        lMap.get(node.tof_dept_id)!.push(node);
      }
      if (node.full_name && node.name) nameMap.set(node.name, node.full_name);
      if (node.children?.length) walk(node.children);
    }
  };
  walk(nodes);
  return { lMap, nMap, nameMap };
};

const getOrg = async () => {
  loading.value = true;
  try {
    const res = await http.post('/api/v1/woa/metas/org_topos/list', { view: 'ieg' });
    const { children } = res.data as { children: ITreeItem[] };
    treeData.value = children;
    const { lMap, nMap, nameMap } = buildMaps(children);
    leafMap.value = lMap;
    nodeMap.value = nMap;
    nameToFullName.value = nameMap;

    if (model.value?.length) {
      await nextTick();
      syncModelToTree();
    }
  } finally {
    loading.value = false;
  }
};

const maxTagCount = 10;

// ==================== 折叠项 tooltip ====================

const formatCollapsed = (items: any[], count: number) => {
  const getFullName = (label: string) => nameToFullName.value.get(label) || label;
  if (items.length > maxTagCount) {
    return items.slice(0, maxTagCount).map(getFullName).join('、').concat(`…${count}个`);
  }
  return items.map(getFullName).join('、');
};

// ==================== TreeSelect ↔ model 双向同步 ====================

/**
 * treeValue → model：将 TreeSelect 值转为 tof_dept_id[]
 * parentFirst 模式下选中父节点时 treeValue 只有父节点 id，需递归收集后代叶子 tof_dept_id
 */
watch(
  treeValue,
  (ids) => {
    const tofIds = new Set<number>();
    ids.forEach((id) => {
      const node = nodeMap.value.get(id);
      if (!node) return;
      // 叶子节点：直接取 tof_dept_id
      if (!node.children?.length && node.tof_dept_id) {
        tofIds.add(node.tof_dept_id);
      } else if (node.children?.length) {
        // 父节点：递归收集所有后代叶子 tof_dept_id（兼容 parentFirst）
        const walk = (items: ITreeItem[]) => {
          for (const child of items) {
            if (!child.children?.length && child.tof_dept_id) {
              tofIds.add(child.tof_dept_id);
            }
            if (child.children?.length) walk(child.children);
          }
        };
        walk(node.children);
      }
    });

    syncingFromTree = true;
    model.value = [...tofIds];
    // 通过 id 构建 Set（唯一），用于 getCoveringIds 按唯一节点比较
    const selectedIdSet = new Set<number>();
    ids.forEach((id) => {
      const node = nodeMap.value.get(id);
      if (!node) return;
      // 收集该节点下所有可选叶子 id
      if (!node.children?.length && node.tof_dept_id) {
        selectedIdSet.add(node.id);
      } else if (node.children?.length) {
        const walk = (items: ITreeItem[]) => {
          for (const child of items) {
            if (!child.children?.length && child.tof_dept_id) selectedIdSet.add(child.id);
            if (child.children?.length) walk(child.children);
          }
        };
        walk(node.children);
      }
    });
    const covering = getCoveringIds(treeData.value, selectedIdSet);
    if (covering.length !== treeValue.value.length || covering.some((id, i) => id !== treeValue.value[i])) {
      treeValue.value = covering;
    }
    nextTick(() => {
      syncingFromTree = false;
    });
  },
  { deep: true },
);

/** 判断某节点的所有后代可选叶子 id 是否都在 selectedIdSet 中（tof_dept_id: 0 不计入） */
const allLeavesSelected = (node: ITreeItem, selectedIdSet: Set<number>): boolean => {
  if (!node.children?.length) {
    if (!node.tof_dept_id) return true;
    return selectedIdSet.has(node.id);
  }
  return node.children.every((child) => allLeavesSelected(child, selectedIdSet));
};

/** 判断节点是否包含至少一个可选的叶子（tof_dept_id > 0） */
const hasSelectableLeaf = (node: ITreeItem): boolean => {
  if (!node.children?.length) return !!node.tof_dept_id;
  return node.children.some(hasSelectableLeaf);
};

/**
 * 将选中的叶子 id Set 压缩为最小覆盖的 node id[]：
 * 如果某父节点的所有后代可选叶子都在 selectedIdSet 中，只返回父节点 id（parentFirst 对齐）
 */
const getCoveringIds = (nodes: ITreeItem[], selectedIdSet: Set<number>): number[] => {
  const result: number[] = [];
  for (const node of nodes) {
    if (allLeavesSelected(node, selectedIdSet) && hasSelectableLeaf(node)) {
      result.push(node.id);
    } else if (node.children?.length) {
      result.push(...getCoveringIds(node.children, selectedIdSet));
    }
  }
  return result;
};

/** model → TreeSelect：将 tof_dept_id[] 转为 node id[] 的 Set，再压缩为最小覆盖 id */
const syncModelToTree = () => {
  if (!model.value?.length) {
    treeValue.value = [];
    return;
  }
  if (!nodeMap.value.size) return;
  // 将 tof_dept_id[] 转为唯一的 leaf id Set（支持同名 tof 映射到多个节点）
  const selectedIdSet = new Set<number>();
  model.value.forEach((tofId) => {
    leafMap.value.get(tofId)?.forEach((n) => selectedIdSet.add(n.id));
  });
  treeValue.value = getCoveringIds(treeData.value, selectedIdSet);
};

watch(model, () => {
  if (syncingFromTree) return;
  syncModelToTree();
});

// ==================== 搜索过滤 ====================

/** 搜索过滤：keys 将 name 映射为 label，filter 收到的 option 应通过 option.label/option.name 读取 */
// eslint-disable-next-line @typescript-eslint/no-explicit-any
const filterMethod = (filterWords: string, option: any) => {
  if (!filterWords) return true;
  const kw = filterWords.toLowerCase();
  const name = String(option.label || option.name || '').toLowerCase();
  const fullName = String(option.full_name || option._raw?.full_name || '').toLowerCase();
  return name.includes(kw) || fullName.includes(kw);
};

// ==================== 清除 ====================

const clear = () => {
  syncingFromTree = true;
  model.value = [];
  treeValue.value = [];
  nextTick(() => {
    syncingFromTree = false;
  });
};

getOrg();
defineExpose({ clear });
</script>

<template>
  <div class="org-tree-selector">
    <TreeSelect
      v-model="treeValue"
      :data="treeData"
      :keys="keys"
      :multiple="true"
      :filterable="true"
      :filter="filterMethod"
      :clearable="true"
      :min-collapsed-num="1"
      :tree-props="treeProps"
      :popup-props="{ overlayClassName: ['t-select__dropdown', 'narrow-scrollbar', 'org-tree-selector-popup'] }"
      placeholder="请选择或输入名称查找"
    >
      <template v-if="loading" #suffixIcon>
        <bk-loading loading mode="spin" size="mini" />
      </template>
      <template #collapsedItems="{ collapsedSelectedItems, count }">
        <Tag
          v-bk-tooltips="{
            content: formatCollapsed(collapsedSelectedItems, count),
          }"
        >
          +{{ count }}
        </Tag>
      </template>
    </TreeSelect>
  </div>
</template>

<style lang="scss" scoped>
/* stylelint-disable selector-class-pattern */

.org-tree-selector {
  width: 100%;

  :deep(.t-input__prefix) {
    flex-wrap: nowrap;
    overflow: hidden;
    text-overflow: ellipsis;
  }
}

:deep(.t-input) {
  --td-bg-color-component: #f0f1f5;
  --td-text-color-primary: #63656e;

  display: inline-flex;
  align-items: stretch;
  width: 100%;
  height: 32px;
  font-size: 12px;
  border: 1px solid #c4c6cc;
  border-radius: 2px;
  transition: all 0.3s;
  padding-left: 10px;
  cursor: pointer;

  &:hover {
    border-color: #979ba5;
  }
}

:deep(.t-input__inner) {
  cursor: pointer;
}

:deep(.t-input--focused) {
  border-color: #3a84ff;
  outline: 0;
  box-shadow: 0 0 3px 0 #a3c5fd;
}

:deep(.t-fake-arrow) {
  transition: transform 0.3s cubic-bezier(0.4, 0, 0.2, 1);
  scale: 0.9;

  path {
    stroke: #979ba5 !important;
  }
}

:deep(.t-fake-arrow--active) {
  transform: rotate(180deg);
}
</style>

<!-- 弹出框渲染在 body 下，不能用 scoped，通过 popupProps.overlayClassName 定位 -->
<style lang="scss">
.org-tree-selector-popup {
  --td-text-color-secondary: #979ba5;
  --td-brand-color-light: #ebf2ff;
  --td-comp-margin-xxl: 16px;
  --td-text-color-brand: #63656e;
  --td-text-color-primary: #63656e;

  .t-icon {
    scale: 1.5;
  }

  .t-popup__content {
    overflow: hidden !important;
  }
}
</style>
