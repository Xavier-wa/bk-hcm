<script setup lang="ts">
import { computed, watchEffect } from 'vue';
import CombineRequest from '@blueking/combine-request';
import { useUserStore } from '@/store/user';
import { AppearanceType, DisplayType } from './typings';
import WxworkLink from './appearance/wxwork-link.vue';

const props = defineProps<{ value: string | string[]; display?: DisplayType }>();

const localValue = computed(() => {
  if (!props.value) {
    return [];
  }
  return Array.isArray(props.value) ? props.value : [props.value];
});

const appearance = computed(() => props.display?.appearance);
const appearanceProps = computed(() => props.display?.appearanceProps);
const appearanceComps: Partial<Record<AppearanceType, any>> = {
  ['wxwork-link']: WxworkLink,
};

const displayValue = computed(() => {
  const names = localValue.value.map((username) => {
    // 每次从全局store中查询获取
    const user = userStore.userList.find((user) => user.username === username);
    if (!user) {
      return `${username}(--)`;
    }
    return `${user.username}(${user.display_name})`;
  });
  return names?.join?.(';') || '--';
});

const userStore = useUserStore();

const combineRequest = CombineRequest.setup(Symbol.for('user-value'), (users) => {
  const uniqueUsers = [...new Set((users as string[][]).reduce((acc, cur) => acc.concat(cur), []))];
  userStore.getUserByName(uniqueUsers);
});

watchEffect(() => {
  if (!localValue.value.length) {
    return;
  }

  const newUsers = localValue.value.filter(
    (username) => !userStore.userList.some((item) => item.username === username),
  );

  if (newUsers.length) {
    combineRequest.add(newUsers);
  }
});
</script>

<template>
  <template v-if="!appearance">
    <bk-overflow-title class="full-width" resizeable type="tips">
      {{ displayValue }}
    </bk-overflow-title>
  </template>
  <component
    v-else
    :is="appearanceComps[appearance]"
    :display-value="displayValue"
    :value="value"
    v-bind="appearanceProps"
  />
</template>
