
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listSafetyClearance(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/clearance?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createSafetyClearance(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/clearance', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionSafetyClearance(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/clearance/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
