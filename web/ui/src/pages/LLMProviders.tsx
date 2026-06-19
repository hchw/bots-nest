import { useEffect, useState } from 'react'
import { Card, Spin, Empty, Alert, Typography, Tag, Row, Col, Button, Modal, Form, Input, Switch, message, Popconfirm, Popover, Tooltip } from 'antd'
import { CheckCircleOutlined, StopOutlined, PlusOutlined, EditOutlined, DeleteOutlined, ReloadOutlined, WarningOutlined } from '@ant-design/icons'
import { getLLMProviders, createLLMProvider, updateLLMProvider, deleteLLMProvider, getProviderModels, LLMProvider } from '../api'

const { Title, Text } = Typography

export default function LLMProviders() {
  const [providers, setProviders] = useState<LLMProvider[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<LLMProvider | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [refreshing, setRefreshing] = useState<Record<string, boolean>>({})
  const [modelWarnings, setModelWarnings] = useState<Record<string, boolean>>({})
  const [form] = Form.useForm()

  const load = () => {
    setLoading(true)
    setError(null)
    getLLMProviders()
      .then(res => setProviders(res.data))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setModalOpen(true)
  }

  const openEdit = (p: LLMProvider) => {
    setEditing(p)
    form.setFieldsValue(p)
    setModalOpen(true)
  }

  const handleOk = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      if (editing) {
        await updateLLMProvider(editing.id, values)
        message.success('已更新')
      } else {
        await createLLMProvider(values)
        message.success('已创建')
      }
      setModalOpen(false)
      load()
    } catch (err: any) {
      if (err?.response?.data?.error) message.error(err.response.data.error)
    } finally {
      setSubmitting(false)
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await deleteLLMProvider(id)
      message.success('已删除')
      load()
    } catch (err: any) {
      message.error(err?.response?.data?.error || '删除失败')
    }
  }

  const handleRefresh = async (id: string) => {
    setRefreshing(prev => ({ ...prev, [id]: true }))
    try {
      const res = await getProviderModels(id)
      const models = res.data.models
      setProviders(prev => prev.map(p => p.id === id ? { ...p, models: JSON.stringify(models) } : p))
      setModelWarnings(prev => ({ ...prev, [id]: false }))
      message.success('模型列表已刷新')
    } catch {
      setModelWarnings(prev => ({ ...prev, [id]: true }))
      message.warning('刷新失败，仍展示缓存列表')
    } finally {
      setRefreshing(prev => ({ ...prev, [id]: false }))
    }
  }

  const renderModels = (models: string) => {
    let list: string[]
    try {
      list = JSON.parse(models)
      if (!Array.isArray(list)) list = []
    } catch {
      list = []
    }
    if (list.length === 0) {
      return <Text type="secondary" italic>未获取到模型</Text>
    }
    const display = list.slice(0, 3)
    const rest = list.slice(3)
    return (
      <span>
        {display.join(', ')}
        {rest.length > 0 && (
          <Popover title="全部模型" content={rest.map(m => <div key={m}>{m}</div>)} trigger="hover">
            <Tag color="blue" style={{ cursor: 'pointer', marginLeft: 4 }}>+{rest.length}</Tag>
          </Popover>
        )}
      </span>
    )
  }

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '100px auto' }} />
  if (error) return <Alert type="error" message="加载失败" description={error} showIcon action={<Button onClick={load}>重试</Button>} />

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>LLM Providers</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增</Button>
      </div>
      {providers.length === 0 ? <Empty description="暂无 LLM Provider 配置" /> : (
        <Row gutter={[16, 16]}>
          {providers.map(p => (
            <Col xs={24} sm={12} lg={8} key={p.id}>
              <Card
                title={
                  <span>
                    {p.name}
                    {modelWarnings[p.id] && (
                      <Tooltip title="模型列表刷新失败，展示缓存">
                        <WarningOutlined style={{ color: '#faad14', marginLeft: 8 }} />
                      </Tooltip>
                    )}
                  </span>
                }
                extra={
                  <Tag color={p.enabled ? 'green' : 'red'} icon={p.enabled ? <CheckCircleOutlined /> : <StopOutlined />}>
                    {p.enabled ? '已启用' : '已禁用'}
                  </Tag>
                }
                actions={[
                  <EditOutlined key="edit" onClick={() => openEdit(p)} />,
                  <ReloadOutlined key="refresh" spin={refreshing[p.id]} onClick={() => handleRefresh(p.id)} />,
                  <Popconfirm key="delete" title="确认删除？" onConfirm={() => handleDelete(p.id)}>
                    <DeleteOutlined />
                  </Popconfirm>,
                ]}
              >
                <p><strong>Endpoint:</strong> {p.endpoint}</p>
                <p style={{ marginBottom: 4 }}><strong>模型:</strong></p>
                <p style={{ marginTop: 0 }}>{renderModels(p.models)}</p>
                <p><strong>创建时间:</strong> {new Date(p.created_at).toLocaleString()}</p>
              </Card>
            </Col>
          ))}
        </Row>
      )}
      <Modal
        title={editing ? '编辑 LLM Provider' : '新增 LLM Provider'}
        open={modalOpen}
        onOk={handleOk}
        onCancel={() => setModalOpen(false)}
        confirmLoading={submitting}
      >
        <Form form={form} layout="vertical">
          {!editing && (
            <Form.Item name="id" label="ID" rules={[{ required: true, message: '请输入 ID' }]}>
              <Input placeholder="唯一标识" />
            </Form.Item>
          )}
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="例如: default" />
          </Form.Item>
          <Form.Item name="endpoint" label="Endpoint" rules={[{ required: true, message: '请输入 Endpoint' }]}>
            <Input placeholder="https://api.openai.com/v1" />
          </Form.Item>
          <Form.Item name="api_key" label={editing ? "API Key（留空不修改）" : "API Key"} rules={editing ? [] : [{ required: true, message: '请输入 API Key' }]}>
            <Input.Password placeholder={editing ? '留空则不修改' : 'sk-...'} />
          </Form.Item>
          {editing && (
            <Form.Item name="enabled" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>
    </div>
  )
}
