<script setup lang="ts">
import EntityPage from '../components/EntityPage.vue';
import ClearancePanel from '../components/common/ClearancePanel.vue';
import type { DomainRecord } from '../types/domain';
import { ENTITY_CONFIGS } from '../types/status';
import { useSafetyClearanceStore } from '../stores/safety-clearance';

const store = useSafetyClearanceStore();
const confirmClearance = (item: DomainRecord) => store.confirmClearance('clearance', item);
const resubmitClearance = (item: DomainRecord) => store.resubmitClearance('clearance', item);
</script>

<template>
  <EntityPage :config="ENTITY_CONFIGS[3]" :store="store" hide-transitions>
    <template #insight><ClearancePanel :records="store.items" mode="clearance" @confirm="confirmClearance" @resubmit="resubmitClearance"/></template>
  </EntityPage>
</template>
