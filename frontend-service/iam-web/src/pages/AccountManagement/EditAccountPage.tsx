import { useCallback, useEffect, useState } from "react";
import { Form, Input, Button, Card, Radio, Select, message } from "antd";
import { useNavigate, useParams } from "react-router-dom";
import { accountApi } from "../../api/iam";
import { useApplicationOptions } from "../../hooks/useApplicationOptions";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS } from "../../types/copywriting";

export default function EditAccountPage() {
  const [form] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();
  const [version, setVersion] = useState<number>(0);
  const { applications, loading: applicationsLoading } = useApplicationOptions();

  const fetchAccount = useCallback(async () => {
    try {
      const response = await accountApi.get(id || "");
      form.setFieldsValue({
        name: response.name,
        email: response.email,
        status: response.status || "enabled",
        applicationIdList: response.applicationIdList ?? [],
      });
      setVersion(response.version);
    } catch (error) {
      showNotification(error, "获取账号信息失败");
    } finally {
      setLoading(false);
    }
  }, [form, id, showNotification]);

  useEffect(() => {
    void fetchAccount();
  }, [fetchAccount]);

  const onFinish = async (values: {
    name?: string;
    email?: string;
    status?: string;
    applicationIdList?: string[];
  }) => {
    run(async () => {
      // application_id_list 为全量替换语义：始终提交当前选中集合（含空数组=清空归属）
      await accountApi.update(id || "", {
        ...values,
        status: values.status,
        applicationIdList: values.applicationIdList ?? [],
        version,
      });
      message.success(`${ACTIONS.save}成功`);
      navigate('/account_management/account_list');
    }).catch((error) => showNotification(error, "账号保存失败"));
  };

  return (
    <div>
      <Card title="编辑账号">
        <Form
          form={form}
          layout="vertical"
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

        <Form.Item label="版本" name="version" hidden initialValue={version}>
          <Input />
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
            <Button type="primary" htmlType="submit" loading={isSubmitting} disabled={loading}>
              {ACTIONS.save}
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
