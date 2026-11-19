import { useMemo } from 'react';
import { Button, Flex, Tag, Tooltip, message } from 'antd';
import { CheckCircleOutlined, StopOutlined } from '@ant-design/icons';
import { useNavigate, useSearchParams } from 'react-router';
import { permissionApi } from '@/iam/api/iam';
import { ListPageShell } from '@/shared/components/ListPageShell';
import { useConfirmDelete } from '@/shared/components/ConfirmDelete';
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { usePermissions } from '@/shared/hooks/permission-context';
import { useTableQuery } from '@/shared/hooks/useTableQuery';
import type { Permission, PermissionKind } from '@/iam/types';
import { PERMISSION_KIND_LABELS } from '@/iam/types';
import { ACTIONS, buildSuccessMessage } from '@/shared/types/copywriting';
import { PERMISSIONS } from '@/shared/types/permissions';
import { formatDateTime } from '@/shared/utils/datetime';

const KIND_TAG_COLORS: Record<PermissionKind, string> = {
  menu: 'blue',
  http_api: 'green',
  grpc: 'purple',
};

export default function PermissionListPage() {
  const navigate = useNavigate();
  const showNotification = useApiErrorNotification();
  const confirmDelete = useConfirmDelete();
  const { hasPermission } = usePermissions();
  const [searchParams] = useSearchParams();
  const tableQuery = useTableQuery({
    domain: 'permissions',
    fetcher: (currentPage, pageSize, keyword) => permissionApi.list(currentPage, pageSize, keyword),
    getItems: (response) => response.permissions,
  });
  const canCreate = hasPermission(PERMISSIONS.permissionCreate);
  const canUpdate = hasPermission(PERMISSIONS.permissionUpdate);
  const canDelete = hasPermission(PERMISSIONS.permissionDelete);

  const columns = useMemo(() => [
    { title: 'ID', dataIndex: 'id', key: 'id' },
    { title: '权限名称', dataIndex: 'name', key: 'name' },
    { title: '权限编码', dataIndex: 'permissionCode', key: 'permissionCode' },
    { title: '所属应用', dataIndex: 'applicationId', key: 'applicationId' },
    {
      title: '类型',
      dataIndex: 'kind',
      key: 'kind',
      render: (kind: PermissionKind) => kind
        ? <Tag color={KIND_TAG_COLORS[kind]}>{PERMISSION_KIND_LABELS[kind] ?? kind}</Tag>
        : '—',
    },
    {
      title: '绑定资源',
      dataIndex: 'resourceList',
      key: 'resourceList',
      render: (resourceList: Permission['resourceList']) => {
        if (!resourceList || resourceList.length === 0) return '—';
        const label = resourceList.map((resource) => resource.name || resource.key).join('、');
        const text = label.length > 20 ? `${label.slice(0, 20)}…` : label;
        return <Tooltip title={label}>{text}</Tooltip>;
      },
    },
    {
      title: '状态',
      dataIndex: 'enabledStatus',
      key: 'enabledStatus',
      render: (status: string) => status === 'enabled'
        ? <Tag icon={<CheckCircleOutlined />}>启用</Tag>
        : <Tag icon={<StopOutlined />}>禁用</Tag>,
    },
    { title: '创建人', dataIndex: 'createdBy', key: 'createdBy' },
    { title: '创建时间', dataIndex: 'createTime', key: 'createTime', render: formatDateTime },
    { title: '修改人', dataIndex: 'updatedBy', key: 'updatedBy' },
    { title: '修改时间', dataIndex: 'updateTime', key: 'updateTime', render: formatDateTime },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: Permission) => (
        <Flex gap={0}>
          {canUpdate ? (
            <Button type="link" onClick={() => navigate(`/authorization_management/permission_edit/${record.id}`)}>
              {ACTIONS.edit}
            </Button>
          ) : null}
          {canDelete ? (
            <Button
              type="link"
              danger
              onClick={() => confirmDelete({
                entityType: '权限',
                entityName: record.name,
                onDelete: async () => {
                  await permissionApi.delete(record.id);
                  await tableQuery.refresh();
                  message.success(buildSuccessMessage(ACTIONS.delete));
                },
                onError: (error) => showNotification(error, '删除权限失败'),
              })}
            >
              {ACTIONS.delete}
            </Button>
          ) : null}
        </Flex>
      ),
    },
  ], [navigate, showNotification, tableQuery, canDelete, canUpdate]);

  return (
    <ListPageShell<Permission>
      columns={columns}
      tableQuery={tableQuery}
      searchPlaceholder="搜索权限名称或编码"
      keyword={searchParams.get('keyword') ?? ''}
      onSearch={tableQuery.setKeyword}
      emptyDescription="暂无权限"
      createLabel={canCreate ? `${ACTIONS.create}权限` : undefined}
      onCreate={canCreate ? () => navigate('/authorization_management/permission_create') : undefined}
    />
  );
}
