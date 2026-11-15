import { useMemo } from 'react';
import { Button, Flex, Tag, message } from 'antd';
import { CheckCircleOutlined, StopOutlined } from '@ant-design/icons';
import { useNavigate, useSearchParams } from 'react-router';
import { applicationApi } from '../../api/iam';
import { ListPageShell } from '../../components/ListPageShell';
import { useConfirmDelete } from '../../components/ConfirmDelete';
import { useApiErrorNotification } from '../../hooks/useApiError';
import { useTableQuery } from '../../hooks/useTableQuery';
import type { Application } from '../../types';
import { ACTIONS, buildSuccessMessage } from '../../types/copywriting';
import { formatDateTime } from '../../utils/datetime';

export default function ApplicationListPage() {
  const navigate = useNavigate();
  const showNotification = useApiErrorNotification();
  const confirmDelete = useConfirmDelete();
  const [searchParams] = useSearchParams();
  const tableQuery = useTableQuery({
    domain: 'applications',
    fetcher: (currentPage, pageSize, keyword) => applicationApi.list(currentPage, pageSize, keyword),
    getItems: (response) => response.applications,
  });

  const columns = useMemo(() => [
    { title: 'ID', dataIndex: 'id', key: 'id' },
    { title: '应用标识', dataIndex: 'appId', key: 'appId' },
    {
      title: '服务段编码', dataIndex: 'code', key: 'code',
      render: (code: string) => <Tag color="geekblue">{code}</Tag>,
    },
    { title: '应用名称', dataIndex: 'name', key: 'name' },
    {
      title: '状态', dataIndex: 'status', key: 'status',
      render: (status: string) => status === 'enabled'
        ? <Tag icon={<CheckCircleOutlined />}>启用</Tag>
        : <Tag icon={<StopOutlined />}>禁用</Tag>,
    },
    { title: '创建时间', dataIndex: 'createTime', key: 'createTime', render: formatDateTime },
    { title: '修改时间', dataIndex: 'updateTime', key: 'updateTime', render: formatDateTime },
    {
      title: '操作', key: 'action',
      render: (_: unknown, record: Application) => (
        <Flex gap={0}>
          <Button type="link" onClick={() => navigate(`/application_management/edit_application/${record.id}`)}>
            {ACTIONS.edit}
          </Button>
          <Button
            type="link"
            danger
            onClick={() => confirmDelete({
              entityType: '应用',
              entityName: record.name,
              onDelete: async () => {
                await applicationApi.delete(record.id);
                await tableQuery.refresh();
                message.success(buildSuccessMessage(ACTIONS.delete));
              },
              onError: (error) => showNotification(error, '删除应用失败'),
            })}
          >
            {ACTIONS.delete}
          </Button>
        </Flex>
      ),
    },
  ], [navigate, showNotification, tableQuery]);

  return (
    <ListPageShell<Application>
      columns={columns}
      tableQuery={tableQuery}
      searchPlaceholder="搜索应用名称"
      keyword={searchParams.get('keyword') ?? ''}
      onSearch={tableQuery.setKeyword}
      emptyDescription="暂无应用"
      createLabel={`${ACTIONS.create}应用`}
      onCreate={() => navigate('/application_management/create_application')}
    />
  );
}
