<script setup lang="ts">
import { computed, useAttrs, type Component } from 'vue';
import { ModelProperty } from '@/model/typings';
import { DisplayType } from '../typings';

export type TagTheme = 'success' | 'danger' | 'warning' | 'info' | 'default';
export type TagIcon = Component | string;

defineOptions({ name: 'DynamicTagStatus', inheritAttrs: false });

const props = defineProps<{
  value: string | number;
  displayValue: string | number;
  option?: ModelProperty['option'];
  displayOn?: DisplayType['on'];
  themeObject: Partial<Record<TagTheme, Array<string | number>>>;
  icon?: TagIcon;
  iconObject?: Partial<Record<TagTheme, TagIcon>>;
}>();

defineSlots<{
  icon(props: { value: string | number; theme: TagTheme; displayValue: string | number }): unknown;
}>();

const attrs = useAttrs();

const themeKey = computed<TagTheme>(() => {
  for (const [key, values] of Object.entries(props.themeObject || {})) {
    if (values?.includes(props.value)) {
      return key as TagTheme;
    }
  }
  return 'default';
});

const theme = computed(() => (themeKey.value === 'default' ? '' : themeKey.value));

const resolvedIcon = computed(() => props.iconObject?.[themeKey.value] ?? props.icon);

const isIconClass = computed(() => typeof resolvedIcon.value === 'string');
</script>

<template>
  <bk-tag v-bind="attrs" :theme="theme">
    <template v-if="$slots.icon || resolvedIcon" #icon>
      <slot name="icon" :value="value" :theme="themeKey" :display-value="displayValue">
        <i v-if="isIconClass" :class="resolvedIcon" />
        <component :is="resolvedIcon" v-else-if="resolvedIcon" />
      </slot>
    </template>
    {{ displayValue }}
  </bk-tag>
</template>
