import { useMemo, useState } from 'react';
import { Button, Flex, Input, message, Table, Tag } from 'antd';
import { CheckCircleOutlined, StopOutlined } from '@ant-design/icons';
import { useNavigate } from 'react-router';
import { accountApi } from '../../api/iam';
import { useConfirmDelete } from '../../components/ConfirmDelete';
import { BindRolesModal } from '../../components/BindRolesModal';
import { AssignApplicationsModal } from '../../components/AssignApplicationsModal';
import { EmptyState, ErrorState } from '../../components/states';
import { useApiErrorNotification } from '../../hooks/useApiError';
import { usePermissions } from '../../hooks/permission-context';
import { useTableQuery } from '../../hooks/useTableQuery';
import type { Account, AccountApplication } from '../../types';
import { ACTIONS, buildSuccessMessage } from '../../types/copywriting';
import { formatDateTime } from '../../utils/datetime';

const { Search } = Input;

export default function AccountListPage() {
  const navigate = useNavigate();
  const showNotification = useApiErrorNotification();
  const { reload } = usePermissions();
  const [bindRolesAccount, setBindRolesAccount] = useState<Account | null>(null);
  const [assignAppsAccount, setAssignAppsAccount] = useState<Account | null>(null);
  const confirmDelete = useConfirmDelete();

  const tableQuery = useTableQuery({
    domain: 'accounts',
    fetcher: (currentPage, pageSize, keyword) => accountApi.list(currentPage, pageSize, keyword),
    getItems: (response) => response.accounts,
  });

  const columns = useMemo(() => [
    { title: 'ID', dataIndex: 'id', key: 'id' },
    { title: '用户名', dataIndex: 'username', key: 'username' },
    { title: '昵称', dataIndex: 'nickname', key: 'nickname', render: (value: string) => value || '—' },
    { title: '邮箱', dataIndex: 'email', key: 'email', render: (value: string) => value || '—' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => status === 'disabled'
        ? <Tag icon={<StopOutlined />}>禁用</Tag>
        : <Tag icon={<CheckCircleOutlined />}>启用</Tag>,
    },
    {
      title: '绑定角色',
      dataIndex: 'roleCodeList',
      key: 'roleCodeList',
      render: (roleCodeList: string[] | undefined) =>
        roleCodeList && roleCodeList.length > 0
          ? <Flex gap={4} wrap>{roleCodeList.map((code) => <Tag key={code}>{code}</Tag>)}</Flex>
          : '—',
    },
    {
      title: '准入应用',
      dataIndex: 'applications',
      key: 'applications',
      render: (applications: AccountApplication[] | undefined) =>
        applications && applications.length > 0
          ? <Flex gap={4} wrap>{applications.map((app) => (
              <Tag key={app.id} color="geekblue">
                {app.status === 'enabled' ? `${app.name}（${app.code}）` : `${app.name}（${app.code}·已停用）`}
              </Tag>
            ))}</Flex>
          : '—',
    },
    { title: '创建人', dataIndex: 'createdBy', key: 'createdBy' },
    { title: '创建时间', dataIndex: 'createTime', key: 'createTime', render: formatDateTime },
    { title: '修改人', dataIndex: 'updatedBy', key: 'updatedBy' },
    { title: '修改时间', dataIndex: 'updateTime', key: 'updateTime', render: formatDateTime },
    {
      title: '操作',
      key: 'action',
      render: (_: unknown, record: Account) => (
        <Flex gap={0}>
          <Button type="link" onClick={() => navigate(`/account_management/edit_account/${record.id}`)}>
            {ACTIONS.edit}
          </Button>
          <Button type="link" onClick={() => setBindRolesAccount(record)}>{ACTIONS.bindRoles}</Button>
          <Button type="link" onClick={() => setAssignAppsAccount(record)}>分配应用</Button>
          <Button
            type="link"
            danger
            onClick={() => confirmDelete({
              entityType: '账号',
              entityName: record.username,
              onDelete: async () => {
                await accountApi.delete(record.id);
                await tableQuery.refresh();
                message.success(buildSuccessMessage(ACTIONS.delete));
              },
              onError: (error) => showNotification(error, '删除账号失败'),
            })}
          >
            {ACTIONS.delete}
          </Button>
        </Flex>
      ),
    },
  ], [navigate, showNotification, tableQuery]);

  return (
    <div>
      <Flex gap={16} style={{ marginBottom: 16 }}>
        <Search
          placeholder="搜索用户名、昵称或邮箱"
          defaultValue={new URLSearchParams(window.location.search).get('keyword') ?? ''}
          onSearch={tableQuery.setKeyword}
          style={{ width: 240 }}
        />
        <Button onClick={() => tableQuery.setKeyword('')}>{ACTIONS.reset}</Button>
        <Button type="primary" onClick={() => navigate('/account_management/create_account')}>
          {ACTIONS.create}账号
        </Button>
      </Flex>
      {tableQuery.status === 'loading' ? (
        <Table<Account> columns={columns} dataSource={[]} rowKey="id" loading pagination={false} />
      ) : tableQuery.status === 'error' ? (
        <ErrorState message="获取账号列表失败" onRetry={() => { void tableQuery.refetch(); }} />
      ) : tableQuery.status === 'forbidden' ? (
        <ErrorState message="无权限" onRetry={() => { void tableQuery.refetch(); }} />
      ) : tableQuery.status === 'empty' ? (
        <EmptyState
          description="暂无账号"
          action={{ text: '创建账号', onClick: () => navigate('/account_management/create_account') }}
        />
      ) : (
        <Table<Account>
          columns={columns}
          dataSource={tableQuery.items}
          rowKey="id"
          pagination={{
            current: tableQuery.page?.currentPage ?? 1,
            pageSize: tableQuery.page?.pageSize ?? 10,
            total: tableQuery.page?.total ?? 0,
            showSizeChanger: true,
            onChange: (page, pageSize) => {
              if (pageSize !== (tableQuery.page?.pageSize ?? 10)) tableQuery.setPageSize(pageSize);
              else tableQuery.setPage(page);
            },
          }}
        />
      )}
      <BindRolesModal
        open={bindRolesAccount !== null}
        account={bindRolesAccount}
        onClose={() => setBindRolesAccount(null)}
        onSuccess={async () => {
          await tableQuery.refresh();
          await reload();
          message.success(buildSuccessMessage(ACTIONS.bindRoles));
        }}
      />
      <AssignApplicationsModal
        open={assignAppsAccount !== null}
        account={assignAppsAccount}
        onClose={() => setAssignAppsAccount(null)}
        onSuccess={async () => {
          await tableQuery.refresh();
          message.success('分配应用成功');
        }}
      />
    </div>
  );
}
