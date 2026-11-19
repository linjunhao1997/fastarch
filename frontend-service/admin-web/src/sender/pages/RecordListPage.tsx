import { useCallback, useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Button, Form, Input, Modal, Select, Tag, Typography } from 'antd';
import type { TableColumnsType } from 'antd';
import { messageApi } from '@/sender/api/sender';
import { ListPageShell } from '@/shared/components/ListPageShell';
import { useConfirmDelete } from '@/shared/components/ConfirmDelete';
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { usePermissions } from '@/shared/hooks/permission-context';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import { useTableQuery } from '@/shared/hooks/useTableQuery';
import { useSearchParams } from 'react-router-dom';
import type { MessageRecord, SendMessageRequest } from '@/sender/types/sender';
import { PERMISSIONS } from '@/shared/types/permissions';

const { Text, Paragraph } = Typography;

const STATUS_COLOR: Record<string, string> = {
  sent: 'success',
  pending: 'processing',
  failed: 'error',
};

interface SendFormValues {
  recipient: string;
  templateCode?: string;
  content: string;
  channel?: string;
}

/** 发送记录列表 + 发送表单（模拟投递 pending→sent） */
export default function RecordListPage() {
  const showNotification = useApiErrorNotification();
  const confirmDelete = useConfirmDelete();
  const { hasPermission } = usePermissions();
  const canSend = hasPermission(PERMISSIONS.senderRecordSend);
  const canDelete = hasPermission(PERMISSIONS.senderRecordDelete);
  const { isSubmitting, run } = useSubmitLock();
  const [form] = Form.useForm<SendFormValues>();
  const [sendOpen, setSendOpen] = useState(false);
  const [searchParams, setSearchParams] = useSearchParams();

  const tableQuery = useTableQuery({
    domain: 'sender-records',
    fetcher: (currentPage, pageSize, keyword, filters) =>
      messageApi.list({ currentPage, pageSize, keyword, sendStatus: filters.send_status }),
    getItems: (res) => res.records,
    filterKeys: ['send_status'],
  });

  const setFilter = useCallback(
    (key: string, value?: string) => {
      const next = new URLSearchParams(searchParams);
      if (!value) next.delete(key);
      else next.set(key, value);
      next.set('current_page', '1');
      setSearchParams(next, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  const sendMutation = useMutation({
    mutationFn: (values: SendFormValues) => messageApi.send(values as SendMessageRequest),
  });

  const handleSend = async () => {
    const values = await form.validateFields();
    await run(async () => {
      try {
        await sendMutation.mutateAsync(values);
        setSendOpen(false);
        form.resetFields();
        await tableQuery.refresh();
      } catch (error) {
        showNotification(error, '发送失败');
      }
    });
  };

  const handleDelete = (record: MessageRecord) => {
    confirmDelete({
      entityType: '发送记录',
      entityName: record.recipient,
      onDelete: async () => {
        try {
          await messageApi.delete(record.id);
          await tableQuery.refresh();
        } catch (error) {
          showNotification(error, '删除失败');
          throw error;
        }
      },
    });
  };

  const columns: TableColumnsType<MessageRecord> = [
    { title: '接收对象', dataIndex: 'recipient', key: 'recipient', width: 160 },
    {
      title: '模板', dataIndex: 'templateCode', key: 'templateCode', width: 120,
      render: (v: string) => v || '-',
    },
    {
      title: '内容', dataIndex: 'content', key: 'content',
      render: (v: string) => (
        <Paragraph ellipsis={{ rows: 1, tooltip: v }} style={{ margin: 0, maxWidth: 280 }}>{v}</Paragraph>
      ),
    },
    { title: '渠道', dataIndex: 'channel', key: 'channel', width: 90, render: (v: string) => v || '-' },
    {
      title: '状态', dataIndex: 'sendStatus', key: 'sendStatus', width: 90,
      render: (v: string) => <Tag color={STATUS_COLOR[v] ?? 'default'}>{v}</Tag>,
    },
    {
      title: '发送时间', dataIndex: 'createTime', key: 'createTime', width: 180,
      render: (v: string) => (v ? new Date(v).toLocaleString() : '-'),
    },
    {
      title: '操作', key: 'action', width: 80,
      render: (_, record) => canDelete ? (
        <Button type="link" danger size="small" onClick={() => handleDelete(record)}>删除</Button>
      ) : '—',
    },
  ];

  return (
    <div>
      <ListPageShell<MessageRecord>
        columns={columns}
        tableQuery={tableQuery}
        searchPlaceholder="搜索接收对象 / 模板"
        keyword={searchParams.get('keyword') ?? ''}
        onSearch={tableQuery.setKeyword}
        emptyDescription="暂无发送记录"
        createLabel={canSend ? '发送消息' : undefined}
        onCreate={canSend ? () => setSendOpen(true) : undefined}
        extraActions={
          <Select
            allowClear
            placeholder="按状态筛选"
            style={{ width: 140 }}
            value={searchParams.get('send_status') ?? undefined}
            onChange={(value) => setFilter('send_status', value)}
            options={[
              { value: 'pending', label: '待发送' },
              { value: 'sent', label: '已发送' },
              { value: 'failed', label: '失败' },
            ]}
          />
        }
      />
      <Text type="secondary" style={{ fontSize: 12 }}>发送记录按当前应用隔离</Text>

      <Modal
        title="发送消息"
        open={sendOpen}
        onCancel={() => setSendOpen(false)}
        onOk={handleSend}
        okText="发送"
        confirmLoading={isSubmitting}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="recipient" label="接收对象" rules={[{ required: true, message: '请输入接收对象' }]}>
            <Input placeholder="地址或标识" />
          </Form.Item>
          <Form.Item name="templateCode" label="模板编码">
            <Input placeholder="可选" />
          </Form.Item>
          <Form.Item name="channel" label="渠道">
            <Input placeholder="可选（字典 channel）" />
          </Form.Item>
          <Form.Item name="content" label="消息内容" rules={[{ required: true, message: '请输入内容' }]}>
            <Input.TextArea rows={4} placeholder="渲染后的消息内容" />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
