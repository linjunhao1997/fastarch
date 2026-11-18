/**
 * sender 控制台路由子树（ADR 0007：单工程双控制台）。
 * 挂载在 /sender 控制台根下，children 相对路径拼成绝对路径后与 sender 菜单种子
 * （/sender/records、/sender/templates）精确匹配。
 */
import { lazy } from 'react';
import type { RouteObject } from 'react-router-dom';

const HomePage = lazy(() => import('@/app/HomePage'));
const RecordListPage = lazy(() => import('@/sender/pages/RecordListPage'));
const TemplateManagementPage = lazy(() => import('@/sender/pages/TemplateManagementPage'));
const MenuManagementPage = lazy(() => import('@/shared/pages/MenuManagementPage'));
const DictManagementPage = lazy(() => import('@/shared/pages/DictManagementPage'));

import { withSuspense } from '@/shared/components/withSuspense';

/** sender 控制台的公共页面（菜单/字典按本控制台自持，前缀 /sender） */
export const senderManagementRoutes: RouteObject[] = [
  { path: 'menu_management', element: withSuspense(<MenuManagementPage />) },
  { path: 'dict_management', element: withSuspense(<DictManagementPage />) },
];

/** sender 控制台子树：IndexPage 之外的全部内部路由 */
export const senderRoutes: RouteObject[] = [
  { index: true, element: withSuspense(<HomePage />) },
  { path: 'records', element: withSuspense(<RecordListPage />) },
  { path: 'templates', element: withSuspense(<TemplateManagementPage />) },
  ...senderManagementRoutes,
];