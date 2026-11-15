import { Form, Input, Button, Radio, Card, message } from "antd";
import { useNavigate } from "react-router-dom";
import { accountApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS } from "../../types/copywriting";

export default function CreateAccountPage() {
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();

  const onFinish = async (values: {
    username: string;
    nickname?: string;
    email?: string;
    password: string;
    status: string;
  }) => {
    run(async () => {
      // 初始密码必填（后端 bcrypt 摘要落库）；email 可空，留空即不设置
      await accountApi.create({
        username: values.username,
        nickname: values.nickname,
        email: values.email,
        password: values.password,
        status: values.status,
      });
      message.success(`${ACTIONS.create}成功`);
      navigate('/account_management/account_list');
    }).catch((error) => showNotification(error, "用户创建失败"));
  };

  return (
    <div>
      <Card title="创建用户">
        <Form
          form={form}
          layout="vertical"
          initialValues={{ status: "enabled" }}
          onFinish={onFinish}
        >
          <Form.Item
            label="用户名"
            name="username"
            rules={[{ required: true, message: "请输入用户名!" }]}
          >
            <Input placeholder="唯一登录名" />
          </Form.Item>

          <Form.Item label="昵称" name="nickname">
            <Input placeholder="显示名（可留空）" />
          </Form.Item>

          <Form.Item
            label="邮箱"
            name="email"
            rules={[{ type: "email", message: "请输入有效的邮箱格式!" }]}
          >
            <Input placeholder="可留空；设置后可用邮箱登录" />
          </Form.Item>

          <Form.Item
            label="初始密码"
            name="password"
            rules={[
              { required: true, message: "请输入初始密码" },
              { min: 8, message: "密码至少 8 位" },
            ]}
          >
            <Input.Password placeholder="至少 8 位" />
          </Form.Item>

          <Form.Item label="状态" name="status" rules={[{ required: true, message: "请选择状态" }]}>
            <Radio.Group>
              <Radio value="enabled">启用</Radio>
              <Radio value="disabled">禁用</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item>
            <Button type="primary" htmlType="submit" loading={isSubmitting}>
              {ACTIONS.create}
            </Button>
            <Button style={{ marginLeft: 8 }} onClick={() => navigate('/account_management/account_list')}>
              {ACTIONS.cancel}
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  );
}
