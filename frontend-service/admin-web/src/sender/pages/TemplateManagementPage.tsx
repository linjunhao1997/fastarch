import { useState } from 'react';
import { useMutation } from '@tanstack/react-query';
import { Button, Form, Input, Modal, Space, Typography } from 'antd';
import type { TableColumnsType } from 'antd';
import { templateApi } from '@/sender/api/sender';
import { ListPageShell } from '@/shared/components/ListPageShell';
import { useConfirmDelete } from '@/shared/components/ConfirmDelete';
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { usePermissions } from '@/shared/hooks/permission-context';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import { useTableQuery } from '@/shared/hooks/useTableQuery';
import { useSearchParams } from 'react-router-dom';
import type { MessageTemplate } from '@/sender/types/sender';
import { PERMISSIONS } from '@/shared/types/permissions';

const { Paragraph, Text } = Typography;

interface TemplateFormValues {
  code: string;
  name: string;
  content: string;
  remark?: string;
}

/** 消息模板管理（应用内 CRUD，乐观锁由后端版本号校验） */
export default function TemplateManagementPage() {
  const showNotification = useApiErrorNotification();
  const confirmDelete = useConfirmDelete();
  const { hasPermission } = usePermissions();
  const canCreate = hasPermission(PERMISSIONS.senderTemplateCreate);
  const canUpdate = hasPermission(PERMISSIONS.senderTemplateUpdate);
  const canDelete = hasPermission(PERMISSIONS.senderTemplateDelete);
  const { isSubmitting, run } = useSubmitLock();
  const [form] = Form.useForm<TemplateFormValues>();
  const [editing, setEditing] = useState<MessageTemplate | null>(null);
  const [open, setOpen] = useState(false);
  const [searchParams] = useSearchParams();

  const tableQuery = useTableQuery({
    domain: 'sender-templates',
    fetcher: (currentPage, pageSize, keyword) => templateApi.list({ currentPage, pageSize, keyword }),
    getItems: (res) => res.templates,
  });

  const saveMutation = useMutation({
    mutationFn: async (values: TemplateFormValues) => {
      if (editing) {
        await templateApi.update(editing.id, { ...values, id: editing.id, version: editing.version });
      } else {
        await templateApi.create(values);
      }
    },
  });

  const openCreate = () => {
    setEditing(null);
    form.resetFields();
    setOpen(true);
  };

  const openEdit = (record: MessageTemplate) => {
    setEditing(record);
    form.setFieldsValue({
      code: record.code,
      name: record.name,
      content: record.content,
      remark: record.remark,
    });
    setOpen(true);
  };

  const handleSave = async () => {
    const values = await form.validateFields();
    await run(async () => {
      try {
        await saveMutation.mutateAsync(values);
        setOpen(false);
        await tableQuery.refresh();
      } catch (error) {
        showNotification(error, '保存失败');
      }
    });
  };

  const handleDelete = (record: MessageTemplate) => {
    confirmDelete({
      entityType: '消息模板',
      entityName: record.name,
      onDelete: async () => {
        try {
          await templateApi.delete(record.id);
          await tableQuery.refresh();
        } catch (error) {
          showNotification(error, '删除失败');
          throw error;
        }
      },
    });
  };

  const columns: TableColumnsType<MessageTemplate> = [
    { title: '编码', dataIndex: 'code', key: 'code', width: 140 },
    { title: '名称', dataIndex: 'name', key: 'name', width: 140 },
    {
      title: '内容', dataIndex: 'content', key: 'content',
      render: (v: string) => (
        <Paragraph ellipsis={{ rows: 1, tooltip: v }} style={{ margin: 0, maxWidth: 300 }}>{v}</Paragraph>
      ),
    },
    { title: '备注', dataIndex: 'remark', key: 'remark', render: (v: string) => v || '-' },
    {
      title: '操作', key: 'action', width: 140,
      render: (_, record) => (
        <Space size="small">
          {canUpdate ? (
            <Button type="link" size="small" onClick={() => openEdit(record)}>编辑</Button>
          ) : null}
          {canDelete ? (
            <Button type="link" danger size="small" onClick={() => handleDelete(record)}>删除</Button>
          ) : null}
        </Space>
      ),
    },
  ];

  return (
    <div>
      <ListPageShell<MessageTemplate>
        columns={columns}
        tableQuery={tableQuery}
        searchPlaceholder="搜索编码 / 名称"
        keyword={searchParams.get('keyword') ?? ''}
        onSearch={tableQuery.setKeyword}
        emptyDescription="暂无消息模板"
        createLabel={canCreate ? '新建模板' : undefined}
        onCreate={canCreate ? openCreate : undefined}
      />
      <Text type="secondary" style={{ fontSize: 12 }}>模板按当前应用隔离，编码应用内唯一</Text>

      <Modal
        title={editing ? '编辑模板' : '新建模板'}
        open={open}
        onCancel={() => setOpen(false)}
        onOk={handleSave}
        okText="保存"
        confirmLoading={isSubmitting}
        destroyOnClose
      >
        <Form form={form} layout="vertical" preserve={false}>
          <Form.Item name="code" label="编码" rules={[{ required: true, message: '请输入编码' }]}>
            <Input placeholder="应用内唯一" disabled={!!editing} />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input />
          </Form.Item>
          <Form.Item name="content" label="内容">
            <Input.TextArea rows={4} />
          </Form.Item>
          <Form.Item name="remark" label="备注">
            <Input />
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
