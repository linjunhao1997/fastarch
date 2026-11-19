import { createContext, useCallback, useContext, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { useQueryClient } from '@tanstack/react-query';
import { profileApi } from '@/iam/api/iam';
import { setRequestDataScope } from '@/shared/api/request';
import type { MyProfileApplication } from '@/shared/types';

/**
 * 数据范围上下文：当前"看哪家应用的数据"。
 *
 * 三个正交概念：
 *  - 控制台（ConsoleContext）：决定菜单/字典与 API 前缀，登录进入时锁定；
 *  - 数据范围（本上下文）：决定列表/详情过滤哪个应用的数据，可左上角切换；
 *  - 权限码：决定能做什么操作。
 *
 * 选中的应用 code 由请求层作为 X-Fa-App-Code 头下发，服务端校验 ∈ 接入集合。
 * 切换范围后清空所有查询缓存，防止串数据。
 */
const DATA_SCOPE_STORAGE_KEY = 'data_scope_app_code';

export interface DataScopeContextValue {
  /** 当前选中的数据归属应用 code */
  appCode: string;
  /** 当前选中的应用摘要 */
  current: MyProfileApplication | undefined;
  /** 用户可切换的应用清单（来自 GetMyProfile.applications） */
  applications: MyProfileApplication[];
  loading: boolean;
  setAppCode: (code: string) => void;
}

export const DataScopeContext = createContext<DataScopeContextValue | undefined>(undefined);

function readStoredCode(): string {
  try {
    return localStorage.getItem(DATA_SCOPE_STORAGE_KEY) ?? '';
  } catch {
    return '';
  }
}

function writeStoredCode(code: string): void {
  try {
    localStorage.setItem(DATA_SCOPE_STORAGE_KEY, code);
  } catch {
    // ignore
  }
}

export function DataScopeProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const [applications, setApplications] = useState<MyProfileApplication[]>([]);
  const [appCode, setAppCodeState] = useState<string>(readStoredCode);
  const [loading, setLoading] = useState(true);
  // 只在挂载时读取一次初始选中值；切换范围不重新拉取接入集合（集合由权限分配决定）。
  const initialAppCode = useRef(appCode);

  useEffect(() => {
    let cancelled = false;
    profileApi.get()
      .then((profile) => {
        if (cancelled) return;
        const apps = profile.applications ?? [];
        setApplications(apps);
        setLoading(false);
        // 选中态校验：已存 code 不在接入集合（换账号/权限变更）则回落到第一个。
        const valid = apps.find((app) => app.code === initialAppCode.current && app.status === 'enabled');
        if (!valid) {
          const fallback = apps.find((app) => app.status === 'enabled');
          const next = fallback?.code ?? '';
          setAppCodeState(next);
          writeStoredCode(next);
        }
      })
      .catch(() => {
        if (!cancelled) setLoading(false);
      });
    return () => {
      cancelled = true;
    };
  }, []); // 仅在挂载时拉取一次；切换应用不改变接入集合

  // 选中态与请求层模块状态保持同步；请求层读模块值优先、localStorage 兜底，
  // 这样首屏第一次请求（Provider 挂载即触发）也能带上已持久化的范围。
  useEffect(() => {
    setRequestDataScope(appCode);
  }, [appCode]);

  const setAppCode = useCallback((code: string) => {
    // 只允许切到接入集合内启用的应用，防止构造非法 appCode 绕过前端选择。
    const allowed = applications.some((app) => app.code === code && app.status === 'enabled');
    if (!allowed) return;
    setAppCodeState(code);
    writeStoredCode(code);
    // 清缓存：切换数据范围后所有列表必须重新按新 appCode 拉取，防止串数据。
    queryClient.clear();
  }, [applications, queryClient]);

  const current = useMemo(
    () => applications.find((app) => app.code === appCode),
    [applications, appCode],
  );

  const value = useMemo<DataScopeContextValue>(
    () => ({ appCode, current, applications, loading, setAppCode }),
    [appCode, current, applications, loading, setAppCode],
  );

  return <DataScopeContext.Provider value={value}>{children}</DataScopeContext.Provider>;
}

export function useDataScope(): DataScopeContextValue {
  const context = useContext(DataScopeContext);
  if (!context) throw new Error('useDataScope must be used within DataScopeProvider');
  return context;
}
