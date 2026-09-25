<script setup lang="ts">
import EntityPage from '../components/EntityPage.vue';
import RiskBadge from '../components/common/RiskBadge.vue';
import { ENTITY_CONFIGS } from '../types/status';
import { useMooringPlanStore } from '../stores/mooring-plan';

const store = useMooringPlanStore();
</script>

<template>
  <EntityPage :config="ENTITY_CONFIGS[1]" :store="store">
    <template #insight><RiskBadge :records="store.items"/></template>
  </EntityPage>
</template>
