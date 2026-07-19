import { useEffect, useState } from 'react'
import { useNavigate } from 'react-router-dom'
import { Card, Form, Input, InputNumber, Select, AutoComplete, Button, message, Spin, Typography } from 'antd'
import { ArrowLeftOutlined } from '@ant-design/icons'
import { createBot, getLLMProviders, getProviderModels, LLMProvider } from '../api'

const { Title } = Typography

const platformOptions = [
  { value: 'wecom', label: '企业微信' },
  { value: 'dingtalk', label: '钉钉' },
]

export default function BotNew() {
  const [providers, setProviders] = useState<LLMProvider[]>([])
  const [modelOptions, setModelOptions] = useState<{ value: string }[]>([])
  const [loading, setLoading] = useState(true)
  const [submitting, setSubmitting] = useState(false)
  const [platformType, setPlatformType] = useState('wecom')
  const [form] = Form.useForm()
  const navigate = useNavigate()

  useEffect(() => {
    getLLMProviders()
      .then(res => setProviders(res.data))
      .catch(() => message.error('加载 Provider 列表失败'))
      .finally(() => setLoading(false))
  }, [])

  const handleProviderChange = (providerId: string) => {
    form.setFieldValue('llm_model', undefined)
    setModelOptions([])
    if (!providerId) return
    getProviderModels(providerId)
      .then(res => setModelOptions(res.data.models.map(m => ({ value: m }))))
      .catch(() => {})
  }

  const handleSubmit = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)

      const platformConfig: Record<string, string> = {}
      if (platformType === 'wecom') {
        platformConfig.bot_id = values.wecom_bot_id || ''
        platformConfig.secret = values.wecom_secret || ''
      } else if (platformType === 'dingtalk') {
        platformConfig.client_id = values.dingtalk_client_id || ''
        platformConfig.client_secret = values.dingtalk_client_secret || ''
      }

      const payload = {
        id: values.id,
        name: values.name,
        platform_type: platformType,
        platform_config: JSON.stringify(platformConfig),
        llm_provider_id: values.llm_provider_id,
        llm_model: values.llm_model,
        llm_temperature: values.llm_temperature,
        llm_max_tokens: values.llm_max_tokens,
        max_session_tokens: values.max_session_tokens,
      }

      const res = await createBot(payload)
      message.success('机器人已创建')
      navigate(`/bots/${res.data.id}/edit`)
    } catch (err: any) {
      if (err?.response?.data?.error) message.error(err.response.data.error)
    } finally {
      setSubmitting(false)
    }
  }

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '100px auto' }} />

  return (
    <div>
      <Button type="link" icon={<ArrowLeftOutlined />} onClick={() => navigate('/bots')} style={{ padding: 0, marginBottom: 16 }}>
        返回机器人列表
      </Button>
      <Title level={3} style={{ marginTop: 0 }}>新增机器人</Title>
      <Card style={{ maxWidth: 720 }}>
        <Form form={form} layout="vertical">
          <Form.Item name="id" label="ID" rules={[{ required: true, message: '请输入唯一标识' }]}>
            <Input placeholder="my-bot" />
          </Form.Item>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="我的机器人" />
          </Form.Item>
          <Form.Item name="platform_type" label="平台类型" initialValue="wecom">
            <Select onChange={(v) => setPlatformType(v)}>
              {platformOptions.map(o => (
                <Select.Option key={o.value} value={o.value}>{o.label}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          {platformType === 'wecom' ? (
            <>
              <Form.Item name="wecom_bot_id" label="Bot ID" rules={[{ required: true, message: '请输入 Bot ID' }]}>
                <Input placeholder="企业微信智能机器人 Bot ID" />
              </Form.Item>
              <Form.Item name="wecom_secret" label="Secret" rules={[{ required: true, message: '请输入 Secret' }]}>
                <Input.Password placeholder="企业微信智能机器人 Secret" />
              </Form.Item>
            </>
          ) : (
            <>
              <Form.Item name="dingtalk_client_id" label="Client ID (AppKey)" rules={[{ required: true, message: '请输入 Client ID' }]}>
                <Input placeholder="钉钉应用 Client ID" />
              </Form.Item>
              <Form.Item name="dingtalk_client_secret" label="Client Secret (AppSecret)" rules={[{ required: true, message: '请输入 Client Secret' }]}>
                <Input.Password placeholder="钉钉应用 Client Secret" />
              </Form.Item>
            </>
          )}
          <Form.Item name="llm_provider_id" label="LLM Provider" rules={[{ required: true, message: '请选择 Provider' }]}>
            <Select placeholder="选择 LLM Provider" onChange={handleProviderChange}>
              {providers.map(p => (
                <Select.Option key={p.id} value={p.id}>{p.name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          <Form.Item name="llm_model" label="模型">
            <AutoComplete options={modelOptions} placeholder="gpt-4o" filterOption={(inputValue, option) =>
              option?.value.toUpperCase().indexOf(inputValue.toUpperCase()) !== -1
            } />
          </Form.Item>
          <Form.Item name="llm_temperature" label="Temperature">
            <InputNumber step={0.1} min={0} max={2} style={{ width: '100%' }} placeholder="0.7" />
          </Form.Item>
          <Form.Item name="llm_max_tokens" label="Max Tokens">
            <InputNumber min={1} style={{ width: '100%' }} placeholder="2048" />
          </Form.Item>
          <Form.Item name="max_session_tokens" label="Max Session Tokens">
            <InputNumber min={1} style={{ width: '100%' }} placeholder="4096" />
          </Form.Item>
          <Form.Item>
            <Button type="primary" loading={submitting} onClick={handleSubmit}>
              保存
            </Button>
          </Form.Item>
        </Form>
      </Card>
    </div>
  )
}
