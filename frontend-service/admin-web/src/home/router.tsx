/**
 * home 控制台路由子树（第四个控制台，path=/home）。
 * 路径与 home-service 迁移里的菜单种子（/home/homepage）精确匹配；
 * 叶子路由声明 handle.permission，供直访守卫与 collectRouteMeta 登记。
 */
import { lazy } from 'react';
import { Navigate, type RouteObject } from 'react-router-dom';
import { withSuspense } from '@/shared/components/withSuspense';

const HomepageEditorPage = lazy(() => import('@/home/pages/HomepageEditorPage'));

export const homeRoutes: RouteObject[] = [
  { index: true, element: <Navigate to="/home/homepage" replace /> },
  {
    path: 'homepage',
    handle: { breadcrumb: '首页配置', permission: 'home.page.get' },
    element: withSuspense(<HomepageEditorPage />),
  },
];
