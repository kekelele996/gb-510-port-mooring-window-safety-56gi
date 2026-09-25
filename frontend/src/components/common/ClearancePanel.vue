<script setup lang="ts">
import { computed } from 'vue';
import type { DomainRecord } from '../../types/domain';
import { useAuth } from '../../hooks/useAuth';
import StatusBadge from './StatusBadge.vue';

const props = defineProps<{ records: DomainRecord[]; mode: 'window' | 'clearance' }>();
const emit = defineEmits<{ confirm: [item: DomainRecord] }>();
const { session } = useAuth();
const roleRank: Record<string, number> = { viewer: 1, operator: 2, reviewer: 3, admin: 4 };
const canSubmit = computed(() => (roleRank[session.value?.role || ''] || 0) >= roleRank.operator);
const canReview = computed(() => (roleRank[session.value?.role || ''] || 0) >= roleRank.reviewer);
const displayedRecords = computed(() => {
  const records = [...props.records];
  if (props.mode === 'clearance') {
    records.sort((left, right) => Number(right.status === 'pending') - Number(left.status === 'pending'));
  }
  return records.slice(0, 3);
});

function canAct(item: DomainRecord): boolean {
  if (props.mode !== 'clearance' || item.status !== 'pending') return false;
  if (!item.submittedBy) return canSubmit.value;
  return canReview.value && item.submittedBy !== session.value?.username;
}

function actionLabel(item: DomainRecord): string {
  return item.submittedBy ? '复核并放行' : '提交安全确认';
}
</script>

<template>
  <section class="clearance-panel" aria-label="安全许可协同面板">
    <header>
      <div><span class="eyebrow">TWO-PERSON SAFETY</span><strong>{{ mode === 'window' ? '窗口许可依据' : '双人安全确认' }}</strong></div>
      <small>{{ mode === 'window' ? '窗口版本将随许可审计固化' : '提交人与复核人必须为不同账号' }}</small>
    </header>
    <div class="clearance-grid">
      <article v-for="item in displayedRecords" :key="item.id">
        <div class="clearance-title"><strong>{{ item.code }}</strong><StatusBadge :status="item.status"/></div>
        <p>{{ item.name }}</p>
        <dl>
          <dt>窗口版本</dt><dd>v{{ mode === 'window' ? item.version : (item.windowVersion || 1) }}</dd>
          <template v-if="mode === 'clearance'">
            <dt>首次提交</dt><dd>{{ item.submittedBy || '待提交' }}</dd>
            <dt>独立复核</dt><dd>{{ item.confirmedBy || '待复核' }}</dd>
          </template>
          <template v-else>
            <dt>风险等级</dt><dd>{{ item.riskLevel }}</dd>
            <dt>评估证据</dt><dd>{{ item.evidence || '待补充' }}</dd>
          </template>
        </dl>
        <el-button v-if="mode === 'clearance' && canAct(item)" type="primary" @click="emit('confirm', item)">{{ actionLabel(item) }}</el-button>
        <small v-else-if="mode === 'clearance' && item.status === 'pending' && item.submittedBy">等待其他复核员确认</small>
      </article>
    </div>
  </section>
</template>
