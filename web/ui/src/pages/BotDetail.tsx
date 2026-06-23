import { useEffect, useState } from 'react'
import { useParams } from 'react-router-dom'
import { Tabs, Spin, Alert, Typography, Table, Button, Modal, Form, Input, Switch, message, Descriptions, Tag, Popconfirm, Space } from 'antd'
import { getBotSessions, getBotSkills, getSession, createSkill, updateSkill, deleteSkill, expireSession, deleteSession, Session, Message, Skill } from '../api'
import ToolPanel from '../components/ToolPanel'
import { PlusOutlined, EditOutlined, DeleteOutlined, ToolOutlined } from '@ant-design/icons'

const { Title } = Typography

export default function BotDetail() {
  const { id } = useParams<{ id: string }>()
  const [sessions, setSessions] = useState<Session[]>([])
  const [skills, setSkills] = useState<Skill[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [skillModalOpen, setSkillModalOpen] = useState(false)
  const [editingSkill, setEditingSkill] = useState<Skill | null>(null)
  const [toolSkill, setToolSkill] = useState<Skill | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [skillForm] = Form.useForm()

  const load = () => {
    if (!id) return
    setLoading(true)
    setError(null)
    Promise.all([
      getBotSessions(id),
      getBotSkills(id),
    ])
      .then(([sessionsRes, skillsRes]) => {
        setSessions(sessionsRes.data.sessions)
        setSkills(skillsRes.data)
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [id])

  const openCreateSkill = () => {
    setEditingSkill(null)
    skillForm.resetFields()
    setSkillModalOpen(true)
  }

  const openEditSkill = (s: Skill) => {
    setEditingSkill(s)
    skillForm.setFieldsValue(s)
    setSkillModalOpen(true)
  }

  const handleSkillOk = async () => {
    if (!id) return
    try {
      const values = await skillForm.validateFields()
      setSubmitting(true)
      if (editingSkill) {
        await updateSkill(id, editingSkill.id, values)
        message.success('技能已更新')
      } else {
        await createSkill(id, values)
        message.success('技能已创建')
      }
      setSkillModalOpen(false)
      load()
    } catch (err: any) {
      if (err?.response?.data?.error) message.error(err.response.data.error)
    } finally {
      setSubmitting(false)
    }
  }

  const handleDeleteSkill = async (skillId: number) => {
    if (!id) return
    try {
      await deleteSkill(id, skillId)
      message.success('技能已删除')
      load()
    } catch (err: any) {
      message.error(err?.response?.data?.error || '删除失败')
    }
  }

  const showSessionDetail = async (key: string) => {
    try {
      const res = await getSession(key)
      const { session, messages } = res.data
      Modal.info({
        title: `会话: ${key}`,
        width: 700,
        content: (
          <div>
            <Descriptions column={1} size="small" bordered>
              <Descriptions.Item label="用户">{session.user_name || session.user_id}</Descriptions.Item>
              <Descriptions.Item label="类型">{session.conversation_type}</Descriptions.Item>
              <Descriptions.Item label="创建时间">{new Date(session.created_at).toLocaleString()}</Descriptions.Item>
            </Descriptions>
            <Title level={5} style={{ marginTop: 16 }}>消息列表</Title>
            {messages.map(msg => (
              <div key={msg.id} style={{
                padding: '8px 12px',
                marginBottom: 8,
                background: msg.role === 'user' ? '#e6f7ff' : msg.expired ? '#f5f5f5' : '#f6ffed',
                borderRadius: 6,
                opacity: msg.expired ? 0.6 : 1,
              }}>
                <Tag color={msg.role === 'user' ? 'blue' : msg.role === 'system' ? 'purple' : 'green'}>
                  {msg.role}
                </Tag>
                <span>{msg.content}</span>
                {msg.expired && <Tag color="default" style={{ marginLeft: 8 }}>已过期</Tag>}
              </div>
            ))}
          </div>
        ),
      })
    } catch {
      message.error('加载会话详情失败')
    }
  }

  const handleExpire = async (key: string) => {
    try {
      await expireSession(key)
      message.success('已标记过期')
    } catch {
      message.error('操作失败')
    }
  }

  const handleDeleteSession = (key: string) => {
    Modal.confirm({
      title: '确认删除',
      content: '将永久删除该会话及所有消息，不可恢复。',
      onOk: async () => {
        try {
          await deleteSession(key)
          message.success('已删除')
          setSessions(prev => prev.filter(s => s.session_key !== key))
        } catch {
          message.error('删除失败')
        }
      },
    })
  }

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '100px auto' }} />
  if (error) return <Alert type="error" message="加载失败" description={error} showIcon action={<Button onClick={load}>重试</Button>} />

  const sessionColumns = [
    { title: '会话 Key', dataIndex: 'session_key', key: 'session_key' },
    { title: '用户', dataIndex: 'user_name', key: 'user_name', render: (v: string, r: Session) => v || r.user_id },
    { title: '类型', dataIndex: 'conversation_type', key: 'conversation_type' },
    {
      title: '更新时间', dataIndex: 'updated_at', key: 'updated_at',
      render: (v: string) => new Date(v).toLocaleString(),
    },
    {
      title: '操作', key: 'action',
      render: (_: unknown, record: Session) => (
        <Space>
          <Button type="link" onClick={() => showSessionDetail(record.session_key)}>详情</Button>
          <Button type="link" onClick={() => handleExpire(record.session_key)}>过期</Button>
          <Button type="link" danger onClick={() => handleDeleteSession(record.session_key)}>删除</Button>
        </Space>
      ),
    },
  ]

  const skillColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description' },
    {
      title: '启用', dataIndex: 'enabled', key: 'enabled',
      render: (v: boolean) => (v ? '是' : '否'),
    },
    {
      title: 'System Prompt', dataIndex: 'system_prompt', key: 'system_prompt',
      ellipsis: true,
    },
    {
      title: '操作', key: 'action',
      render: (_: unknown, record: Skill) => (
        <Space>
          <Button type="link" icon={<ToolOutlined />} onClick={() => setToolSkill(record)}>Tool</Button>
          <Button type="link" icon={<EditOutlined />} onClick={() => openEditSkill(record)}>编辑</Button>
          <Popconfirm title="确认删除此技能？" onConfirm={() => handleDeleteSkill(record.id)}>
            <Button type="link" danger icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <Title level={3}>机器人详情: {id}</Title>
      <Tabs defaultActiveKey="sessions" items={[
        {
          key: 'sessions',
          label: `会话 (${sessions.length})`,
          children: <Table dataSource={sessions} columns={sessionColumns} rowKey="session_key" pagination={false} />,
        },
        {
          key: 'skills',
          label: `技能 (${skills.length})`,
          children: (
            <div>
              <div style={{ marginBottom: 16, textAlign: 'right' }}>
                <Button type="primary" icon={<PlusOutlined />} onClick={openCreateSkill}>新增技能</Button>
              </div>
              <Table dataSource={skills} columns={skillColumns} rowKey="id" pagination={false} />
            </div>
          ),
        },
      ]} />
      <Modal
        title={editingSkill ? '编辑技能' : '新增技能'}
        open={skillModalOpen}
        onOk={handleSkillOk}
        onCancel={() => setSkillModalOpen(false)}
        confirmLoading={submitting}
        width={640}
      >
        <Form form={skillForm} layout="vertical">
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="技能名称" />
          </Form.Item>
          <Form.Item name="description" label="描述" rules={[{ required: true, message: '请输入描述' }]}>
            <Input placeholder="技能描述" />
          </Form.Item>
          <Form.Item name="system_prompt" label="System Prompt" rules={[{ required: true, message: '请输入 System Prompt' }]}>
            <Input.TextArea rows={4} placeholder="你是一个搜索助手..." />
          </Form.Item>
          {editingSkill && (
            <Form.Item name="enabled" label="启用" valuePropName="checked">
              <Switch />
            </Form.Item>
          )}
        </Form>
      </Modal>

      {toolSkill && id && (
        <Modal
          title={`Tool 管理 - ${toolSkill.name}`}
          open={!!toolSkill}
          onCancel={() => setToolSkill(null)}
          footer={null}
          width={800}
        >
          <ToolPanel botId={id} skillId={toolSkill.id} />
        </Modal>
      )}
    </div>
  )
}
