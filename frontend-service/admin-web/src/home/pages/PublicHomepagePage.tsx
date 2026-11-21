import { useEffect, useState } from 'react';
import { Spin } from 'antd';
import HomepagePreview from '@/home/pages/HomepagePreview';
import { parseHomepageJson, type HomepageContent, type PublishedHomepageRecord } from '@/home/types/homepage';

/**
 * 企业官网公开首页：不经过后台登录与侧栏，直接读取已发布版本渲染。
 * 数据源为 GET /home/homepage/published（公开读接口）。
 */
export default function PublicHomepagePage() {
  const [content, setContent] = useState<HomepageContent | null>(null);
  const [error, setError] = useState('');

  useEffect(() => {
    let active = true;
    fetch('/home/homepage/published', { headers: { 'X-Fa-App-Code': 'home' } })
      .then(async (response) => {
        if (!response.ok) throw new Error(`HTTP ${response.status}`);
        const record = (await response.json()) as PublishedHomepageRecord;
        if (!active) return;
        setContent(parseHomepageJson(record.contentJson));
      })
      .catch((e: unknown) => {
        if (!active) return;
        setError(e instanceof Error ? e.message : '加载失败');
      });
    return () => {
      active = false;
    };
  }, []);

  useEffect(() => {
    if (content) document.title = `${content.brandName} · 企业官网`;
  }, [content]);

  if (error) {
    return (
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          flexDirection: 'column',
          alignItems: 'center',
          justifyContent: 'center',
          gap: 12,
          background: '#f6f8fb',
          color: '#475569',
          fontFamily: '"PingFang SC", "Microsoft YaHei", sans-serif',
        }}
      >
        <div style={{ fontSize: 18, color: '#0f172a', fontWeight: 600 }}>官网内容暂时无法加载</div>
        <div style={{ fontSize: 13 }}>{error}</div>
      </div>
    );
  }

  if (!content) {
    return (
      <div
        style={{
          minHeight: '100vh',
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          background: '#f6f8fb',
        }}
      >
        <Spin size="large" tip="正在加载官网内容" />
      </div>
    );
  }

  return (
    <div style={{ minHeight: '100vh', background: '#f6f8fb' }}>
      <HomepagePreview content={content} />
    </div>
  );
}
