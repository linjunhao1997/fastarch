import { Button, Empty, Flex } from 'antd';

interface EmptyStateAction {
  text: string;
  onClick: () => void;
}

interface EmptyStateProps {
  description: string;
  action?: EmptyStateAction;
}

export function EmptyState({ description, action }: EmptyStateProps) {
  return (
    <Flex justify="center" align="center" style={{ padding: '48px 0' }}>
      <Empty description={description} image={Empty.PRESENTED_IMAGE_SIMPLE}>
        {action ? <Button type="primary" onClick={action.onClick}>{action.text}</Button> : null}
      </Empty>
    </Flex>
  );
}