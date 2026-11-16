import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useCallback, useEffect, useMemo, useRef, useState, type ReactNode } from 'react';
import { message } from 'antd';
import { useNavigate } from 'react-router-dom';
import { authApi } from '../api/iam';
import type { MyProfileApplication } from '../types';
import { createContext, useContext } from 'react';

const STORAGE_KEY = 'current_application_id';

function clearCookie(name: string): void {
  document.cookie = `${name}=; expires=Thu, 01 Jan 1970 00:00:00 UTC; path=/;`;
}

/**
 * 应用上下文（多应用裁决 #8/#9）：
 *   - 可进入应用清单来自 ListMyApplications（admin 全量启用应用；其他按准入绑定）；
 *   - "当前应用"是服务端概念：token(jti) → 应用映射存 Redis，切换经
 *     SelectApplication 覆写映射、不重签令牌；本地仅缓存选择偏好（localStorage）；
 *   - 每次挂载都重新绑定（映射随令牌过期失效，页面刷新后自愈）；
 *   - 绑定失败不再静默：可见错误提示；旧令牌（无 jti）直接引导重新登录；
 *   - 切换成功后失效全部业务查询缓存，由调用方决定导航行为。
 */
export interface ApplicationContextValue {
  applications: MyProfileApplication[];
  currentApplicationId: string | null;
  currentApplication: MyProfileApplication | null;
  /** 切换当前应用：服务端绑定成功后生效；失败抛错（已同步弹提示） */
  setCurrentApplicationId: (id: string) => Promise<void>;
  /** 应用清单加载中 */
  loading: boolean;
  /** 应用清单加载失败 */
  error: unknown;
  refetch: () => Promise<unknown>;
}

export const ApplicationContext = createContext<ApplicationContextValue | undefined>(undefined);

export function ApplicationProvider({ children }: { children: ReactNode }) {
  const queryClient = useQueryClient();
  const navigate = useNavigate();
  const query = useQuery({
    queryKey: ['my-applications'],
    queryFn: authApi.listMyApplications,
    staleTime: 5 * 60_000,
    retry: 1,
  });

  const [currentApplicationId, setCurrentApplicationIdState] = useState<string | null>(() =>
    localStorage.getItem(STORAGE_KEY),
  );
  // 防止重复绑定：同一 (应用清单版本, 目标应用) 只发一次 SelectApplication
  const boundRef = useRef<string>('');

  const applications = useMemo(() => query.data?.applications ?? [], [query.data]);

  // 旧令牌（升级前签发、无 jti）无法建立应用映射：清凭证强制重新登录
  const forceRelogin = useCallback(
    (reason: string) => {
      clearCookie('access_token');
      localStorage.removeItem('access_token');
      localStorage.removeItem(STORAGE_KEY);
      message.error(reason);
      navigate('/login');
    },
    [navigate],
  );

  const bindOnServer = useCallback(
    async (app: MyProfileApplication, silent: boolean) => {
      const fingerprint = `${app.id}:${applications.map((a) => a.id).join(',')}`;
      if (boundRef.current === fingerprint) return;
      try {
        await authApi.selectApplication(app.code);
      } catch (error) {
        boundRef.current = '';
        const text = error instanceof Error ? error.message : String(error);
        if (text.includes('令牌')) {
          forceRelogin('登录凭证已过期或不兼容，请重新登录');
        } else if (!silent) {
          message.error(`切换应用失败：${text}`);
        } else {
          message.error(`应用绑定失败：${text}`);
        }
        throw error;
      }
      boundRef.current = fingerprint;
      setCurrentApplicationIdState(app.id);
      localStorage.setItem(STORAGE_KEY, app.id);
      // 绑定成功后业务数据（菜单树/权限/字典等）都依赖新应用上下文，全量失效重取
      void queryClient.invalidateQueries();
    },
    [applications, queryClient, forceRelogin],
  );

  // 清单到位后：恢复既有选择（仍有效则重放绑定），失效则回退第一个可用应用
  useEffect(() => {
    if (query.isPending || applications.length === 0) return;
    const stored = applications.find((app) => app.id === currentApplicationId && app.status === 'enabled');
    const target = stored ?? applications.find((app) => app.status === 'enabled') ?? applications[0];
    void bindOnServer(target, true).catch(() => {
      // 已弹提示；自动绑定失败时不打断页面（菜单区显示重试入口）
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [applications, query.isPending]);

  const setCurrentApplicationId = useCallback(
    async (id: string) => {
      const app = applications.find((a) => a.id === id);
      if (!app) return;
      boundRef.current = '';
      await bindOnServer(app, false);
    },
    [applications, bindOnServer],
  );

  const value = useMemo<ApplicationContextValue>(
    () => ({
      applications,
      currentApplicationId,
      currentApplication: applications.find((app) => app.id === currentApplicationId) ?? null,
      setCurrentApplicationId,
      loading: query.isPending,
      error: query.error,
      refetch: () => query.refetch(),
    }),
    [applications, currentApplicationId, setCurrentApplicationId, query.isPending, query.error, query],
  );

  return <ApplicationContext.Provider value={value}>{children}</ApplicationContext.Provider>;
}

export function useApplicationContext(): ApplicationContextValue {
  const context = useContext(ApplicationContext);
  if (!context) throw new Error('useApplicationContext must be used within ApplicationProvider');
  return context;
}
