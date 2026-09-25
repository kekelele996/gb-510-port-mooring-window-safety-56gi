<script setup lang="ts">
import { computed } from 'vue';
import type { DomainRecord } from '../../types/domain';
import { useAuth } from '../../hooks/useAuth';
import { formatDate } from '../../utils/format';
import StatusBadge from './StatusBadge.vue';

const props = defineProps<{ records: DomainRecord[]; mode: 'window' | 'clearance' }>();
const emit = defineEmits<{ confirm: [item: DomainRecord]; resubmit: [item: DomainRecord] }>();
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

function canResubmit(item: DomainRecord): boolean {
  return props.mode === 'clearance' && ['restricted', 'expired'].includes(item.status) && canSubmit.value;
}

function actionLabel(item: DomainRecord): string {
  return item.submittedBy ? '复核并放行' : '提交安全确认';
}
</script>

<template>
  <section class="clearance-panel" aria-label="安全许可协同面板">
    <header>
      <div><span class="eyebrow">TWO-PERSON SAFETY</span><strong>{{ mode === 'window' ? '窗口许可依据' : '双人安全确认' }}</strong></div>
      <small>{{ mode === 'window' ? '窗口失效后关联许可同步失效' : '提交人与复核人必须为不同账号' }}</small>
    </header>
    <div class="clearance-grid">
      <article v-for="item in displayedRecords" :key="item.id">
        <div class="clearance-title"><strong>{{ item.code }}</strong><StatusBadge :status="item.status"/></div>
        <p>{{ item.name }}</p>
        <dl>
          <template v-if="mode === 'clearance'">
            <dt>关联窗口</dt><dd>{{ item.relatedCode }}（{{ item.facility }}）</dd>
            <dt>冻结版本</dt><dd>v{{ item.windowVersion || 1 }}</dd>
            <dt>窗口有效期</dt><dd>至 {{ formatDate(item.windowExpireAt || '') }}</dd>
            <dt>首次提交</dt><dd>{{ item.submittedBy || '待提交' }}</dd>
            <dt>独立复核</dt><dd>{{ item.confirmedBy || '待复核' }}</dd>
          </template>
          <template v-else>
            <dt>窗口版本</dt><dd>v{{ item.version }}</dd>
            <dt>失效时间</dt><dd>{{ formatDate(item.expireAt || '') }}</dd>
            <dt>风险等级</dt><dd>{{ item.riskLevel }}</dd>
            <dt>评估证据</dt><dd>{{ item.evidence || '待补充' }}</dd>
          </template>
        </dl>
        <el-alert v-if="mode === 'clearance' && item.invalidReason" :title="item.invalidReason"
          :type="item.status === 'expired' ? 'error' : 'warning'" :closable="false" show-icon class="clearance-invalid"/>
        <el-button v-if="mode === 'clearance' && canAct(item)" type="primary" @click="emit('confirm', item)">{{ actionLabel(item) }}</el-button>
        <el-button v-else-if="mode === 'clearance' && canResubmit(item)" type="warning" @click="emit('resubmit', item)">按当前窗口重新提交</el-button>
        <small v-else-if="mode === 'clearance' && item.status === 'pending' && item.submittedBy">等待其他复核员确认</small>
      </article>
    </div>
  </section>
</template>

<style scoped>
.clearance-invalid { margin: 8px 0; }
</style>
