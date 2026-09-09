import { Form, Input, Button, Radio, message } from "antd";
import { useNavigate } from "react-router";
import { applicationApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS } from "../../types/copywriting";

export default function CreateApplicationPage() {
  const navigate = useNavigate();
  const [form] = Form.useForm();
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();

  const onFinish = async (values: { name: string; description: string; enabledStatus: string }) => {
    run(async () => {
      await applicationApi.create({
        name: values.name,
        description: values.description,
        enabledStatus: values.enabledStatus,
      });
      message.success(`${ACTIONS.create}成功`);
      navigate('/application_management/application_list');
    }).catch((error) => showNotification(error, "应用创建失败"));
  };

  return (
    <div>
      <h2>创建应用</h2>
      <Form
        form={form}
        layout="vertical"
        onFinish={onFinish}
        initialValues={{
          name: "",
          description: "",
          enabledStatus: "enabled",
        }}
      >
        <Form.Item
          label="应用名称"
          name="name"
          rules={[{ required: true, message: "请输入应用名称" }]}
        >
          <Input />
        </Form.Item>
        <Form.Item label="描述" name="description">
          <Input.TextArea />
        </Form.Item>
        <Form.Item label="状态" name="enabledStatus" rules={[{ required: true, message: "请选择状态" }]}>
          <Radio.Group>
            <Radio value="enabled">启用</Radio>
            <Radio value="disabled">禁用</Radio>
          </Radio.Group>
        </Form.Item>
        <Form.Item>
          <Button type="primary" htmlType="submit" loading={isSubmitting}>
            {ACTIONS.create}
          </Button>
          <Button onClick={() => navigate('/application_management/application_list')}>
            {ACTIONS.cancel}
          </Button>
        </Form.Item>
      </Form>
    </div>
  );
}
