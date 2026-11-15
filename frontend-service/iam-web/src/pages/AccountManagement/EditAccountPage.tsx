import { useCallback, useEffect, useState } from "react";
import { Form, Input, Button, Card, Divider, Radio, message } from "antd";
import { useNavigate, useParams } from "react-router-dom";
import { accountApi } from "../../api/iam";
import { useApiErrorNotification } from "../../hooks/useApiError";
import { useSubmitLock } from "../../hooks/useSubmitLock";
import { ACTIONS } from "../../types/copywriting";

export default function EditAccountPage() {
  const [form] = Form.useForm();
  const [pwdForm] = Form.useForm();
  const [loading, setLoading] = useState(true);
  const navigate = useNavigate();
  const { id } = useParams<{ id: string }>();
  const showNotification = useApiErrorNotification();
  const { isSubmitting, run } = useSubmitLock();
  const { isSubmitting: isPwdSubmitting, run: runPwd } = useSubmitLock();
  const [version, setVersion] = useState<number>(0);
  const [username, setUsername] = useState<string>("");

  const fetchAccount = useCallback(async () => {
    try {
      const response = await accountApi.get(id || "");
      form.setFieldsValue({
        username: response.username,
        nickname: response.nickname,
        email: response.email,
        status: response.status || "enabled",
      });
      setUsername(response.username);
      setVersion(response.version);
    } catch (error) {
      showNotification(error, "获取用户信息失败");
    } finally {
      setLoading(false);
    }
  }, [form, id, showNotification]);

  useEffect(() => {
    void fetchAccount();
  }, [fetchAccount]);

  const onFinish = async (values: {
    nickname?: string;
    email?: string;
    status?: string;
  }) => {
    run(async () => {
      // username 不可变更（唯一登录标识）；仅昵称/邮箱/状态可编辑
      await accountApi.update(id || "", {
        nickname: values.nickname,
        email: values.email,
        status: values.status,
        version,
      });
      message.success(`${ACTIONS.save}成功`);
      navigate('/account_management/account_list');
    }).catch((error) => showNotification(error, "用户保存失败"));
  };

  const onResetPassword = async (values: { newPassword: string }) => {
    runPwd(async () => {
      await accountApi.resetPassword(id || "", { id: id || "", newPassword: values.newPassword });
      message.success("密码重置成功");
      pwdForm.resetFields();
    }).catch((error) => showNotification(error, "密码重置失败"));
  };

  return (
    <div>
      <Card title={`编辑用户${username ? `：${username}` : ""}`}>
        <Form form={form} layout="vertical" onFinish={onFinish}>
          <Form.Item label="用户名" name="username">
            <Input disabled title="用户名为唯一登录标识，不可修改" />
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

          <Form.Item label="状态" name="status" rules={[{ required: true, message: "请选择状态" }]}>
            <Radio.Group>
              <Radio value="enabled">启用</Radio>
              <Radio value="disabled">禁用</Radio>
            </Radio.Group>
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

        <Divider />

        <Card size="small" title="重置密码">
          <Form form={pwdForm} layout="inline" onFinish={onResetPassword}>
            <Form.Item
              name="newPassword"
              rules={[
                { required: true, message: "请输入新密码" },
                { min: 8, message: "密码至少 8 位" },
              ]}
            >
              <Input.Password placeholder="新密码（至少 8 位）" style={{ width: 240 }} />
            </Form.Item>
            <Form.Item>
              <Button type="primary" htmlType="submit" loading={isPwdSubmitting}>
                重置密码
              </Button>
            </Form.Item>
          </Form>
        </Card>
      </Card>
    </div>
  );
}
