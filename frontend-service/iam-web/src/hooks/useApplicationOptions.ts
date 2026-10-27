import { useEffect, useState } from 'react';
import { applicationApi } from '../api/iam';
import type { Application } from '../types';

/**
 * 拉取应用选项列表（账号归属多选、创建页默认应用等场景共用）。
 * 单次批量拉取（pageSize=100），失败时保留空列表由调用方提示。
 */
export function useApplicationOptions() {
  const [applications, setApplications] = useState<Application[]>([]);
  const [loading, setLoading] = useState(false);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);
    applicationApi
      .list(1, 100)
      .then((response) => {
        if (!cancelled) setApplications(response.applications);
      })
      .catch(() => {
        // 选项加载失败不阻断页面渲染，提交时服务端校验兜底
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []);

  return { applications, loading };
}
