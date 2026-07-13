<script setup lang="ts">
import { ref, shallowRef, computed, watch, nextTick } from 'vue';
import debounce from 'lodash/debounce';

export interface ITreeItem {
  id: number | string;
  name: string;
  full_name?: string;
  has_children?: 0 | 1 | boolean;
  children?: ITreeItem[];
  level?: number;
  tof_dept_id?: number;
}

export interface ITreeSelectorProps {
  data: ITreeItem[] | (() => Promise<ITreeItem[]>);
  multiple?: boolean;
  disabled?: boolean;
  showOnInit?: boolean;
  dropdownMaxHeight?: number;
  collapseTags?: boolean;
  placeholder?: string;
  searchPlaceholder?: string;
}

defineOptions({
  name: 'tree-selector',
});

const model = defineModel<string | number | (string | number)[]>();

const modelChecked = defineModel<ITreeItem | ITreeItem[]>('checked');

const props = withDefaults(defineProps<ITreeSelectorProps>(), {
  data: () => [] as ITreeItem[],
  multiple: true,
  disabled: false,
  dropdownMaxHeight: 320,
  collapseTags: true,
  showOnInit: false,
});

const selectRef = ref();
const treeRef = ref();

const loading = ref(false);

// 性能优化：checked 改为 ID Set 存储，避免数千对象进入响应式 deep watch
// shallowRef 确保 Set 引用变化才触发 watch，不递归遍历对象属性
const checked = shallowRef<Set<string | number>>(new Set());

// 全部选择的项（含半选），仅用于清空树勾选状态
const allTreeChecked = shallowRef<ITreeItem[]>([]);

const treeData = shallowRef<ITreeItem[]>([]);

// 扁平节点映射：id → ITreeItem，O(1) 查找节点完整数据
// 在 loadTree 中一次性 O(n) 遍历全树构建
const flatNodeMap = shallowRef<Map<string | number, ITreeItem>>(new Map());

const buildFlatNodeMap = (nodes: ITreeItem[]): Map<string | number, ITreeItem> => {
  const map = new Map<string | number, ITreeItem>();
  const walk = (items: ITreeItem[]) => {
    for (const node of items) {
      if (node.id !== undefined) {
        map.set(node.id, node);
      }
      if (node.children?.length) {
        walk(node.children);
      }
    }
  };
  walk(nodes);
  return map;
};

const localMultiple = computed(() => {
  if (Array.isArray(model.value) && model.value.length > 1 && !props.multiple) {
    return true;
  }
  return props.multiple;
});

const selectAction = computed(() => (localMultiple.value ? 'setChecked' : 'setSelect'));

// 从 checked ID Set 中取前 10 个 ID，用 flatNodeMap 查 full_name 显示
const selectValue = computed(() => {
  const rootNode = treeData.value?.[0];
  if (
    treeRef.value?.isRootNode(rootNode) &&
    treeRef.value?.isNodeChecked(rootNode) &&
    !treeRef.value?.getNodeAttr(rootNode, '__is_indeterminate')
  ) {
    return ['全部'];
  }

  const ids = [...checked.value];
  if (ids.length > 10) {
    return ids
      .slice(0, 10)
      .map((id) => flatNodeMap.value.get(id)?.full_name ?? '')
      .concat(`...${ids.length}个`);
  }
  return ids.map((id) => flatNodeMap.value.get(id)?.full_name ?? '');
});

// 驱动 bk-tree 的 :checked prop，Set → 数组
const checkedIds = computed(() => [...checked.value]);

// 默认选中的值
const defaultChecked = computed(() => {
  if (localMultiple.value) {
    if (model.value && !Array.isArray(model.value)) {
      return [model.value];
    }
    return model.value || [];
  }
  if (Array.isArray(model.value)) {
    return model.value[0] || '';
  }
  return model.value || '';
});

const getTreeInternalData = () => treeRef.value.getData();

const loadTree = async (defaultCheckedIds: ITreeItem['id'] | ITreeItem['id'][]) => {
  // 默认先拉出第一层级
  loading.value = true;
  const topData = typeof props.data === 'function' ? await props.data() : props.data;
  loading.value = false;

  treeData.value = topData;
  // 构建 flatNodeMap 用于后续 O(1) 查找
  flatNodeMap.value = buildFlatNodeMap(topData);

  // 处理默认选中
  if (
    (!Array.isArray(defaultCheckedIds) && defaultCheckedIds) ||
    (Array.isArray(defaultCheckedIds) && defaultCheckedIds.length)
  ) {
    const values = Array.isArray(defaultCheckedIds) ? defaultCheckedIds : [defaultCheckedIds];
    nextTick(() => {
      const treeFlatData = getTreeInternalData();
      const newChecked = new Set(checked.value);
      values.forEach((value) => {
        const checkedNode = treeFlatData.data.find((item: ITreeItem) => item.id === value);
        if (checkedNode) {
          newChecked.add(checkedNode.id);
          // 展开选中的节点及其父节点
          treeRef.value.setOpen(checkedNode, true, true);
          // 设置为选中
          treeRef.value[selectAction.value](checkedNode, true, false);
        }
      });
      checked.value = newChecked;
    });
  }

  nextTick(() => {
    // 默认展开第一层级
    if (treeData.value.length) {
      treeRef.value.setOpen(treeData.value[0], true, false);
    }
  });
};

