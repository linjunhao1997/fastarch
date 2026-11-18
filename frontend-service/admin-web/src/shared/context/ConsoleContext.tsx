import { createContext, useContext, useMemo, type ReactNode } from 'react';
import { CONSOLE_LABELS, getConsole, type ConsoleKinds } from '@/shared/api/app-context';

/**
 * 控制台上下文（ADR 0007）：登录即锁定，无运行时切换。
 *
 * 与已废弃的 ApplicationContext 不同：这里没有应用清单、没有切换、没有服务端绑定，
 * 只有一个静态的 console 标识（iam/sender）。cache key 用 console 做隔离，
 * 菜单/字典 API 前缀由 shared/api/app-context 的模块级值决定。
 */
export interface ConsoleContextValue {
  /** 当前控制台标识（登录即锁定，不可变） */
  console: ConsoleKinds;
  /** 控制台展示名（面包屑/顶部） */
  label: string;
}

export const ConsoleContext = createContext<ConsoleContextValue | undefined>(undefined);

export function ConsoleProvider({ children }: { children: ReactNode }) {
  const consoleKind = getConsole();
  const value = useMemo<ConsoleContextValue>(
    () => ({ console: consoleKind, label: CONSOLE_LABELS[consoleKind] }),
    [consoleKind],
  );
  return <ConsoleContext.Provider value={value}>{children}</ConsoleContext.Provider>;
}

export function useConsole(): ConsoleContextValue {
  const context = useContext(ConsoleContext);
  if (!context) throw new Error('useConsole must be used within ConsoleProvider');
  return context;
}