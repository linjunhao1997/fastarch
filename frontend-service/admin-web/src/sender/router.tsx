/**
 * sender 路由子树（垂直切片）。
 * path 与 sender 迁移种子写死一致（/sender/records、/sender/templates），
 * 菜单树按 menu.path 精确匹配路由，故不能改。
 */
import { lazy } from 'react';
import type { RouteObject } from 'react-router-dom';
import { PERMISSIONS } from '@/shared/types/permissions';
import { withSuspense } from '@/shared/components/withSuspense';

const RecordListPage = lazy(() => import('@/sender/pages/RecordListPage'));
const TemplateManagementPage = lazy(() => import('@/sender/pages/TemplateManagementPage'));

export const senderRoutes: RouteObject[] = [
  {
    path: 'sender/records',
    handle: {
      label: '发送记录',
      breadcrumb: '发送记录',
      permission: PERMISSIONS.senderRecordList,
    },
    element: withSuspense(<RecordListPage />),
  },
  {
    path: 'sender/templates',
    handle: {
      label: '消息模板',
      breadcrumb: '消息模板',
      permission: PERMISSIONS.senderTemplateList,
    },
    element: withSuspense(<TemplateManagementPage />),
  },
];
