import { useMemo, useState } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import {
  Button, Card, Drawer, Flex, Form, Input, InputNumber, Modal, Radio, Table, Tag, message,
} from 'antd';
import type { TableColumnsType } from 'antd';
import { CheckCircleOutlined, PlusOutlined, StopOutlined } from '@ant-design/icons';
import { useSearchParams } from 'react-router';
import { dictApi } from '@/shared/api/menu-dict-api';
import { getConsole } from '@/shared/api/app-context';
import { ListPageShell } from '@/shared/components/ListPageShell';
import { useConfirmDelete } from '@/shared/components/ConfirmDelete';
import { useApiErrorNotification } from '@/shared/hooks/useApiError';
import { useSubmitLock } from '@/shared/hooks/useSubmitLock';
import { useTableQuery } from '@/shared/hooks/useTableQuery';
import type { DictItem, DictType } from '@/shared/types';
import { ACTIONS, buildSuccessMessage } from '@/shared/types/copywriting';
import { formatDateTime } from '@/shared/utils/datetime';

interface DictTypeFormValues {
  code: string;
  name: string;
  remark?: string;
  status: string;
}

interface DictItemFormValues {
  label: string;
  value: string;
  sort?: number;
  status: string;
}

const StatusTag = ({ status }: { status: string }) =>
  status === 'enabled'
    ? <Tag icon={<CheckCircleOutlined />}>启用</Tag>
    : <Tag icon={<StopOutlined />}>禁用</Tag>;

