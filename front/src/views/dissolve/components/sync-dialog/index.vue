<script setup lang="ts">
import { ref, computed, inject, type Ref } from 'vue';
import { Message } from 'bkui-vue';
import { useDissolveQuotaStore } from '@/store/dissolve/quota';
import type { IDissolveProjectCycle } from '@/store/dissolve/quota';

const isShow = defineModel<boolean>('isShow', { required: true });
const emit = defineEmits<{ (e: 'success'): void }>();

const store = useDissolveQuotaStore();
const loading = ref(false);

// 从父组件注入数据
const dissolveProjects = inject<Ref<IDissolveProjectCycle[]>>('dissolveProjects', ref([]));
const projectTypeList = inject<Ref<Array<{ value: number; label: string }>>>('projectTypeList', ref([]));

// 汇总所有裁撤时间段内配置的项目类型列表
const projectTypes = computed(() => {
  const merged = new Map<number, { value: string; label: string }>();
  dissolveProjects.value.forEach((p) => {
    (p.projects || []).forEach((proj) => {
      if (!merged.has(proj.id)) {
        const projectType = projectTypeList.value.find((pt) => pt.value === proj.id);
        merged.set(proj.id, {
          value: String(proj.id),
          label: projectType?.label || proj.memo || String(proj.id),
        });
      }
    });
  });
  return [...merged.values()];
});

const handleSync = async () => {
  loading.value = true;
  try {
    await store.syncDissolve();
    Message({ theme: 'success', message: '同步成功' });
    isShow.value = false;
    emit('success');
  } catch (e: any) {
    Message({ theme: 'error', message: e?.message || '同步失败' });
  } finally {
    loading.value = false;
  }
};
</script>

<template>
  <bk-dialog v-model:is-show="isShow" title="同步" :width="480" :close-icon="false">
    <p class="sync-tip">本次将同步以下项目类型（来自裁撤配置,仅展示）</p>
    <div class="project-type-list">
      <div v-for="pt in projectTypes" :key="pt.value" class="type-item">
        {{ pt.label }}
      </div>
      <p v-if="!projectTypes.length" class="empty-tip">暂无项目类型</p>
    </div>
    <template #footer>
      <bk-button theme="primary" :loading="loading" @click="handleSync">确定</bk-button>
      <bk-button @click="isShow = false" :loading="loading">取消</bk-button>
    </template>
  </bk-dialog>
</template>

<style lang="scss" scoped>
.sync-tip {
  margin-bottom: 16px;
  color: #4d4f56;
  font-size: 12px;
  line-height: 20px;
}

.project-type-list {
  display: flex;
  flex-direction: column;
  overflow: hidden;
  gap: 8px;
  color: #4d4f56;

  .type-item {
    height: 32px;
    border-radius: 2px;
    padding: 0 12px;
    background: #f5f7fa;
    font-size: 12px;
    line-height: 20px;
    display: flex;
    align-items: center;
  }

  .empty-tip {
    padding: 24px 16px;
    color: #c4c6cc;
    text-align: center;
    font-size: 12px;
  }
}
</style>
