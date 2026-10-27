import { useEffect, useState, useCallback } from 'react';
import { Tree } from 'antd';
import type { TreeDataNode } from 'antd';
import { resourceApi } from '../api/iam';

export interface ResourceTreeSelectorProps {
  applicationId: string;
  /** 资源 id 一律用字符串：后端 int64 超出 JS 安全整数范围，Number 转换会丢精度 */
  value: (string | number)[];
  onChange: (value: string[]) => void;
}

// Session-persistent expand state (resets on new session per spec clarification)
function useSessionExpandKeys(key: string) {
  const getInitial = (): string[] => {
    try {
      return JSON.parse(sessionStorage.getItem(key) || '[]');
    } catch {
      return [];
    }
  };
  const [expandedKeys, setExpandedKeysState] = useState<string[]>(getInitial);
  const setExpandedKeys = useCallback((keys: React.Key[]) => {
    const strKeys = keys.map(String);
    setExpandedKeysState(strKeys);
    sessionStorage.setItem(key, JSON.stringify(strKeys));
  }, [key]);
  return [expandedKeys, setExpandedKeys] as const;
}

export default function ResourceTreeSelector({ applicationId, value, onChange }: ResourceTreeSelectorProps) {
  const [treeData, setTreeData] = useState<TreeDataNode[]>([]);
  const [expandedKeys, setExpandedKeys] = useSessionExpandKeys(`resource-selector-expand-${applicationId}`);

  useEffect(() => {
    if (!applicationId) {
      setTreeData([]);
      return;
    }
    resourceApi.listTree(applicationId).then((response) => {
      const ids: string[] = [];
      const convert = (nodes: typeof response.roots): TreeDataNode[] =>
        nodes.map((node) => {
          ids.push(node.resource.id);
          return {
            key: node.resource.id,
            title: node.resource.name,
            children: convert(node.children),
          };
        });
      const data = convert(response.roots);
      setTreeData(data);
      if (ids.length > 0 && expandedKeys.length === 0) {
        setExpandedKeys(ids);
      }
    });
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [applicationId]);

  return (
    <Tree
      checkable
      checkStrictly
      treeData={treeData}
      expandedKeys={expandedKeys}
      onExpand={setExpandedKeys}
      checkedKeys={(value ?? []).map(String)}
      onCheck={(checked) => {
        const keys = Array.isArray(checked) ? checked : checked.checked;
        onChange(keys.map(String));
      }}
    />
  );
}
