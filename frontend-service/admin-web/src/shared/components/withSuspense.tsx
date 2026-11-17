import { Suspense, type ReactNode } from 'react';
import { Spin } from 'antd';
import { layoutTokens } from '@/shared/theme';

/** 路由懒加载的统一 Suspense 占位（跨 services 共用） */
export function withSuspense(element: ReactNode) {
  return (
    <Suspense
      fallback={
        <Spin style={{ display: 'block', margin: `${layoutTokens.sectionGap * 4}px auto` }} />
      }
    >
      {element}
    </Suspense>
  );
}
