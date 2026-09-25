<script setup lang="ts">
import EntityPage from '../components/EntityPage.vue';
import RiskBadge from '../components/common/RiskBadge.vue';
import ClearancePanel from '../components/common/ClearancePanel.vue';
import { ENTITY_CONFIGS } from '../types/status';
import { useWeatherWindowStore } from '../stores/weather-window';

const store = useWeatherWindowStore();
</script>

<template>
  <EntityPage :config="ENTITY_CONFIGS[2]" :store="store">
    <template #insight>
      <RiskBadge :records="store.items"/>
      <ClearancePanel :records="store.items" mode="window"/>
    </template>
  </EntityPage>
</template>
