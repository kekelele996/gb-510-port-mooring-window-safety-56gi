<script setup lang="ts">
import type { DomainRecord } from '../../types/domain';
import StatusBadge from './StatusBadge.vue';

defineProps<{ records: DomainRecord[] }>();
</script>

<template>
  <section class="risk-board" aria-label="系泊风险等级">
    <header>
      <strong>系泊风险等级</strong>
      <span>{{ records.filter((item) => ['high', 'critical'].includes(item.riskLevel)).length }} 项需优先复核</span>
    </header>
    <div v-if="records.length" class="evidence-strip">
      <article v-for="item in records.slice(0, 4)" :key="item.id">
        <div><strong>{{ item.code }}</strong><span>{{ item.name }}</span></div>
        <em :class="`risk risk--${item.riskLevel}`">{{ item.riskLevel }}</em>
        <StatusBadge :status="item.status"/>
      </article>
    </div>
    <div v-else class="empty">暂无风险评估记录</div>
  </section>
</template>
