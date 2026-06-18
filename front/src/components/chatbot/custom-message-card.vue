<script setup lang="ts">
import { computed, ref } from 'vue';
import { AngleDown, AngleUp, Success, Warn } from 'bkui-vue/lib/icon';

// 自定义消息卡通用外壳：统一容器、只读收起/展开摘要、提示横幅、头部布局、操作区与全页/浮窗响应式。
// 各业务卡（账号选择 / 申领方案推荐 / 预提单 等）只通过插槽填充差异内容：
//   - #title：头部标题；#header-extra：头部右侧（翻页 / 全屏等）
//   - 默认插槽：主体内容（选项 / 键值表 / 表格等）
//   - #actions：底部操作按钮
// 只读态有两种形态，由 collapsible 控制：
//   - collapsible=true（默认，已知选中结果）：默认收起为摘要行，可点击展开为只读主体；
//   - collapsible=false（如历史兜底未匹配到选中项）：不显示摘要行，直接只读展示主体。
// 全页（投影无描边）/ 浮窗（描边 + 收紧 padding）的差异由父容器对 .custom-msg-card 做 :deep 覆盖。

interface Props {
  readonly?: boolean;
  summaryText?: string;
  banner?: string;
  // 只读态是否可收起为摘要行（需有可展示的 summaryText 时才置 true）
  collapsible?: boolean;
  // 底部操作区对齐：主机申领左对齐，账号选择确认右对齐
  actionsAlign?: 'left' | 'right';
}

const props = withDefaults(defineProps<Props>(), {
  readonly: false,
  summaryText: '',
  banner: '',
  collapsible: true,
  actionsAlign: 'left',
});

const isExpanded = ref(false);
const toggle = () => {
  isExpanded.value = !isExpanded.value;
};

// 只读 + 可收起 → 出现摘要行（收起/展开）；否则直接展示主体
const showCollapsedSummary = computed(() => props.readonly && props.collapsible && !isExpanded.value);
const showExpandedSummary = computed(() => props.readonly && props.collapsible && isExpanded.value);
</script>

<template>
  <div class="custom-msg-card">
    <!-- 只读收起态：摘要行 -->
    <div v-if="showCollapsedSummary" class="cmc-summary" @click="toggle">
      <Success class="cmc-summary-check" />
      <span class="cmc-summary-text">{{ summaryText }}</span>
      <AngleDown class="cmc-summary-arrow" />
    </div>

    <template v-else>
      <!-- 只读展开态：可点击收起的摘要行 -->
      <div v-if="showExpandedSummary" class="cmc-summary is-expanded" @click="toggle">
        <Success class="cmc-summary-check" />
        <span class="cmc-summary-text">{{ summaryText }}</span>
        <AngleUp class="cmc-summary-arrow" />
      </div>

      <!-- 头部：标题 + 右侧控件 -->
      <div v-if="$slots.title || $slots['header-extra']" class="cmc-header">
        <div class="cmc-title">
          <slot name="title" />
        </div>
        <div v-if="$slots['header-extra']" class="cmc-header-extra">
          <slot name="header-extra" />
        </div>
      </div>

      <!-- 提示横幅（数据驱动，可选） -->
      <div v-if="banner" class="cmc-banner">
        <Warn class="cmc-banner-icon" />
        <span class="cmc-banner-text">{{ banner }}</span>
      </div>

      <!-- 主体内容 -->
      <div class="cmc-body">
        <slot />
      </div>

      <!-- 底部操作区 -->
      <div v-if="$slots.actions" class="cmc-actions" :class="{ 'is-right': actionsAlign === 'right' }">
        <slot name="actions" />
      </div>
    </template>
  </div>
</template>

<style scoped lang="scss">
.custom-msg-card {
  // 全页模式内边距（默认）；浮窗模式由父容器（.aa-messages）覆盖为 8 12 12 12
  padding: 16px 24px;
  background: #fff;

  // 默认（浮窗）：1px 描边；全页态由父容器（.chatbot-chat-messages）覆盖为投影无描边
  border: 1px solid #dcdee5;
  border-radius: 8px;

  // 以卡片自身宽度作为容器查询基准：全页宽 → 常规布局；浮窗窄 → 紧凑布局
  container-type: inline-size;
}

.cmc-summary {
  display: flex;
  align-items: center;
  cursor: pointer;

  .cmc-summary-check {
    flex-shrink: 0;
    margin-right: 8px;
    font-size: 22px;
    color: #2caf5e;
  }

  .cmc-summary-text {
    flex: 1;
    font-size: 14px;
    color: #313238;
  }

  .cmc-summary-arrow {
    flex-shrink: 0;
    font-size: 28px;
    color: #979ba5;
  }

  &.is-expanded {
    margin-bottom: 12px;
  }
}

.cmc-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 12px;

  .cmc-title {
    font-size: 14px;
    font-weight: 700;
    color: #313238;
  }

  .cmc-header-extra {
    display: flex;
    flex-shrink: 0;
    gap: 8px;
    align-items: center;
  }
}

.cmc-banner {
  display: flex;
  align-items: flex-start;
  padding: 8px 12px;
  margin-bottom: 12px;
  background: #fff4e2;
  border-radius: 4px;

  .cmc-banner-icon {
    flex-shrink: 0;
    margin-top: 2px;
    margin-right: 8px;
    font-size: 16px;
    color: #ff9c01;
  }

  .cmc-banner-text {
    font-size: 12px;
    line-height: 20px;
    color: #63656e;
  }
}

.cmc-actions {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-top: 20px;

  &.is-right {
    justify-content: flex-end;
  }
}

// 窄容器（浮窗）：摘要收紧、头部换行、操作按钮整行铺满
@container (max-width: 380px) {
  .cmc-summary {
    .cmc-summary-text {
      font-size: 12px;
    }

    .cmc-summary-check {
      font-size: 18px;
    }

    .cmc-summary-arrow {
      font-size: 22px;
    }
  }

  .cmc-header {
    flex-wrap: wrap;
    gap: 8px;
  }

  .cmc-actions {
    :deep(.bk-button) {
      flex: 1 1 100%;
    }
  }
}
</style>
