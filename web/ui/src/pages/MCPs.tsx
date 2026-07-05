import { useEffect, useState } from 'react'
import { Card, Spin, Empty, Alert, Typography, Tag, Row, Col, Button, Modal, Form, Input, Switch, message, Popconfirm, Radio, Select } from 'antd'
import { CheckCircleOutlined, StopOutlined, PlusOutlined, EditOutlined, DeleteOutlined, LinkOutlined, CodeOutlined } from '@ant-design/icons'
import { getMCPs, createMCP, updateMCP, deleteMCP, MCP } from '../api'

const { Title, Text } = Typography
const { TextArea } = Input

const COMMAND_TEMPLATES = [
  { label: '文件系统 MCP', command: 'npx', args: '-y @modelcontextprotocol/server-filesystem /path' },
  { label: 'GitHub MCP', command: 'npx', args: '-y @modelcontextprotocol/server-github' },
  { label: 'SQLite MCP', command: 'python', args: '-m mcp_server_sqlite --db-path /path/to/db' },
  { label: 'Docker MCP', command: 'docker', args: 'run -i --rm mcp/server-mcp' },
]

export default function MCPs() {
  const [mcps, setMCPs] = useState<MCP[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<MCP | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [mcpType, setMCPType] = useState<string>('url')
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
    setMCPType('url')
    form.resetFields()
    setModalOpen(true)
  }

  const openEdit = (m: MCP) => {
    setEditing(m)
    setMCPType(m.type || 'url')
    const argsArr = m.args ? JSON.parse(m.args) : []
    form.setFieldsValue({ ...m, args: Array.isArray(argsArr) ? argsArr.join(' ') : '' })
    setModalOpen(true)
  }

  const handleOk = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      const payload: any = { ...values, type: mcpType }
      if (mcpType === 'url') {
        delete payload.command
        delete payload.args
      } else {
        delete payload.endpoint
        if (typeof payload.args === 'string') {
          payload.args = payload.args.trim().split(/\s+/).filter(Boolean)
        }
      }
      let res: any
      if (editing) {
        res = await updateMCP(editing.id, payload)
        message.success('已更新')
      } else {
        res = await createMCP(payload)
        message.success('已创建')
      }
      if (res?.data?.warning) {
        message.warning(res.data.warning)
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

  const fillTemplate = (t: typeof COMMAND_TEMPLATES[0]) => {
    form.setFieldsValue({ command: t.command, args: t.args })
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
            const isCommand = m.type === 'command' || m.command !== ''
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
                  <p>
                    <Tag icon={isCommand ? <CodeOutlined /> : <LinkOutlined />} color={isCommand ? 'blue' : 'cyan'}>
                      {isCommand ? '本地命令' : 'URL'}
                    </Tag>
                  </p>
                  <p><strong>{isCommand ? '命令:' : 'Endpoint:'}</strong> {isCommand ? `${m.command} ${m.args ? JSON.parse(m.args).join(' ') : ''}` : m.endpoint}</p>
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
        width={520}
      >
        <Form form={form} layout="vertical">
          {!editing && (
            <Form.Item name="id" label="ID" rules={[{ required: true, message: '请输入 ID' }]}>
              <Input placeholder="唯一标识" />
            </Form.Item>
          )}
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="例如: my-mcp" />
          </Form.Item>
          <Form.Item label="连接类型">
            <Radio.Group value={mcpType} onChange={e => setMCPType(e.target.value)}>
              <Radio.Button value="url"><LinkOutlined /> URL</Radio.Button>
              <Radio.Button value="command"><CodeOutlined /> 本地命令</Radio.Button>
            </Radio.Group>
          </Form.Item>
          {mcpType === 'url' ? (
            <Form.Item name="endpoint" label="Endpoint" rules={[{ required: true, message: '请输入 Endpoint' }]}>
              <Input placeholder="http://localhost:9090" />
            </Form.Item>
          ) : (
            <>
              <Form.Item name="command" label="Command" rules={[{ required: true, message: '请输入命令' }]}>
                <Input placeholder="例如: npx" />
              </Form.Item>
              <Form.Item name="args" label="Args">
                <Input placeholder="例如: -y @modelcontextprotocol/server-filesystem /path" />
              </Form.Item>
              <Form.Item label="常用模板">
                <Select placeholder="点击选择命令模板" onChange={(val: string) => {
                  const t = COMMAND_TEMPLATES.find(t => t.label === val)
                  if (t) fillTemplate(t)
                }}>
                  {COMMAND_TEMPLATES.map(t => (
                    <Select.Option key={t.label} value={t.label}>{t.label}</Select.Option>
                  ))}
                </Select>
              </Form.Item>
            </>
          )}
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
