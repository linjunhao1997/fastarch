import { useEffect } from "react";
import { Form, Input, Button, Radio, message } from "antd";
import { useNavigate, useParams } from "react-router";
import { applicationApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS } from "../../types/copywriting";

export default function EditApplicationPage() {
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const [form] = Form.useForm();
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();

  useEffect(() => {
    const fetchApplication = async () => {
      if (!id) return;
      try {
        const response = await applicationApi.get(id);
        form.setFieldsValue({
          name: response.name,
          description: response.description,
          enabledStatus: response.enabledStatus || "enabled",
        });
      } catch (error) {
        showNotification(error, "获取应用信息失败");
      }
    };
    fetchApplication();
  }, [id, form, showNotification]);

  const onFinish = async (values: { name: string; description: string; enabledStatus: string }) => {
    if (!id) return;
    run(async () => {
      await applicationApi.update(id, {
        name: values.name,
        description: values.description,
        enabledStatus: values.enabledStatus,
      });
      message.success(`${ACTIONS.save}成功`);
      navigate('/application_management/application_list');
    }).catch((error) => showNotification(error, "应用保存失败"));
  };

  return (
    <div>
      <h2>编辑应用</h2>
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
            {ACTIONS.save}
          </Button>
          <Button onClick={() => navigate('/application_management/application_list')}>
            {ACTIONS.cancel}
          </Button>
        </Form.Item>
      </Form>
    </div>
  );
}
