<script setup lang="ts">
import { inject, ref, type Ref } from 'vue';
import { VendorEnum } from '@/common/constant';
import type { ModelPropertyDisplay } from '@/model/typings';
import GridContainer from '@/components/layout/grid-container/grid-container.vue';
import GridItem from '@/components/layout/grid-container/grid-item.vue';
import routerAction from '@/router/utils/action';
import { useWhereAmI } from '@/hooks/useWhereAmI';

// ---- 1. 注入当前云厂商（多类型模块需要，单类型可删除）----
const currentVendor = inject<Ref<VendorEnum>>('currentVendor', ref(VendorEnum.TCLOUD));

// ---- 2. 接收数据（Sideslider 模式通过 props；独立路由模式自行获取）----
// eslint-disable-next-line vue/no-setup-props-destructure
const props = defineProps<{
  data: Record<string, any>;
}>();

// ---- 3. 字段模型（多类型用 Factory，单类型直接 getModel）----
// import { FieldFactory } from './field-factory';
// const model = FieldFactory.createModel(currentVendor.value);
// const properties = model.getPropertiesByGroup<ModelPropertyDisplay>();
const properties = ref<Record<string, ModelPropertyDisplay[]>>({}); // 占位

// ---- 4. 工具 ----
const { getBizsId } = useWhereAmI();

// ---- 5. 路由跳转示例 ----
// import { MENU_BUSINESS_XXX } from '@/constants/menu-symbol';
// const handleGoToRelated = (id: string) => {
//   routerAction.redirect(
//     { name: MENU_BUSINESS_XXX, query: { id, bizs: getBizsId() } },
//     { history: true },
//   );
// };
</script>

<template>
  <div class="xxx-details">
    <div v-for="(fields, group) in properties" :key="group" class="details-panel">
      <div class="panel-title">{{ group }}</div>
      <grid-container :column="1" :label-width="120">
        <grid-item v-for="field in fields" :key="field.id" :label="field.name">
          <!-- 特殊字段自定义渲染示例 -->
          <template v-if="field.id === 'account_id'">
            <!-- <SecondaryAccountValue :value="data.cloud_account_id" :biz-id="getBizsId()" :vendor="currentVendor" /> -->
            <span>{{ data[field.id] }}</span>
          </template>

          <!-- 默认使用 display-value 渲染 -->
          <!-- value 绑定规则：render 需要访问其它字段时传 data，否则传 data[field.id] -->
          <template v-else>
            <display-value
              :property="field"
              :value="field.meta?.display?.render ? data : data[field.id]"
              :display="{ ...field.meta?.display, on: 'info' }"
            />
          </template>
        </grid-item>
      </grid-container>
    </div>
  </div>
</template>

<style lang="scss" scoped>
.xxx-details {
  display: flex;
  flex-direction: column;
  gap: 12px;

  .details-panel {
    background: #fff;
    border-radius: 2px;
    box-shadow: 0 2px 4px 0 #1919290d;
    padding: 16px 24px;

    .panel-title {
      font-size: 14px;
      font-weight: 700;
      color: #313238;
      line-height: 22px;
      margin-bottom: 8px;
    }
  }
}
</style>
