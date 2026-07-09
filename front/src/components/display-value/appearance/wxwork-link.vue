<script setup lang="ts">
const props = defineProps<{
  value: string;
  displayValue: string;
  showDisplayValue?: boolean;
}>();

const handleClick = () => {
  window.open(`wxwork://message?username=${props.value}`);
};
</script>

<template>
  <bk-tag @click="handleClick" class="wxwork-link">
    <i class="hcm-icon bkhcm-icon-user2"></i>
    {{ showDisplayValue ? displayValue : value }}
  </bk-tag>
</template>

<style scoped lang="scss">
.wxwork-link {
  cursor: pointer;
  padding: 0 4px;
}
</style>
