import { useCallback, useEffect, useMemo } from 'react';
import { useQuery, useQueryClient } from '@tanstack/react-query';
import { useSearchParams } from 'react-router-dom';
import { ApiError } from '../api/request';
import { useDebouncedValue } from './useDebouncedValue';
import type { Page } from '../api/request';

export type TableQueryStatus = 'loading' | 'success' | 'empty' | 'error' | 'forbidden';

interface PagedResponse {
  page: Page;
}

interface UseTableQueryOptions<TResponse extends PagedResponse, T> {
  domain: string;
  fetcher: (currentPage: number, pageSize: number, keyword: string) => Promise<TResponse>;
  getItems: (response: TResponse) => T[];
}

export function useTableQuery<TResponse extends PagedResponse, T extends { id: string }>(
  options: UseTableQueryOptions<TResponse, T>,
) {
  const { domain, fetcher, getItems } = options;
  const [searchParams, setSearchParams] = useSearchParams();
  const queryClient = useQueryClient();

  const currentPage = Math.max(1, Number.parseInt(searchParams.get('current_page') ?? '1', 10) || 1);
  const pageSize = Math.max(1, Number.parseInt(searchParams.get('page_size') ?? '10', 10) || 10);
  const keyword = searchParams.get('keyword') ?? '';
  const debouncedKeyword = useDebouncedValue(keyword.trim());

  const queryKey = useMemo(
    () => [domain, debouncedKeyword, currentPage, pageSize] as const,
    [domain, debouncedKeyword, currentPage, pageSize],
  );

  const query = useQuery({
    queryKey,
    queryFn: () => fetcher(currentPage, pageSize, debouncedKeyword),
    staleTime: 30_000,
    retry: 1,
  });

  const updateParams = useCallback(
    (updates: Record<string, string | null>) => {
      const nextParams = new URLSearchParams(searchParams);
      Object.entries(updates).forEach(([key, value]) => {
        if (value === null || value === '') nextParams.delete(key);
        else nextParams.set(key, value);
      });
      setSearchParams(nextParams, { replace: true });
    },
    [searchParams, setSearchParams],
  );

  const setKeyword = useCallback(
    (nextKeyword: string) => {
      updateParams({ keyword: nextKeyword.trim(), current_page: '1' });
    },
    [updateParams],
  );

  const setPage = useCallback((page: number) => updateParams({ current_page: String(page) }), [updateParams]);
  const setPageSize = useCallback(
    (size: number) => updateParams({ page_size: String(size), current_page: '1' }),
    [updateParams],
  );

  useEffect(() => {
    const totalPages = query.data?.page.totalPages ?? 0;
    if (query.data && currentPage > totalPages && totalPages > 0) {
      setPage(totalPages);
    }
  }, [currentPage, query.data, setPage]);

  const refresh = useCallback(
    async () => {
      await queryClient.invalidateQueries({ queryKey });
    },
    [queryClient, queryKey],
  );

  const items = query.data ? getItems(query.data) : [];

  const status: TableQueryStatus = query.isPending
    ? 'loading'
    : query.isError
      ? query.error instanceof ApiError && query.error.status === 403 ? 'forbidden' : 'error'
      : items.length === 0 ? 'empty' : 'success';

  return {
    items: items ?? [],
    page: query.data?.page,
    status,
    error: query.error,
    setKeyword,
    setPage,
    setPageSize,
    refresh,
    refetch: query.refetch,
  };
}
