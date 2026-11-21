import { request } from '@/shared/api/request';
import type { HomepageRecord, PublishedHomepageRecord } from '@/home/types/homepage';

/**
 * 企业官网控制台的配置接口固定按 home 应用判权。
 * 左上角数据范围是其它控制台的概念；这里显式下发 X-Fa-App-Code: home，
 * 避免用户上次选择 flow 导致 home-service 拿到错误的数据归属。
 */
const HOME_APP_HEADERS = { 'X-Fa-App-Code': 'home' };

export const homeApi = {
  get(): Promise<HomepageRecord> {
    return request<HomepageRecord>('/home/homepage', {
      skipDataScope: true,
      headers: HOME_APP_HEADERS,
    });
  },

  saveDraft(contentJson: string, expectedVersion: number): Promise<HomepageRecord> {
    return request<HomepageRecord>('/home/homepage/draft', {
      method: 'PUT',
      skipDataScope: true,
      headers: HOME_APP_HEADERS,
      body: { contentJson, expectedVersion },
    });
  },

  publish(expectedVersion: number): Promise<HomepageRecord> {
    return request<HomepageRecord>('/home/homepage/publish', {
      method: 'POST',
      skipDataScope: true,
      headers: HOME_APP_HEADERS,
      body: { expectedVersion },
    });
  },

  getPublished(): Promise<PublishedHomepageRecord> {
    return request<PublishedHomepageRecord>('/home/homepage/published', {
      skipDataScope: true,
      headers: HOME_APP_HEADERS,
    });
  },
};