// 将 modelChecked 值统一转为数组格式
const resolveCheckedItems = (value: ITreeItem | ITreeItem[]): ITreeItem[] => {
  if (Array.isArray(value)) return value;
  if (value) return [value];
  return [];
};

// 防循环标志：正向同步（checked → modelChecked）时置 true，避免 modelChecked watch 反向触发
let syncingFromChecked = false;

// checked 变化时同步到 model 和 modelChecked
// shallowRef Set 引用变化即触发，无需 deep watch
watch(checked, (idSet) => {
  const ids = [...idSet];
  model.value = localMultiple.value ? ids : ids?.[0];
  // 从 flatNodeMap 构造完整对象数组输出给 modelChecked
  const items = ids.map((id) => flatNodeMap.value.get(id)).filter(Boolean) as ITreeItem[];
  syncingFromChecked = true;
  modelChecked.value = localMultiple.value ? items : items?.[0];
  nextTick(() => {
    syncingFromChecked = false;
  });
});

// 当外部通过 v-model:checked 设置选中值时，同步到内部 checked（ID Set）
// syncingFromChecked 为 true 时跳过，避免 checked → modelChecked → checked 循环
watch(modelChecked, (value) => {
  if (syncingFromChecked) return;
  const items = resolveCheckedItems(value);
  checked.value = new Set(items.map((item) => item.id));
});

const clear = () => {
  // 直接清空 checked Set，:checked="checkedIds" prop 会驱动 bk-tree 更新所有 checkbox
  // 不再逐个调用 setChecked（遍历数千节点导致卡死）
  allTreeChecked.value = [];
  checked.value = new Set();
};

// 性能优化：用 Set.has 替代 includes，O(n+m) 替代 O(n*m)
const handleNodeChecked = (allChecked: ITreeItem[], allHalfChecked: ITreeItem[]) => {
  allTreeChecked.value = allChecked;
  const halfCheckedIdSet = new Set(allHalfChecked.map((item) => item.id));
  checked.value = new Set(allChecked.filter((item) => !halfCheckedIdSet.has(item.id)).map((item) => item.id));
};

const handleNodeSelected = (payload: { selected: boolean; node: ITreeItem }) => {
  checked.value = new Set([payload.node.id]);
  selectRef.value.hidePopover();
};

const handleSelectRemoveTag = (name: ITreeItem['full_name']) => {
  // 用 flatNodeMap 反查 id，替代遍历 checked 数组
  let targetId: string | number | undefined;
  for (const id of checked.value) {
    const node = flatNodeMap.value.get(id);
    if (node?.full_name === name) {
      targetId = id;
      break;
    }
  }
  if (targetId === undefined) return;

  const newChecked = new Set(checked.value);
  newChecked.delete(targetId);
  checked.value = newChecked;

  // 使用 tree 组件方法时操作对象需要是 treeNode
  const treeFlatData = getTreeInternalData();
  const treeNode = treeFlatData.data.find((item: ITreeItem) => item.id === targetId);
  if (treeNode) {
    treeRef.value[selectAction.value](treeNode, false, false);
  }
};

const handleClear = () => {
  clear();
};

const searchValue = ref('');

const handleSearch = debounce(async (value: string) => {
  searchValue.value = value;

  // 展开搜索结果
  nextTick(() => {
    const treeFlatData = getTreeInternalData();
    const matched = treeFlatData.data.filter((item: ITreeItem) => treeRef.value.isNodeMatched(item));
    treeRef.value.setOpen(matched.pop(), true, true);
  });
}, 200);

loadTree(defaultChecked.value);

defineExpose({
  clear,
});
</script>

<template>
  <div class="tree-selector">
    <bk-select
      ref="selectRef"
      :collapse-tags="false"
      :disabled="disabled"
      :loading="loading"
      :model-value="selectValue"
      :multiple="localMultiple"
      :placeholder="placeholder"
      :scroll-height="dropdownMaxHeight"
      :search-placeholder="searchPlaceholder"
      :show-on-init="showOnInit"
      display-key="name"
      multiple-mode="'default'"
      custom-content
      filterable
      @clear="handleClear"
      @search-change="handleSearch"
      @tag-remove="handleSelectRemoveTag"
    >
      <bk-tree
        ref="treeRef"
        class="tree-selector-tree"
        :check-strictly="false"
        :checked="checkedIds"
        :data="treeData"
        :expand-all="false"
        :height="dropdownMaxHeight"
        :level-line="false"
        :node-content-action="localMultiple ? ['click'] : ['selected']"
        :selectable="!localMultiple"
        :show-checkbox="localMultiple"
        :show-node-type-icon="false"
        :virtual-render="true"
        :search="searchValue"
        children="children"
        label="name"
        node-key="id"
        @node-checked="handleNodeChecked"
        @node-selected="handleNodeSelected"
      />
    </bk-select>
  </div>
</template>

<style lang="scss">
.tree-selector {
  font-size: 12px;
}

.tree-selector-tree {
  color: #63656e;

  .bk-node-action {
    display: inline-flex;
    align-items: center;
  }

  .bk-node-content {
    gap: 4px;

    & > span {
      display: flex;
      align-items: center;
    }
  }
}
</style>
