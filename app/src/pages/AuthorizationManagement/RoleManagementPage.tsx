import { useMemo } from 'react';
import { Button, Flex, Tag, message } from 'antd';
import { CheckCircleOutlined, StopOutlined } from '@ant-design/icons';
import { useNavigate, useSearchParams } from 'react-router';
import { roleApi } from '../../api/iam';
import { ListPageShell } from '../../components/ListPageShell';
import { useConfirmDelete } from '../../components/ConfirmDelete';
import { useApiErrorNotification } from '../../hooks/useApiError';
import { useTableQuery } from '../../hooks/useTableQuery';
import type { Role } from '../../types';
import { ACTIONS, buildSuccessMessage } from '../../types/copywriting';

export default function RoleManagementPage() {
  const navigate = useNavigate();
  const showNotification = useApiErrorNotification();
  const confirmDelete = useConfirmDelete();
  const [searchParams] = useSearchParams();
  const tableQuery = useTableQuery({
    domain: 'roles',
    fetcher: (currentPage, pageSize, keyword) => roleApi.list(currentPage, pageSize, keyword),
    getItems: (response) => response.roles,
  });

  const columns = useMemo(() => [
    { title: 'ID', dataIndex: 'id', key: 'id' },
    { title: '角色名称', dataIndex: 'name', key: 'name' },
    { title: '角色编码', dataIndex: 'roleCode', key: 'roleCode' },
    { title: '所属应用', dataIndex: 'applicationId', key: 'applicationId' },
    {
      title: '状态',
      dataIndex: 'enabledStatus',
      key: 'enabledStatus',
      render: (status: string) => status === 'enabled'
        ? <Tag icon={<CheckCircleOutlined />}>启用</Tag>
        : <Tag icon={<StopOutlined />}>禁用</Tag>,
    },
    {
      title: '权限编码',
      dataIndex: 'permissionCodeList',
      key: 'permissionCodeList',
      render: (permissionCodeList: string[] | undefined) =>
        permissionCodeList && permissionCodeList.length > 0
          ? <Flex gap={4} wrap>{permissionCodeList.map((code) => <Tag key={code}>{code}</Tag>)}</Flex>
          : '—',
    },
    { title: '创建人', dataIndex: 'createdBy', key: 'createdBy' },
    { title: '创建时间', dataIndex: 'createTime', key: 'createTime', render: (value: string) => value || '—' },
    { title: '修改人', dataIndex: 'updatedBy', key: 'updatedBy' },
    { title: '修改时间', dataIndex: 'updateTime', key: 'updateTime', render: (value: string) => value || '—' },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: Role) => (
        <Flex gap={0}>
          <Button type="link" onClick={() => navigate(`/authorization_management/role_edit/${record.id}`)}>
            {ACTIONS.edit}
          </Button>
          <Button
            type="link"
            danger
            onClick={() => confirmDelete({
              entityType: '角色',
              entityName: record.name,
              onDelete: async () => {
                await roleApi.delete(record.id);
                await tableQuery.refresh();
                message.success(buildSuccessMessage(ACTIONS.delete));
              },
              onError: (error) => showNotification(error, '删除角色失败'),
            })}
          >
            {ACTIONS.delete}
          </Button>
        </Flex>
      ),
    },
  ], [navigate, showNotification, tableQuery]);

  return (
    <ListPageShell<Role>
      columns={columns}
      tableQuery={tableQuery}
      searchPlaceholder="搜索角色名称或编码"
      keyword={searchParams.get('keyword') ?? ''}
      onSearch={tableQuery.setKeyword}
      emptyDescription="暂无角色"
      createLabel={`${ACTIONS.create}角色`}
      onCreate={() => navigate('/authorization_management/role_create')}
    />
  );
}
