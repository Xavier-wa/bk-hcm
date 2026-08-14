<script setup lang="ts">
import { computed } from 'vue';

import { getVendorDisplay } from '@/hooks/chatbot/vendor-display';
import { type SelectedAccountEcho } from '@/hooks/chatbot/use-account-select';

const props = defineProps<{ echo: SelectedAccountEcho }>();

const icon = computed(() => getVendorDisplay(props.echo.vendor).icon);
</script>

<template>
  <div class="account-select-echo">
    <img class="ase-icon" :src="icon" :alt="echo.vendorName" />
    <span class="ase-text">云账号：【{{ echo.vendorName }}】- {{ echo.accountName }}</span>
  </div>
</template>

<style scoped lang="scss">
.account-select-echo {
  display: flex;
  align-items: center;
  height: 36px;
  padding: 0 16px;
  background: #f0f1f5;

  .ase-icon {
    flex-shrink: 0;
    width: 18px;
    height: 18px;
    margin-right: 8px;
  }

  .ase-text {
    overflow: hidden;
    font-size: 13px;
    color: #313238;
    text-overflow: ellipsis;
    white-space: nowrap;
  }
}
</style>
