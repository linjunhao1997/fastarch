/**
 * flow 控制台路由子树（第三个控制台，path=/flow）。
 * 路径与 flow-service 迁移里的菜单种子（/flow/tasks、/flow/questions）精确匹配；
 * 叶子路由必须声明 handle.permission：既用于直访守卫，也让 collectRouteMeta 能登记
 * path → meta，否则服务端菜单项会因"路由表无对应页面"被跳过（侧栏空白）。
 */
import { lazy } from 'react';
import type { RouteObject } from 'react-router-dom';
import { withSuspense } from '@/shared/components/withSuspense';

const TaskWorkbenchPage = lazy(() => import('@/flow/pages/TaskWorkbenchPage'));
const QuestionCenterPage = lazy(() => import('@/flow/pages/QuestionCenterPage'));
const MembersPage = lazy(() => import('@/flow/pages/MembersPage'));
const MenuManagementPage = lazy(() => import('@/shared/pages/MenuManagementPage'));
const DictManagementPage = lazy(() => import('@/shared/pages/DictManagementPage'));

export const flowRoutes: RouteObject[] = [
  { index: true, element: withSuspense(<TaskWorkbenchPage />) },
  {
    path: 'tasks',
    handle: { breadcrumb: '任务工作台', permission: 'flow.task.list' },
    element: withSuspense(<TaskWorkbenchPage />),
  },
  {
    path: 'questions',
    handle: { breadcrumb: '问题中心', permission: 'flow.question.list' },
    element: withSuspense(<QuestionCenterPage />),
  },
  {
    path: 'members',
    handle: { breadcrumb: '成员与角色', permission: 'flow.member.get' },
    element: withSuspense(<MembersPage />),
  },
  {
    path: 'menu_management',
    handle: { breadcrumb: '菜单管理', permission: 'flow.menu.list' },
    element: withSuspense(<MenuManagementPage />),
  },
  {
    path: 'dict_management',
    handle: { breadcrumb: '字典管理', permission: 'flow.dict.list' },
    element: withSuspense(<DictManagementPage />),
  },
];
