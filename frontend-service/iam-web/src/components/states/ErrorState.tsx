import { Button, Result, Flex } from 'antd';

interface ErrorStateProps {
  message: string;
  onRetry: () => void;
}

export function ErrorState({ message, onRetry }: ErrorStateProps) {
  return (
    <Flex justify="center" align="center" style={{ padding: '48px 0' }}>
      <Result
        status="error"
        title="请求失败"
        subTitle={message}
        extra={<Button type="primary" onClick={onRetry}>重试</Button>}
      />
    </Flex>
  );
}