export default function DictManagementPage() {
  const [searchParams] = useSearchParams();
  const queryClient = useQueryClient();
  const showNotification = useApiErrorNotification();
  const confirmDelete = useConfirmDelete();
  const { isSubmitting, run } = useSubmitLock();

  const [typeForm] = Form.useForm<DictTypeFormValues>();
  const [editingType, setEditingType] = useState<{ mode: 'create' | 'edit'; type?: DictType } | null>(null);

  const [itemForm] = Form.useForm<DictItemFormValues>();
  const [drawerType, setDrawerType] = useState<DictType | null>(null);
  const [editingItem, setEditingItem] = useState<{ mode: 'create' | 'edit'; item?: DictItem } | null>(null);

  const tableQuery = useTableQuery({
    domain: 'dict-types',
    fetcher: (currentPage, pageSize, keyword) => dictApi.listTypes(currentPage, pageSize, keyword),
    getItems: (response) => response.dictTypes,
  });

  const itemsQuery = useQuery({
    // 缓存按应用隔离（#9），与 useTableQuery 同策略
    queryKey: ['dict-items', getConsole(), drawerType?.id],
    queryFn: () => dictApi.listItems(drawerType?.id ?? ''),
    enabled: drawerType !== null,
  });

  const saveType = async (values: DictTypeFormValues) => {
    run(async () => {
      if (editingType?.mode === 'edit' && editingType.type) {
        await dictApi.updateType(editingType.type.id, {
          name: values.name,
          remark: values.remark,
          status: values.status,
          version: editingType.type.version,
        });
      } else {
        await dictApi.createType(values);
      }
      message.success(buildSuccessMessage(editingType?.mode === 'edit' ? ACTIONS.save : ACTIONS.create));
      setEditingType(null);
      typeForm.resetFields();
      await tableQuery.refresh();
    }).catch((error) => showNotification(error, '字典类型保存失败'));
  };

  const saveItem = async (values: DictItemFormValues) => {
    run(async () => {
      if (editingItem?.mode === 'edit' && editingItem.item) {
        await dictApi.updateItem(editingItem.item.id, {
          label: values.label,
          value: values.value,
          sort: values.sort ?? 0,
          status: values.status,
          version: editingItem.item.version,
        });
      } else if (drawerType) {
        await dictApi.createItem({ typeId: drawerType.id, ...values });
      }
      message.success(buildSuccessMessage(editingItem?.mode === 'edit' ? ACTIONS.save : ACTIONS.create));
      setEditingItem(null);
      itemForm.resetFields();
      await queryClient.invalidateQueries({ queryKey: ['dict-items', getConsole(), drawerType?.id] });
    }).catch((error) => showNotification(error, '字典条目保存失败'));
  };

  const openTypeModal = (state: { mode: 'create' | 'edit'; type?: DictType }) => {
    setEditingType(state);
    if (state.mode === 'edit' && state.type) {
      typeForm.setFieldsValue({
        code: state.type.code,
        name: state.type.name,
        remark: state.type.remark,
        status: state.type.status,
      });
    } else {
      typeForm.resetFields();
      typeForm.setFieldsValue({ status: 'enabled' });
    }
  };

  const openItemModal = (state: { mode: 'create' | 'edit'; item?: DictItem }) => {
    setEditingItem(state);
    if (state.mode === 'edit' && state.item) {
      itemForm.setFieldsValue({
        label: state.item.label,
        value: state.item.value,
        sort: state.item.sort,
        status: state.item.status,
      });
    } else {
      itemForm.resetFields();
      itemForm.setFieldsValue({ status: 'enabled', sort: 0 });
    }
  };

  const columns: TableColumnsType<DictType> = useMemo(() => [
    { title: '类型编码', dataIndex: 'code', key: 'code' },
    { title: '类型名称', dataIndex: 'name', key: 'name' },
    { title: '备注', dataIndex: 'remark', key: 'remark', render: (value: string) => value || '—' },
    { title: '状态', dataIndex: 'status', key: 'status', render: (status: string) => <StatusTag status={status} /> },
    { title: '创建时间', dataIndex: 'createTime', key: 'createTime', render: formatDateTime },
    {
      title: '操作', key: 'action', width: 240,
      render: (_: unknown, record: DictType) => (
        <Flex gap={0}>
          <Button type="link" size="small" onClick={() => setDrawerType(record)}>管理条目</Button>
          <Button type="link" size="small" onClick={() => openTypeModal({ mode: 'edit', type: record })}>
            {ACTIONS.edit}
          </Button>
          <Button
            type="link"
            size="small"
            danger
            onClick={() => confirmDelete({
              entityType: '字典类型',
              entityName: record.name,
              onDelete: async () => {
                await dictApi.deleteType(record.id);
                await tableQuery.refresh();
                message.success(buildSuccessMessage(ACTIONS.delete));
              },
              onError: (error) => showNotification(error, '删除字典类型失败'),
            })}
          >
            {ACTIONS.delete}
          </Button>
        </Flex>
      ),
    },
  ], [confirmDelete, showNotification, tableQuery]);

  const itemColumns: TableColumnsType<DictItem> = useMemo(() => [
    { title: '标签', dataIndex: 'label', key: 'label' },
    { title: '取值', dataIndex: 'value', key: 'value' },
    { title: '排序', dataIndex: 'sort', key: 'sort', width: 80 },
    { title: '状态', dataIndex: 'status', key: 'status', width: 100, render: (status: string) => <StatusTag status={status} /> },
    {
      title: '操作', key: 'action', width: 160,
      render: (_: unknown, record: DictItem) => (
        <Flex gap={0}>
          <Button type="link" size="small" onClick={() => openItemModal({ mode: 'edit', item: record })}>
            {ACTIONS.edit}
          </Button>
          <Button
            type="link"
            size="small"
            danger
            onClick={() => confirmDelete({
              entityType: '字典条目',
              entityName: record.label,
              onDelete: async () => {
                await dictApi.deleteItem(record.id);
                await queryClient.invalidateQueries({ queryKey: ['dict-items', getConsole(), drawerType?.id] });
                message.success(buildSuccessMessage(ACTIONS.delete));
              },
              onError: (error) => showNotification(error, '删除字典条目失败'),
            })}
          >
            {ACTIONS.delete}
          </Button>
        </Flex>
      ),
    },
  ], [confirmDelete, drawerType, queryClient, showNotification]);

  return (
    <div>
      <ListPageShell<DictType>
        columns={columns}
        tableQuery={tableQuery}
        searchPlaceholder="搜索类型编码或名称"
        keyword={searchParams.get('keyword') ?? ''}
        onSearch={tableQuery.setKeyword}
        emptyDescription="暂无字典类型"
        createLabel="新建字典类型"
        onCreate={() => openTypeModal({ mode: 'create' })}
      />

      <Modal
        title={editingType?.mode === 'edit' ? `编辑字典类型：${editingType?.type?.name ?? ''}` : '新建字典类型'}
        open={editingType !== null}
        onCancel={() => setEditingType(null)}
        onOk={() => typeForm.submit()}
        confirmLoading={isSubmitting}
        destroyOnClose
      >
        <Form form={typeForm} layout="vertical" onFinish={saveType}>
          <Form.Item
            label="类型编码"
            name="code"
            rules={[{ required: true, message: '请输入类型编码' }]}
            tooltip={editingType?.mode === 'edit' ? '编码创建后不可修改' : undefined}
          >
            <Input placeholder="唯一业务码，如 gender" disabled={editingType?.mode === 'edit'} />
          </Form.Item>
          <Form.Item label="类型名称" name="name" rules={[{ required: true, message: '请输入类型名称' }]}>
            <Input placeholder="展示名称" />
          </Form.Item>
          <Form.Item label="备注" name="remark">
            <Input.TextArea rows={2} maxLength={255} />
          </Form.Item>
          <Form.Item label="状态" name="status" rules={[{ required: true }]}>
            <Radio.Group>
              <Radio value="enabled">启用</Radio>
              <Radio value="disabled">禁用</Radio>
            </Radio.Group>
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title={`字典条目：${drawerType?.name ?? ''}`}
        width={640}
        open={drawerType !== null}
        onClose={() => setDrawerType(null)}
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={() => openItemModal({ mode: 'create' })}>新建条目</Button>}
      >
        <Card size="small">
          <Table<DictItem>
            rowKey="id"
            columns={itemColumns}
            dataSource={itemsQuery.data?.dictItems ?? []}
            loading={itemsQuery.isPending && drawerType !== null}
            pagination={false}
          />
        </Card>
      </Drawer>

      <Modal
        title={editingItem?.mode === 'edit' ? `编辑条目：${editingItem?.item?.label ?? ''}` : '新建条目'}
        open={editingItem !== null}
        onCancel={() => setEditingItem(null)}
        onOk={() => itemForm.submit()}
        confirmLoading={isSubmitting}
        destroyOnClose
      >
        <Form form={itemForm} layout="vertical" onFinish={saveItem}>
          <Form.Item label="标签" name="label" rules={[{ required: true, message: '请输入标签' }]}>
            <Input placeholder="展示文本" />
          </Form.Item>
          <Form.Item label="取值" name="value" rules={[{ required: true, message: '请输入取值' }]}>
            <Input placeholder="业务值" />
          </Form.Item>
          <Form.Item label="排序" name="sort">
            <InputNumber min={0} style={{ width: '100%' }} />
          </Form.Item>
          <Form.Item label="状态" name="status" rules={[{ required: true }]}>
            <Radio.Group>
              <Radio value="enabled">启用</Radio>
              <Radio value="disabled">禁用</Radio>
            </Radio.Group>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
}
