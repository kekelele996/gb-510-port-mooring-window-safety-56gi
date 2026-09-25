
import { defineStore } from 'pinia';
import { request } from '../api/client';
import type { DomainRecord, PageMeta } from '../types/domain';
export function createEntityStore(id: string) {
  return defineStore(id, {
    state: () => ({ items: [] as DomainRecord[], meta: { page: 1, pageSize: 20, total: 0 } as PageMeta, loading: false, error: '' }),
    actions: {
      async load(path: string, search = '') { this.loading = true; this.error = ''; try { const result = await request<DomainRecord[]>(`/${path}?page=1&pageSize=20&search=${encodeURIComponent(search)}`); this.items = result.data; this.meta = result.meta || { page: 1, pageSize: 20, total: result.data.length }; } catch (error) { this.error = error instanceof Error ? error.message : String(error); } finally { this.loading = false; } },
      async createRecord(path: string, input: Partial<DomainRecord>) { this.loading = true; this.error = ''; try { await request<DomainRecord>(`/${path}`, { method: 'POST', body: JSON.stringify(input) }); await this.load(path); } catch (error) { this.error = error instanceof Error ? error.message : String(error); } finally { this.loading = false; } },
      async transition(path: string, item: DomainRecord, status: string) {
        this.loading = true;
        this.error = '';
        try {
          await request<DomainRecord>(`/${path}/${item.id}/transition`, { method: 'POST', body: JSON.stringify({ status, expectedVersion: item.version, reason: '前端工作台人工确认' }) });
          await this.load(path);
        } catch (error) {
          this.error = error instanceof Error ? error.message : String(error);
          await this.load(path);
        } finally { this.loading = false; }
      },
      async confirmClearance(path: string, item: DomainRecord) {
        this.loading = true;
        this.error = '';
        try {
          await request<DomainRecord>(`/${path}/${item.id}/transition`, {
            method: 'POST',
            body: JSON.stringify({
              status: 'cleared', expectedVersion: item.version,
              reason: '已复核风浪窗口版本、缆绳方案和回退措施',
            }),
          });
          await this.load(path);
        } catch (error) {
          // 窗口换版/受限/过期会在放行前拒绝；刷新列表以拿到最新的失效状态与提示。
          this.error = error instanceof Error ? error.message : String(error);
          await this.load(path);
        } finally {
          this.loading = false;
        }
      },
      async resubmitClearance(path: string, item: DomainRecord) {
        this.loading = true;
        this.error = '';
        try {
          await request<DomainRecord>(`/${path}/${item.id}/transition`, {
            method: 'POST',
            body: JSON.stringify({
              status: 'pending', expectedVersion: item.version,
              reason: '窗口依据已更新，按当前窗口版本重新提交安全确认',
            }),
          });
          await this.load(path);
        } catch (error) {
          this.error = error instanceof Error ? error.message : String(error);
          await this.load(path);
        } finally {
          this.loading = false;
        }
      },
    },
  });
}
