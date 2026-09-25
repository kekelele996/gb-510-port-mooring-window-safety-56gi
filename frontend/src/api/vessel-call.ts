
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listVesselCall(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/vessels?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createVesselCall(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/vessels', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionVesselCall(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/vessels/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
