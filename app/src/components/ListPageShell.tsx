import type { ReactNode } from 'react';
import { Button, Flex, Input, Table } from 'antd';
import type { TableColumnsType } from 'antd';
import { EmptyState, ErrorState } from './states';
import { ACTIONS } from '../types/copywriting';

type TableQuery<T extends { id: string }> = {
  items: T[];
  page?: { currentPage: number; pageSize: number; total: number };
  status: 'loading' | 'success' | 'empty' | 'error' | 'forbidden';
  setKeyword: (keyword: string) => void;
  setPage: (page: number) => void;
  setPageSize: (pageSize: number) => void;
  refetch: () => Promise<unknown>;
};

interface ListPageShellProps<T extends { id: string }> {
  columns: TableColumnsType<T>;
  tableQuery: TableQuery<T>;
  searchPlaceholder: string;
  keyword: string;
  onSearch: (keyword: string) => void;
  emptyDescription: string;
  createLabel?: string;
  onCreate?: () => void;
  extraActions?: ReactNode;
}

export function ListPageShell<T extends { id: string }>({
  columns,
  tableQuery,
  searchPlaceholder,
  keyword,
  onSearch,
  emptyDescription,
  createLabel,
  onCreate,
  extraActions,
}: ListPageShellProps<T>) {
  return (
    <div>
      <Flex gap={16} style={{ marginBottom: 16 }}>
        <Input.Search
          placeholder={searchPlaceholder}
          defaultValue={keyword}
          onSearch={onSearch}
          style={{ width: 240 }}
        />
        <Button onClick={() => onSearch('')}>{ACTIONS.reset}</Button>
        {createLabel && onCreate ? (
          <Button type="primary" onClick={onCreate}>{createLabel}</Button>
        ) : null}
        {extraActions}
      </Flex>
      {tableQuery.status === 'loading' ? (
        <Table<T> columns={columns} dataSource={[]} rowKey="id" loading pagination={false} />
      ) : tableQuery.status === 'error' ? (
        <ErrorState message="请求失败" onRetry={() => { void tableQuery.refetch(); }} />
      ) : tableQuery.status === 'forbidden' ? (
        <ErrorState message="无权限" onRetry={() => { void tableQuery.refetch(); }} />
      ) : tableQuery.status === 'empty' ? (
        <EmptyState description={emptyDescription} action={onCreate && createLabel ? { text: createLabel, onClick: onCreate } : undefined} />
      ) : (
        <Table<T>
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
    </div>
  );
}
