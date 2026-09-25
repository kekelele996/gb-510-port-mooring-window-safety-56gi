import type { EntityConfig } from './domain';

export type CallState = 'planned' | 'approach' | 'moored' | 'departed';
export const ALL_CALL_STATE: readonly CallState[] = ['planned', 'approach', 'moored', 'departed'];
export type ClearanceState = 'pending' | 'cleared' | 'restricted' | 'expired';
export const ALL_CLEARANCE_STATE: readonly ClearanceState[] = ['pending', 'cleared', 'restricted', 'expired'];

export const ENTITY_CONFIGS: readonly EntityConfig[] = [
  { key: 'vesselCall', path: 'vessels', label: '船舶靠泊', statuses: ['planned', 'approach', 'moored', 'departed'] as const },
  { key: 'mooringPlan', path: 'plans', label: '系泊方案', statuses: ['draft', 'review', 'approved', 'superseded'] as const },
  { key: 'weatherWindow', path: 'weather-windows', label: '风浪窗口', statuses: ['forecast', 'safe', 'restricted', 'expired'] as const },
  { key: 'safetyClearance', path: 'clearance', label: '安全许可', statuses: ['pending', 'cleared', 'restricted', 'expired'] as const }
];
