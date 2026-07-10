import { useEffect, useState } from 'react'
import { Card, Table, Spin, Empty, Alert, Button, Modal, Form, Input, Select, Switch, message, Tag, Typography, Tabs, Popconfirm, Drawer, Space, Row, Col } from 'antd'
import { PlusOutlined, EditOutlined, DeleteOutlined, CheckCircleOutlined, StopOutlined, ClockCircleOutlined, RobotOutlined, ThunderboltOutlined } from '@ant-design/icons'
import { getTaskPlugins, getGlobalTasks, createGlobalTask, updateGlobalTask, deleteGlobalTask, getExecutionLogs, parseLLMTask, getBots, TaskPlugin, GlobalTask, TaskExecutionLog, Bot } from '../api'

const { Title, Text } = Typography
const { TextArea } = Input

export default function ScheduledTasks() {
  const [plugins, setPlugins] = useState<TaskPlugin[]>([])
  const [tasks, setTasks] = useState<GlobalTask[]>([])
  const [logs, setLogs] = useState<TaskExecutionLog[]>([])
  const [bots, setBots] = useState<Bot[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const [modalOpen, setModalOpen] = useState(false)
  const [editing, setEditing] = useState<GlobalTask | null>(null)
  const [submitting, setSubmitting] = useState(false)
  const [logTaskId, setLogTaskId] = useState<string>('')
  const [logOpen, setLogOpen] = useState(false)
  const [llmInput, setLLMInput] = useState('')
  const [llmLoading, setLLMLoading] = useState(false)
  const [activeTab, setActiveTab] = useState('tasks')
  const [form] = Form.useForm()

  const load = () => {
    setLoading(true)
    setError(null)
    Promise.all([
      getTaskPlugins(),
      getGlobalTasks(),
      getBots(),
    ])
      .then(([pluginsRes, tasksRes, botsRes]) => {
        setPlugins(pluginsRes.data)
        setTasks(tasksRes.data)
        setBots(botsRes.data)
      })
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  const openCreate = () => {
    setEditing(null)
    form.resetFields()
    form.setFieldsValue({ enabled: true, route: 'direct', task_type: 'cron', bot_ids: [] })
    setModalOpen(true)
  }

  const openEdit = (t: GlobalTask) => {
    setEditing(t)
    form.setFieldsValue({
      name: t.name,
      task_type: t.task_type,
      cron_expr: t.cron_expr,
      interval_sec: t.interval_sec,
      route: t.route,
      content: t.content,
      enabled: t.enabled,
      bot_ids: t.bot_ids || [],
    })
    setModalOpen(true)
  }

  const handleOk = async () => {
    try {
      const values = await form.validateFields()
      setSubmitting(true)
      const payload = {
        ...values,
        bot_ids: values.bot_ids || [],
      }
      if (editing) {
        await updateGlobalTask(editing.id, payload)
        message.success('已更新')
      } else {
        await createGlobalTask(payload)
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
      await deleteGlobalTask(id)
      message.success('已删除')
      load()
    } catch (err: any) {
      message.error(err?.response?.data?.error || '删除失败')
    }
  }

  const handleLLMCreate = async () => {
    if (!llmInput.trim()) {
      message.warning('请输入自然语言描述')
      return
    }
    setLLMLoading(true)
    try {
      const res = await parseLLMTask(llmInput)
      if (res.data.parsed) {
        const parsed = res.data.parsed as any
        form.setFieldsValue({
          name: parsed.name || '',
          task_type: parsed.task_type || 'cron',
          cron_expr: parsed.cron_expr || '',
          interval_sec: parsed.interval_sec || 0,
          route: parsed.route || 'direct',
          content: parsed.content || '',
        })
        message.success('AI 解析完成，请确认并提交')
      } else {
        message.warning('AI 解析失败，请手动填写')
      }
    } catch (err: any) {
      message.error(err?.response?.data?.error || 'AI 解析失败')
    } finally {
      setLLMLoading(false)
    }
  }

  const viewLogs = (taskId: string) => {
    setLogTaskId(taskId)
    getExecutionLogs(taskId)
      .then(res => setLogs(res.data))
      .catch(() => message.error('加载日志失败'))
    setLogOpen(true)
  }

  const taskColumns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '类型', dataIndex: 'task_type', key: 'task_type', render: (v: string) => v === 'cron' ? <Tag>Cron</Tag> : v === 'interval' ? <Tag color="blue">间隔</Tag> : <Tag color="purple">一次性</Tag> },
    {
      title: '执行时间', key: 'time',
      render: (_: any, r: GlobalTask) => r.task_type === 'cron' ? r.cron_expr : r.task_type === 'interval' ? `每 ${r.interval_sec} 秒` : '-',
    },
    { title: '路由', dataIndex: 'route', key: 'route', render: (v: string) => <Tag color={v === 'llm' ? 'green' : 'default'}>{v === 'llm' ? 'AI 处理' : '直接推送'}</Tag> },
    {
      title: '状态', dataIndex: 'enabled', key: 'enabled',
      render: (v: boolean) => <Tag icon={v ? <CheckCircleOutlined /> : <StopOutlined />} color={v ? 'green' : 'red'}>{v ? '启用' : '禁用'}</Tag>,
    },
    {
      title: '绑定机器人', key: 'bots',
      render: (_: any, r: GlobalTask) => <span>{r.bot_ids?.length || 0} 个</span>,
    },
    {
      title: '操作', key: 'actions',
      render: (_: any, r: GlobalTask) => (
        <Space>
          <Button type="link" size="small" icon={<EditOutlined />} onClick={() => openEdit(r)}>编辑</Button>
          <Button type="link" size="small" icon={<ClockCircleOutlined />} onClick={() => viewLogs(r.id)}>日志</Button>
          <Popconfirm title="确认删除？" onConfirm={() => handleDelete(r.id)}>
            <Button type="link" danger size="small" icon={<DeleteOutlined />}>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  const logColumns = [
    { title: '执行时间', dataIndex: 'executed_at', key: 'executed_at', render: (v: string) => v ? new Date(v).toLocaleString() : '-' },
    { title: '机器人', dataIndex: 'bot_id', key: 'bot_id' },
    { title: '会话', dataIndex: 'session_key', key: 'session_key' },
    { title: '状态', dataIndex: 'status', key: 'status', render: (v: string) => <Tag color={v === 'success' ? 'green' : v === 'failed' ? 'red' : 'blue'}>{v}</Tag> },
    { title: '结果', dataIndex: 'result', key: 'result', ellipsis: true },
  ]

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '100px auto' }} />
  if (error) return <Alert type="error" message="加载失败" description={error} showIcon action={<Button onClick={load}>重试</Button>} />

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>定时任务</Title>
        <Space>
          <Button type="primary" icon={<PlusOutlined />} onClick={openCreate}>创建任务</Button>
        </Space>
      </div>

      <Tabs activeKey={activeTab} onChange={setActiveTab} items={[
        {
          key: 'tasks',
          label: <span><ClockCircleOutlined /> 全局任务</span>,
          children: (
            <>
              {plugins.length > 0 && (
                <Card size="small" style={{ marginBottom: 16 }}>
                  <Space wrap>
                    <Text strong>已注册组件：</Text>
                    {plugins.map(p => (
                      <Tag key={p.id} icon={<ThunderboltOutlined />} color={p.enabled ? 'green' : 'default'}>{p.name} ({p.type})</Tag>
                    ))}
                  </Space>
                </Card>
              )}
              {tasks.length === 0 ? <Empty description="暂无全局定时任务" /> : (
                <Table dataSource={tasks} columns={taskColumns} rowKey="id" pagination={{ pageSize: 10 }} />
              )}
            </>
          ),
        },
        {
          key: 'plugins',
          label: <span><ThunderboltOutlined /> 插件管理</span>,
          children: plugins.length === 0 ? <Empty description="暂无任务插件" /> : (
            <Table dataSource={plugins} columns={[
              { title: '名称', dataIndex: 'name', key: 'name' },
              { title: '类型', dataIndex: 'type', key: 'type' },
              { title: '描述', dataIndex: 'description', key: 'description' },
              { title: '状态', dataIndex: 'enabled', key: 'enabled', render: (v: boolean) => <Tag color={v ? 'green' : 'red'}>{v ? '已启用' : '已禁用'}</Tag> },
            ]} rowKey="id" pagination={false} />
          ),
        },
      ]} />

      <Modal
        title={editing ? '编辑全局任务' : '创建全局任务'}
        open={modalOpen}
        onOk={handleOk}
        onCancel={() => setModalOpen(false)}
        confirmLoading={submitting}
        width={640}
      >
        <Form form={form} layout="vertical">
          <Form.Item name="name" label="任务名称" rules={[{ required: true, message: '请输入任务名称' }]}>
            <Input placeholder="例如: 每日天气预报推送" />
          </Form.Item>
          <Form.Item name="task_type" label="执行类型" rules={[{ required: true, message: '请选择执行类型' }]}>
            <Select>
              <Select.Option value="cron">Cron 表达式</Select.Option>
              <Select.Option value="interval">间隔执行</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item noStyle shouldUpdate={(prev, cur) => prev.task_type !== cur.task_type}>
            {({ getFieldValue }) => {
              const t = getFieldValue('task_type')
              return t === 'cron' ? (
                <Form.Item name="cron_expr" label="Cron 表达式" rules={[{ required: true, message: '请输入 cron 表达式' }]}>
                  <Input placeholder="例如: 0 8 * * *" />
                </Form.Item>
              ) : t === 'interval' ? (
                <Form.Item name="interval_sec" label="间隔秒数" rules={[{ required: true, message: '请输入间隔秒数' }]}>
                  <Input type="number" placeholder="例如: 3600" />
                </Form.Item>
              ) : null
            }}
          </Form.Item>
          <Form.Item name="route" label="执行路由" rules={[{ required: true, message: '请选择执行路由' }]}>
            <Select>
              <Select.Option value="llm">AI 处理（走 LLM 生成回复后推送）</Select.Option>
              <Select.Option value="direct">直接推送（直接发送消息）</Select.Option>
            </Select>
          </Form.Item>
          <Form.Item name="content" label="执行内容" rules={[{ required: true, message: '请输入执行内容' }]}>
            <TextArea rows={3} placeholder="LLM 提示语或直接消息文本" />
          </Form.Item>
          <Form.Item name="bot_ids" label="绑定机器人">
            <Select mode="multiple" placeholder="选择机器人（不选则仅创建任务，不绑定）">
              {bots.map(b => (
                <Select.Option key={b.id} value={b.id}><RobotOutlined /> {b.name}</Select.Option>
              ))}
            </Select>
          </Form.Item>
          {!editing && (
            <Card size="small" title={<span><ThunderboltOutlined /> AI 创建</span>} style={{ marginBottom: 16 }}>
              <Row gutter={8}>
                <Col flex="auto">
                  <Input.TextArea
                    value={llmInput}
                    onChange={e => setLLMInput(e.target.value)}
                    placeholder="输入自然语言描述，例如：每天早上8点给所有机器人推送天气预报"
                    rows={2}
                  />
                </Col>
                <Col>
                  <Button type="primary" ghost icon={<ThunderboltOutlined />} loading={llmLoading} onClick={handleLLMCreate}>
                    AI 解析
                  </Button>
                </Col>
              </Row>
            </Card>
          )}
          <Form.Item name="enabled" label="启用" valuePropName="checked">
            <Switch />
          </Form.Item>
        </Form>
      </Modal>

      <Drawer
        title="执行日志"
        open={logOpen}
        onClose={() => setLogOpen(false)}
        width={600}
      >
        {logs.length === 0 ? <Empty description="暂无执行日志" /> : (
          <Table dataSource={logs} columns={logColumns} rowKey="id" pagination={{ pageSize: 10 }} size="small" />
        )}
      </Drawer>
    </div>
  )
}
