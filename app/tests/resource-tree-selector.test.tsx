import { describe, it, expect, vi, beforeEach } from 'vitest';
import { render, screen, fireEvent, waitFor } from '@testing-library/react';
import { Form, ConfigProvider } from 'antd';
import ResourceTreeSelector from '../src/components/ResourceTreeSelector';
import { resourceApi } from '../src/api/iam';

const { listTreeMock } = vi.hoisted(() => ({ listTreeMock: vi.fn() }));
vi.mock('../src/api/iam', () => ({
  resourceApi: {
    listTree: listTreeMock,
  },
}));

const tree = {
  roots: [
    {
      resource: { id: '1', name: '菜单A', kind: 'menu', enabledStatus: 'enabled' },
      children: [
        {
          resource: { id: '2', name: '接口B', kind: 'http_api', enabledStatus: 'enabled' },
          children: [],
        },
        {
          resource: { id: '3', name: '接口C', kind: 'http_api', enabledStatus: 'enabled' },
          children: [],
        },
      ],
    },
  ],
};

function rowState(title: string): boolean {
  const row = Array.from(document.querySelectorAll('.ant-tree-treenode')).find((r) =>
    r.querySelector('.ant-tree-title')?.textContent === title,
  );
  const cb = row?.querySelector('.ant-tree-checkbox');
  return !!cb && cb.className.includes('ant-tree-checkbox-checked');
}

function checkNode(title: string) {
  const row = Array.from(document.querySelectorAll('.ant-tree-treenode')).find((r) =>
    r.querySelector('.ant-tree-title')?.textContent === title,
  );
  fireEvent.click(row!.querySelector('.ant-tree-checkbox')!);
}

describe('ResourceTreeSelector 勾选链路', () => {
  beforeEach(() => {
    listTreeMock.mockResolvedValue(tree);
  });

  it('编辑页回显（string[] 初始值）：checkStrictly 下勾选互不联动', async () => {
    const FormHarness = () => {
      const [form] = Form.useForm();
      return (
        <ConfigProvider>
          <Form form={form} initialValues={{ resourceIds: ['3'] }}>
            <Form.Item label="绑定资源" name="resourceIds" rules={[{ required: true }]}>
              <ResourceTreeSelector applicationId="1" />
            </Form.Item>
          </Form>
        </ConfigProvider>
      );
    };
    render(<FormHarness />);
    await waitFor(() => expect(screen.getByText('接口B')).toBeTruthy());
    // 初始回显：checkStrictly 不做祖先联动，仅 3 被勾选
    expect(rowState('菜单A')).toBe(false);
    expect(rowState('接口B')).toBe(false);
    expect(rowState('接口C')).toBe(true);

    checkNode('接口B');
    await waitFor(() => expect(rowState('接口B')).toBe(true));
    // 已选的 3 不应被波及
    expect(rowState('接口C')).toBe(true);
    expect(rowState('菜单A')).toBe(false);

    // 取消 B：B 移除，C 保留
    checkNode('接口B');
    await waitFor(() => expect(rowState('接口B')).toBe(false));
    expect(rowState('接口C')).toBe(true);

    // 取消 C：C 移除，其余不联动
    checkNode('接口C');
    await waitFor(() => expect(rowState('接口C')).toBe(false));
    expect(rowState('菜单A')).toBe(false);
  });

  it('value 为 undefined（创建页初始态）时不崩溃且可勾选', async () => {
    const FormHarness = () => {
      const [form] = Form.useForm();
      return (
        <ConfigProvider>
          <Form form={form}>
            <Form.Item label="绑定资源" name="resourceIds" rules={[{ required: true }]}>
              <ResourceTreeSelector applicationId="1" />
            </Form.Item>
          </Form>
        </ConfigProvider>
      );
    };
    render(<FormHarness />);
    await waitFor(() => expect(screen.getByText('接口C')).toBeTruthy());
    expect(rowState('接口C')).toBe(false);

    checkNode('接口C');
    await waitFor(() => expect(rowState('接口C')).toBe(true));
    // checkStrictly：勾选 C 不联动祖先与兄弟
    expect(rowState('菜单A')).toBe(false);
    expect(rowState('接口B')).toBe(false);
  });
});
