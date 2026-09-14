<script setup lang="ts">
import { computed, ref, watch } from 'vue';
import { cloneDeep } from 'lodash';
import { Button } from 'bkui-vue';

import {
  type HostApplyRecommendation,
  type HostApplyRecommendValue,
  type HostApplySuborder,
} from '@/hooks/chatbot/types';
import { toSpecDisplayItems } from '@/hooks/chatbot/host-apply-display';
import { ensureDeviceMeta, type HostApplyDeviceMeta } from '@/hooks/chatbot/host-apply-device-meta';
import { useFollowScrollOnMount } from '@/hooks/chatbot/use-follow-scroll';
import ReqTypeValue from '@/components/display-value/req-type-value.vue';
import CustomMessageCard from './custom-message-card.vue';
import HostApplyAdjustDialog from './host-apply-adjust-dialog.vue';

interface Props {
  content: HostApplyRecommendValue;
  readonly?: boolean;
  selectedIndex?: number;
  // 初始定位下标（从「选择方案 + 添加到配置清单」跳转回放）：仅定位展示，不影响只读态
  initialIndex?: number;
  // F-004：他处正在操作该会话，锁定操作按钮（不进只读，仅禁用 + 顶部提示）
  locked?: boolean;
  onSelect: (index: number) => void;
  onAddToList: (suborder: HostApplySuborder) => void;
}

const props = withDefaults(defineProps<Props>(), {
  readonly: false,
  selectedIndex: -1,
  initialIndex: 0,
  locked: false,
});

useFollowScrollOnMount();

// A 的「调整配置」为本地预览编辑（无明确 agent 回传语义），仅更新当前方案展示，故持有本地副本
const localRecommendations = ref<HostApplyRecommendation[]>(cloneDeep(props.content.value.recommendations ?? []));
watch(
  () => props.content.value.recommendations,
  (recommendations) => {
    localRecommendations.value = cloneDeep(recommendations ?? []);
  },
);

// 非只读态翻页下标，初值取 initialIndex；跳转回放时该 prop 在加载后才写入，故 watch 同步定位
const pageIndex = ref(props.initialIndex);
watch(
  () => props.initialIndex,
  (index) => {
    pageIndex.value = index;
  },
);
const adjustVisible = ref(false);

const total = computed(() => localRecommendations.value.length);

// 只读态默认展示已选方案；历史回放无 selectedIndex（-1）时退化为首条
const selectedIdx = computed(() =>
  props.selectedIndex >= 0 && props.selectedIndex < total.value ? props.selectedIndex : 0,
);

const activeIndex = computed(() => (props.readonly ? selectedIdx.value : pageIndex.value));
const currentRecommendation = computed<HostApplyRecommendation | null>(
  () => localRecommendations.value[activeIndex.value] ?? null,
);

// device_type → meta；先展示编码，异步补全后刷新「机型」行
const deviceMetaMap = ref<Record<string, HostApplyDeviceMeta | null>>({});
watch(
  () => currentRecommendation.value?.suborder?.device_type,
  async (deviceType) => {
    if (!deviceType || deviceMetaMap.value[deviceType] !== undefined) return;
    const meta = await ensureDeviceMeta(deviceType);
    deviceMetaMap.value = { ...deviceMetaMap.value, [deviceType]: meta };
  },
  { immediate: true },
);

const specItems = computed(() => {
  const recommendation = currentRecommendation.value;
  if (!recommendation) return [];
  const deviceType = recommendation.suborder.device_type;
  const meta = deviceType ? deviceMetaMap.value[deviceType] : undefined;
  return toSpecDisplayItems(recommendation.suborder, meta);
});

const titleLabel = computed(() => (props.readonly ? '申领方案预览' : '申领方案推荐'));

const prevDisabled = computed(() => pageIndex.value <= 0);
const nextDisabled = computed(() => pageIndex.value >= total.value - 1);

const goPrev = () => {
  if (!prevDisabled.value) pageIndex.value -= 1;
};
const goNext = () => {
  if (!nextDisabled.value) pageIndex.value += 1;
};

