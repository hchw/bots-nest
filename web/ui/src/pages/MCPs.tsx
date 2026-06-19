import { useEffect, useState } from 'react'
import { Card, Spin, Empty, Alert, Typography, Tag, Row, Col, Button, Modal, Form, Input, Switch, message, Popconfirm } from 'antd'
import { CheckCircleOutlined, StopOutlined, PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons'
import { getMCPs, createMCP, updateMCP, deleteMCP, MCP } from '../api'

const { Title } = Typography

export default function MCPs() {
  const [mcps, setMCPs] = useState<MCP[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<MCP | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [form] = Form.useForm()

  const load = () => {
    setLoading(true)
    setError(null)
    getMCPs()
      .then(res => setMCPs(res.data))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    setModalOpen(true)
  }

  const openEdit = (m: MCP) => {
    setEditing(m)
    form.setFieldsValue(m)
    setModalOpen(true)
  }

  const handleOk = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      if (editing) {
        await updateMCP(editing.id, values)
        message.success('已更新')
      } else {
        await createMCP(values)
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
      await deleteMCP(id)
      message.success('已删除')
      load()
    } catch (err: any) {
      message.error(err?.response?.data?.error || '删除失败')
    }
  }

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '100px auto' }} />
  if (error) return <Alert type="error" message="加载失败" description={error} showIcon action={<Button onClick={load}>重试</Button>} />

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>MCPs</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>新增</Button>
      </div>
      {mcps.length === 0 ? <Empty description="暂无 MCP 配置" /> : (
        <Row gutter={[16, 16]}>
          {mcps.map(m => {
            const tools = (() => { try { return JSON.parse(m.tools); } catch { return []; } })()
            return (
              <Col xs={24} sm={12} lg={8} key={m.id}>
                <Card
                  title={m.name}
                  extra={
                    <Tag color={m.enabled ? 'green' : 'red'} icon={m.enabled ? <CheckCircleOutlined /> : <StopOutlined />}>
                      {m.enabled ? '已启用' : '已禁用'}
                    </Tag>
                  }
                  actions={[
                    <EditOutlined key="edit" onClick={() => openEdit(m)} />,
                    <Popconfirm key="delete" title="确认删除？" onConfirm={() => handleDelete(m.id)}>
                      <DeleteOutlined />
                    </Popconfirm>,
                  ]}
                >
                  <p><strong>Endpoint:</strong> {m.endpoint}</p>
                  <p><strong>工具数:</strong> {Array.isArray(tools) ? tools.length : 0}</p>
                </Card>
              </Col>
            )
          })}
        </Row>
      )}
      <Modal
        title={editing ? '编辑 MCP' : '新增 MCP'}
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
            <Input placeholder="例如: example-mcp" />
          </Form.Item>
          <Form.Item name="endpoint" label="Endpoint" rules={[{ required: true, message: '请输入 Endpoint' }]}>
            <Input placeholder="http://localhost:9090" />
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
