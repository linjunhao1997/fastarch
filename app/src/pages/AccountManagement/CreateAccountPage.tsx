import { Form, Input, Button, Radio, Select, Card, message } from "antd";
import { useNavigate } from "react-router-dom";
import { accountApi } from "../../api/iam";
import { useApplicationOptions } from "../../hooks/useApplicationOptions";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS } from "../../types/copywriting";

export default function CreateAccountPage() {
  const [form] = Form.useForm();
  const navigate = useNavigate();
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();
  const { applications, loading: applicationsLoading } = useApplicationOptions();

  const onFinish = async (values: {
    name: string;
    email: string;
    password: string;
    kind: string;
    status: string;
    applicationIdList?: string[];
  }) => {
    run(async () => {
      const kindStr = values.kind === "user" ? "0" : "1";
      await accountApi.create({
        name: values.name,
        email: values.email,
        password: values.password,
        kind: kindStr,
        status: values.status,
        applicationIdList: values.applicationIdList ?? [],
      });
      message.success(`${ACTIONS.create}成功`);
      navigate('/account_management/account_list');
    }).catch((error) => showNotification(error, "账号创建失败"));
  };

  return (
    <div>
      <Card title="创建账号">
        <Form
          form={form}
          layout="vertical"
          initialValues={{ status: "enabled" }}
          onFinish={onFinish}
        >
          <Form.Item
            label="姓名"
            name="name"
            rules={[{ required: true, message: "请输入姓名!" }]}
          >
            <Input placeholder="请输入姓名" />
          </Form.Item>

          <Form.Item
            label="邮箱"
            name="email"
            rules={[
              { required: true, message: "请输入邮箱!" },
              { type: "email", message: "请输入有效的邮箱格式!" }
            ]}
          >
            <Input placeholder="请输入邮箱" />
          </Form.Item>

          <Form.Item
            label="密码"
            name="password"
            rules={[{ required: true, message: "请输入密码" }]}
          >
            <Input.Password placeholder="请输入密码" />
          </Form.Item>

          <Form.Item
            label="类型"
            name="kind"
            rules={[{ required: true, message: "请选择类型!" }]}
          >
            <Select placeholder="请选择类型">
              <Select.Option value="user">用户</Select.Option>
              <Select.Option value="service">服务</Select.Option>
            </Select>
          </Form.Item>

          <Form.Item label="状态" name="status" rules={[{ required: true, message: "请选择状态" }]}>
            <Radio.Group>
              <Radio value="enabled">启用</Radio>
              <Radio value="disabled">禁用</Radio>
            </Radio.Group>
          </Form.Item>

          <Form.Item label="所属应用" name="applicationIdList">
            <Select
              mode="multiple"
              placeholder="请选择所属应用（可多选，可留空）"
              loading={applicationsLoading}
              options={applications.map((app) => ({
                value: app.id,
                label: app.enabledStatus === 'enabled' ? app.name : `${app.name}（已停用）`,
                disabled: app.enabledStatus !== 'enabled',
              }))}
            />
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
