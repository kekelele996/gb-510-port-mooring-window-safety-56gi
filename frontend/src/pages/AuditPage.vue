<script setup lang="ts">
import { onMounted, ref } from 'vue';
import { listAudits } from '../api/audit';
import type { AuditLog } from '../types/domain';
import { formatDate } from '../utils/format';

const logs = ref<AuditLog[]>([]);
const error = ref('');
onMounted(() => listAudits().then((result) => logs.value = result.data).catch((reason) => error.value = String(reason)));
</script>

<template>
  <main class="workspace">
    <header class="page-header">
      <div><p class="eyebrow">治理与追踪</p><h1>操作审计</h1><p>记录窗口版本、操作者、请求 ID 和不可逆状态变化。</p></div>
    </header>
    <el-alert v-if="error" :title="error" type="error"/>
    <section class="audit-list">
      <article v-for="log in logs" :key="log.id">
        <time>{{ formatDate(log.createdAt) }}</time>
        <strong>{{ log.actor }} · {{ log.action }}</strong>
        <span>{{ log.entityType }} #{{ log.entityId }}</span>
        <code>{{ log.beforeState || '-' }} → {{ log.afterState || '-' }}<small v-if="log.windowVersion">窗口 v{{ log.windowVersion }}</small></code>
        <small>{{ log.requestId }}</small>
      </article>
    </section>
  </main>
</template>
