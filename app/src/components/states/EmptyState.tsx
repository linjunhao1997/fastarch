import { Button, Empty } from 'antd';

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
    <Empty description={description}>
      {action ? <Button type="primary" onClick={action.onClick}>{action.text}</Button> : null}
    </Empty>
  );
}
