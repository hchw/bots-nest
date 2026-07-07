import { useEffect, useState } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Card, Form, Input, InputNumber, Select, AutoComplete, Button, message, Spin, Typography, Table, Popconfirm, Space, Tag, Modal, Switch } from 'antd'
import { ArrowLeftOutlined, PlusOutlined, EditOutlined, DeleteOutlined, ToolOutlined } from '@ant-design/icons'
import { getLLMProviders, getProviderModels, getBot, getBotSkills, createSkill, updateSkill, deleteSkill, updateBot, getKnowledgeBases, getBotBindings, updateBotBindings, LLMProvider, Bot, Skill, KnowledgeBase } from '../api'
import ToolPanel from '../components/ToolPanel'

const { Title } = Typography

export default function BotEdit() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [bot, setBot] = useState<Bot | null>(null)
  const [providers, setProviders] = useState<LLMProvider[]>([])
  const [modelOptions, setModelOptions] = useState<{ value: string }[]>([])
  const [skills, setSkills] = useState<Skill[]>([])
  const [loading, setLoading] = useState(true)
  const [saving, setSaving] = useState(false)
  const [skillSubmitting, setSkillSubmitting] = useState(false)
  const [skillModalOpen, setSkillModalOpen] = useState(false)
  const [editingSkill, setEditingSkill] = useState<Skill | null>(null)
  const [toolSkill, setToolSkill] = useState<Skill | null>(null)
  const [skillForm] = Form.useForm()
  const [form] = Form.useForm()
  const [allKBs, setAllKBs] = useState<KnowledgeBase[]>([])
  const [selectedKBIDs, setSelectedKBIDs] = useState<string[]>([])

  const load = () => {
    if (!id) return
    setLoading(true)
    Promise.all([
      getBot(id),
      getLLMProviders(),
      getBotSkills(id),
      getKnowledgeBases(),
      getBotBindings(id),
    ])
      .then(([botRes, provRes, skillsRes, kbRes, bindingRes]) => {
        const botData = botRes.data
        setBot(botData)
        setProviders(provRes.data)
        setSkills(skillsRes.data)
        setAllKBs(kbRes.data)
        setSelectedKBIDs(bindingRes.data.map((b: any) => b.kb_id))
        form.setFieldsValue(botData)
        if (botData.llm_provider_id) {
          handleProviderChange(botData.llm_provider_id)
        }
      })
      .catch(() => message.error('加载失败'))
      .finally(() => setLoading(false))
  }

  useEffect(load, [id])

  const handleProviderChange = (providerId: string) => {
    setModelOptions([])
    if (!providerId) return
    getProviderModels(providerId)
      .then(res => setModelOptions(res.data.models.map(m => ({ value: m }))))
      .catch(() => {})
  }

  const handleSaveBasic = async () => {
    if (!id) return
    try {
      const values = await form.validateFields()
      setSaving(true)
      const payload: Record<string, any> = {}
      const fields = ['name', 'wecom_bot_id', 'wecom_secret',
        'llm_provider_id', 'llm_model', 'llm_temperature', 'llm_max_tokens', 'max_session_tokens', 'enabled']
      for (const key of fields) {
        if (values[key] !== undefined && values[key] !== null && values[key] !== '') {
          payload[key] = values[key]
        }
      }
      await updateBot(id, payload)
      message.success('基础配置已保存')
      navigate('/bots')
    } catch (err: any) {
      if (err?.response?.data?.error) message.error(err.response.data.error)
    } finally {
      setSaving(false)
    }
  }

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
      setSkillSubmitting(true)
      if (editingSkill) {
        await updateSkill(id, editingSkill.id, values)
        message.success('技能已更新')
      } else {
        await createSkill(id, values)
        message.success('技能已创建')
      }
      setSkillModalOpen(false)
      const res = await getBotSkills(id)
      setSkills(res.data)
    } catch (err: any) {
      if (err?.response?.data?.error) message.error(err.response.data.error)
    } finally {
      setSkillSubmitting(false)
    }
  }

  const handleDeleteSkill = async (skillId: number) => {
    if (!id) return
    try {
      await deleteSkill(id, skillId)
      message.success('技能已删除')
      const res = await getBotSkills(id)
      setSkills(res.data)
    } catch (err: any) {
      message.error(err?.response?.data?.error || '删除失败')
    }
  }

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '100px auto' }} />
  if (!bot) return <Typography.Text type="danger">未找到机器人</Typography.Text>

  const skillColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description' },
    {
      title: '启用', dataIndex: 'enabled', key: 'enabled',
      render: (v: boolean) => v ? <Tag color="green">是</Tag> : <Tag>否</Tag>,
    },
    {
      title: 'System Prompt',
      dataIndex: 'system_prompt',
      key: 'system_prompt',
      ellipsis: true,
      width: 300,
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
      <Button type="link" icon={<ArrowLeftOutlined />} onClick={() => navigate('/bots')} style={{ padding: 0, marginBottom: 16 }}>
        返回机器人列表
      </Button>
      <Title level={3} style={{ marginTop: 0 }}>编辑机器人: {bot.name}</Title>

      <Card title="基础配置" style={{ marginBottom: 24 }}>
        <Form form={form} layout="vertical" style={{ maxWidth: 720 }}>
          <Form.Item name="name" label="名称" rules={[{ required: true, message: '请输入名称' }]}>
            <Input placeholder="我的机器人" />
          </Form.Item>
          <Form.Item name="wecom_bot_id" label="Bot ID">
            <Input placeholder="留空则不修改" />
          </Form.Item>
          <Form.Item name="wecom_secret" label="Secret">
            <Input.Password placeholder="留空则不修改" />
          </Form.Item>
          <Form.Item name="llm_provider_id" label="LLM Provider">
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
          <Form.Item name="enabled" label="启用">
            <Select>
              <Select.Option value={true as any}>是</Select.Option>
              <Select.Option value={false as any}>否</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item>
            <Space>
              <Button type="primary" loading={saving} onClick={handleSaveBasic}>保存基础配置</Button>
              <Button onClick={() => navigate('/bots')}>取消</Button>
            </Space>
          </Form.Item>
        </Form>
      </Card>

      <Card
        title="技能配置"
        extra={<Button type="primary" icon={<PlusOutlined />} onClick={openCreateSkill}>添加技能</Button>}
      >
        {skills.length === 0 ? (
          <Typography.Text type="secondary">暂无技能，点击"添加技能"创建</Typography.Text>
        ) : (
          <Table dataSource={skills} columns={skillColumns} rowKey="id" pagination={false} />
        )}
      </Card>

      <Modal
        title={editingSkill ? '编辑技能' : '添加技能'}
        open={skillModalOpen}
        onOk={handleSkillOk}
        onCancel={() => setSkillModalOpen(false)}
        confirmLoading={skillSubmitting}
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

      <Card title="知识库绑定" style={{ marginBottom: 24 }}>
        <Select
          mode="multiple"
          style={{ width: '100%' }}
          placeholder="选择绑定的知识库"
          value={selectedKBIDs}
          onChange={setSelectedKBIDs}
          options={allKBs.map(kb => ({ label: `${kb.name} (${kb.id})`, value: kb.id }))}
        />
        <Button
          type="primary"
          style={{ marginTop: 12 }}
          onClick={async () => {
            try {
              await updateBotBindings(id!, { kb_ids: selectedKBIDs })
              message.success('知识库绑定已更新')
              if (id) {
                const res = await getBotBindings(id)
                setSelectedKBIDs(res.data.map((b: any) => b.kb_id))
              }
            } catch (err: any) {
              message.error(err.response?.data?.error || '更新失败')
            }
          }}
        >
          保存绑定
        </Button>
      </Card>

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