const handleSelectPlan = () => {
  if (props.readonly || props.locked || !currentRecommendation.value) return;
  props.onSelect(activeIndex.value);
};

const handleAddToList = () => {
  if (props.readonly || props.locked || !currentRecommendation.value) return;
  props.onAddToList(currentRecommendation.value.suborder);
};

const handleAdjustSave = (suborder: HostApplySuborder) => {
  const recommendation = localRecommendations.value[activeIndex.value];
  if (recommendation) recommendation.suborder = suborder;
  adjustVisible.value = false;
};
</script>

<template>
  <CustomMessageCard :readonly="readonly" :locked="locked" summary-text="您已选择申领方案">
    <template #title>
      <span class="ha-title-label">{{ titleLabel }}</span>
      <span class="ha-page-badge">{{ activeIndex + 1 }} / {{ total }}</span>
    </template>

    <template v-if="total > 1 && !readonly" #header-extra>
      <Button size="small" :disabled="prevDisabled" @click="goPrev">上一个</Button>
      <Button size="small" :disabled="nextDisabled" @click="goNext">下一个</Button>
    </template>

    <!-- 参数明细键值表 -->
    <div class="ha-spec">
      <div v-for="item in specItems" :key="item.key" class="ha-spec-row">
        <span class="ha-spec-label">{{ item.label }}</span>
        <span class="ha-spec-value">
          <ReqTypeValue
            v-if="item.key === 'require_type' && currentRecommendation"
            :value="Number(currentRecommendation.suborder.require_type)"
          />
          <template v-else>{{ item.value }}</template>
        </span>
      </div>
    </div>

    <template #actions>
      <Button theme="primary" :disabled="readonly || locked" @click="handleSelectPlan">选择方案</Button>
      <Button :disabled="readonly || locked" @click="handleAddToList">添加到配置清单</Button>
      <!-- 暂不支持调整方案 -->
      <!-- <Button :disabled="readonly" @click="adjustVisible = true">调整配置</Button> -->
    </template>
  </CustomMessageCard>

  <HostApplyAdjustDialog
    v-if="currentRecommendation"
    v-model:visible="adjustVisible"
    :suborder="currentRecommendation.suborder"
    @save="handleAdjustSave"
  />
</template>

<style scoped lang="scss">
.ha-title-label {
  vertical-align: middle;
}

// 页码胶囊徽标（浅蓝底 + 蓝字 + 圆角），样式取自设计 SVG
.ha-page-badge {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  min-width: 32px;
  height: 18px;
  padding: 0 8px;
  margin-left: 8px;
  font-size: 12px;
  font-weight: 400;
  line-height: 18px;
  color: #1768ef;
  vertical-align: middle;
  background: #e1ecff;
  border-radius: 9px;
}

.ha-spec {
  overflow: hidden;
  border: 1px solid #dcdee5;
}

.ha-spec-row {
  display: flex;
  align-items: stretch;
  min-height: 36px;
  font-size: 12px;

  & + & {
    border-top: 1px solid #dcdee5;
  }

  .ha-spec-label {
    display: flex;
    flex-shrink: 0;
    align-items: center;
    justify-content: flex-end;
    width: 200px;
    padding: 0 12px;
    color: #4d4f56;
    text-align: right;
    background: #fafbfd;
    border-right: 1px solid #f0f1f5;
  }

  .ha-spec-value {
    display: flex;
    flex: 1;
    align-items: center;
    padding: 8px 12px;
    color: #313238;
    word-break: break-all;
    white-space: pre-line;
  }
}

// 窄容器（浮窗）：label 收窄并左对齐（flex 容器需改 justify-content，text-align 无效）
// 阈值与 custom-message-card.vue 保持一致（由浮窗默认宽 500 反推，见那里的说明）
@container (max-width: 460px) {
  .ha-spec-row .ha-spec-label {
    width: 100px;
    justify-content: flex-start;
    text-align: left;
  }
}
</style>
