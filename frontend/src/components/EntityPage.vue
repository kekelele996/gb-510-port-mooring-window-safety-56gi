<script setup lang="ts">
import { computed, onMounted, ref } from 'vue';
import type { DomainRecord, EntityConfig } from '../types/domain';
import { formatDate, nextStatus } from '../utils/format';
import { useAuth } from '../hooks/useAuth';
import StatusBadge from './common/StatusBadge.vue';
import MetricCard from './common/MetricCard.vue';
import ConfirmDialog from './common/ConfirmDialog.vue';

const props = defineProps<{ config: EntityConfig; store: any; hideTransitions?: boolean }>();
const { session } = useAuth();
const search = ref('');
const showCreate = ref(false);
const pending = ref<{ item: DomainRecord; status: string } | null>(null);
const roleRank: Record<string, number> = { viewer: 1, operator: 2, reviewer: 3, admin: 4 };
const canWrite = computed(() => (roleRank[session.value?.role || ''] || 0) >= roleRank.operator);
const highRisk = computed(() => props.store.items.filter((item: DomainRecord) => ['high', 'critical'].includes(item.riskLevel)).length);

onMounted(() => void props.store.load(props.config.path));

async function createDemo() {
  if (!canWrite.value) return;
  const now = Date.now();
  const isClearance = props.config.key === 'safetyClearance';
  // 许可必须按区域 + 窗口编码关联当前可用窗口，这里使用演示数据中的 WW-001。
  const facility = isClearance ? '港口系泊安全窗口评估区域1' : '默认作业区';
  await props.store.createRecord(props.config.path, {
    code: `${props.config.key.toUpperCase()}-${String(now).slice(-6)}`,
    name: `新增${props.config.label}`,
    description: '通过前端工作台创建的业务记录',
    facility, owner: '现场操作员', category: '常规', riskLevel: 'medium',
    metricValue: 25, metricUnit: 'unit', effectiveAt: new Date().toISOString(),
    expireAt: new Date(now + 6 * 60 * 60 * 1000).toISOString(),
    evidence: '已完成创建前检查',
    relatedCode: isClearance ? 'WW-001' : '',
  });
  showCreate.value = false;
}

async function confirmTransition() {
  if (!pending.value || !canWrite.value) return;
  await props.store.transition(props.config.path, pending.value.item, pending.value.status);
  pending.value = null;
}
</script>

<template>
  <main class="workspace">
    <header class="page-header">
      <div>
        <p class="eyebrow">业务工作台</p>
        <h1>{{ config.label }}</h1>
        <p>统一管理{{ config.label }}的状态、风险、证据与责任人。</p>
      </div>
      <el-button v-if="canWrite" type="primary" @click="showCreate = true">新增{{ config.label }}</el-button>
    </header>
    <section class="metrics">
      <MetricCard label="记录总数" :value="store.meta.total" detail="当前筛选范围"/>
      <MetricCard label="高风险" :value="highRisk" detail="需要优先复核"/>
      <MetricCard label="状态种类" :value="new Set(store.items.map((item: DomainRecord) => item.status)).size" detail="状态机覆盖"/>
    </section>
    <slot name="insight" :items="store.items"/>
    <section class="toolbar">
      <el-input v-model="search" :placeholder="`搜索${config.label}编码或名称`" clearable/>
      <el-button type="primary" @click="store.load(config.path, search)">查询</el-button>
      <el-button @click="search = ''; store.load(config.path)">重置</el-button>
    </section>
    <el-alert v-if="store.error" :title="store.error" type="error" show-icon/>
    <section class="table-shell">
      <el-table v-loading="store.loading" :data="store.items">
        <el-table-column prop="code" label="编码" width="150"/>
        <el-table-column label="名称" min-width="180">
          <template #default="{ row }"><strong>{{ row.name }}</strong><small>{{ row.facility }}</small></template>
        </el-table-column>
        <el-table-column label="状态" width="140"><template #default="{ row }"><StatusBadge :status="row.status"/></template></el-table-column>
        <el-table-column v-if="config.key === 'weatherWindow'" label="失效时间" width="180">
          <template #default="{ row }"><span :class="{ 'muted': new Date(row.expireAt).getTime() < Date.now() }">{{ formatDate(row.expireAt) }}</span></template>
        </el-table-column>
        <el-table-column v-if="config.key === 'safetyClearance'" label="关联窗口" width="170">
          <template #default="{ row }">
            <strong>{{ row.relatedCode }}</strong>
            <small>{{ row.facility }}</small>
          </template>
        </el-table-column>
        <el-table-column v-if="config.key === 'safetyClearance'" label="冻结版本/有效期" width="200">
          <template #default="{ row }">
            <span>v{{ row.windowVersion || 1 }}</span>
            <small>至 {{ formatDate(row.windowExpireAt || '') }}</small>
          </template>
        </el-table-column>
        <el-table-column prop="riskLevel" label="风险" width="90"/>
        <el-table-column prop="owner" label="责任人"/>
        <el-table-column label="指标"><template #default="{ row }">{{ row.metricValue }} {{ row.metricUnit }}</template></el-table-column>
        <el-table-column v-if="config.key === 'safetyClearance'" label="失效提示" min-width="220">
          <template #default="{ row }">
            <el-alert v-if="row.invalidReason" :title="row.invalidReason" :type="row.status === 'expired' ? 'error' : 'warning'" :closable="false" show-icon/>
            <span v-else-if="row.submittedBy" class="muted">等待复核（提交人 {{ row.submittedBy }}）</span>
            <span v-else class="muted">待提交</span>
          </template>
        </el-table-column>
        <el-table-column label="更新时间" width="180"><template #default="{ row }">{{ formatDate(row.updatedAt) }}</template></el-table-column>
        <el-table-column label="操作" width="200">
          <template #default="{ row }">
            <el-button v-if="config.key === 'safetyClearance' && ['restricted', 'expired'].includes(row.status) && canWrite" link type="warning" @click="store.resubmitClearance(config.path, row)">重新提交</el-button>
            <el-button v-else-if="!hideTransitions && canWrite && nextStatus(row.status, config.statuses)" link type="primary" @click="pending = { item: row, status: nextStatus(row.status, config.statuses)! }">推进至 {{ nextStatus(row.status, config.statuses) }}</el-button>
            <span v-else-if="!canWrite" class="muted">只读权限</span>
            <span v-else-if="hideTransitions" class="muted">由安全确认面板处理</span>
            <span v-else class="muted">流程结束</span>
          </template>
        </el-table-column>
      </el-table>
    </section>
    <ConfirmDialog v-model="showCreate" :title="`新增${config.label}`" @confirm="createDemo">
      <p>将创建一条包含完整责任人、风险和证据信息的演示记录。</p>
    </ConfirmDialog>
    <ConfirmDialog :model-value="Boolean(pending)" title="确认状态迁移" @update:model-value="pending = null" @confirm="confirmTransition">
      <p>状态迁移会写入审计日志，并使用版本号避免并发覆盖。</p>
      <strong>{{ pending?.item.status }} → {{ pending?.status }}</strong>
    </ConfirmDialog>
  </main>
</template>
