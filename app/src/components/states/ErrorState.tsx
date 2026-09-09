import { Button, Result } from 'antd';

interface ErrorStateProps {
  message: string;
  onRetry: () => void;
}

export function ErrorState({ message, onRetry }: ErrorStateProps) {
  return <Result status="error" title={message} extra={<Button onClick={onRetry}>重试</Button>} />;
}
