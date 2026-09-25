import { createRouter, createWebHistory } from 'vue-router';
import VesselCallPage from '../pages/VesselCallPage.vue';
import MooringPlanPage from '../pages/MooringPlanPage.vue';
import WeatherWindowPage from '../pages/WeatherWindowPage.vue';
import SafetyClearancePage from '../pages/SafetyClearancePage.vue';
import AuditPage from '../pages/AuditPage.vue';
import { ensureSession } from '../hooks/useAuth';

const roleRank: Record<string, number> = { viewer: 1, operator: 2, reviewer: 3, admin: 4 };

export const router = createRouter({ history: createWebHistory(), routes: [
  { path: '/', redirect: '/vessels' },
  { path: '/vessels', component: VesselCallPage, meta: { minimumRole: 'viewer' } },
  { path: '/plans', component: MooringPlanPage, meta: { minimumRole: 'viewer' } },
  { path: '/weather-windows', component: WeatherWindowPage, meta: { minimumRole: 'viewer' } },
  { path: '/clearance', component: SafetyClearancePage, meta: { minimumRole: 'operator' } },
  { path: '/audit', component: AuditPage, meta: { minimumRole: 'reviewer' } },
] });

router.beforeEach(async (to) => {
  const current = await ensureSession();
  const minimumRole = String(to.meta.minimumRole || 'viewer');
  if ((roleRank[current.role] || 0) < (roleRank[minimumRole] || 1)) return '/vessels';
  return true;
});
