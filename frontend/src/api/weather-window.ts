
import { request } from './client';
import type { DomainRecord } from '../types/domain';

export async function listWeatherWindow(page = 1, pageSize = 20, search = '') {
  return request<DomainRecord[]>(`/weather-windows?page=${page}&pageSize=${pageSize}&search=${encodeURIComponent(search)}`);
}
export async function createWeatherWindow(input: Partial<DomainRecord>) {
  return request<DomainRecord>('/weather-windows', { method: 'POST', body: JSON.stringify(input) });
}
export async function transitionWeatherWindow(id: number, status: string, expectedVersion: number, reason: string) {
  return request<DomainRecord>(`/weather-windows/${id}/transition`, {
    method: 'POST', body: JSON.stringify({ status, expectedVersion, reason }),
  });
}
