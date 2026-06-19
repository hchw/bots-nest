import { useEffect, useState } from 'react'
import { Spin, Alert, Empty, Typography, Tag, Button, message, Popconfirm, Row, Col, Card, Space, Tooltip } from 'antd'
import { CheckCircleOutlined, CloseCircleOutlined, PlusOutlined, EditOutlined, DeleteOutlined, PlayCircleOutlined, PauseCircleOutlined, EyeOutlined, LoadingOutlined, ExclamationCircleOutlined } from '@ant-design/icons'
import { useNavigate } from 'react-router-dom'
import { getBots, deleteBot, startBot, stopBot, Bot } from '../api'

const { Title, Text } = Typography

const iconFiles = Array.from({ length: 15 }, (_, i) => `bot (${i + 1}).png`)

export default function Bots() {
  const [bots, setBots] = useState<Bot[]>([])
  const [loading, setLoading] = useState(true)
  const [error, setError] = useState<string | null>(null)
  const navigate = useNavigate()
  const [iconIdx, setIconIdx] = useState<Record<string, number>>({})

  const load = () => {
    setLoading(true)
    setError(null)
    getBots()
      .then(res => setBots(res.data))
      .catch(err => setError(err.message))
      .finally(() => setLoading(false))
  }

  useEffect(load, [])

  useEffect(() => {
    if (bots.length === 0) return
    setIconIdx(prev => {
      const next = { ...prev }
      bots.forEach(b => {
        if (!(b.id in next)) {
          next[b.id] = Math.floor(Math.random() * iconFiles.length)
        }
      })
      return next
    })
  }, [bots])

  useEffect(() => {
    if (Object.keys(iconIdx).length === 0) return
    const timer = setTimeout(() => {
      setIconIdx(prev => {
        const ids = Object.keys(prev)
        if (ids.length === 0) return prev
        const id = ids[Math.floor(Math.random() * ids.length)]
        return { ...prev, [id]: Math.floor(Math.random() * iconFiles.length) }
      })
    }, 15000 + Math.random() * 45000)
    return () => clearTimeout(timer)
  }, [iconIdx])

  const handleDelete = async (id: string) => {
    try {
      await deleteBot(id)
      message.success('已删除')
      load()
    } catch (err: any) {
      message.error(err?.response?.data?.error || '删除失败')
    }
  }

  const handleStart = async (id: string) => {
    try {
      await startBot(id)
      message.success('已启动')
      load()
    } catch (err: any) {
      message.error(err?.response?.data?.error || '启动失败')
    }
  }

  const handleStop = async (id: string) => {
    try {
      await stopBot(id)
      message.success('已停止')
      load()
    } catch (err: any) {
      message.error(err?.response?.data?.error || '停止失败')
    }
  }

  if (loading) return <Spin size="large" style={{ display: 'block', margin: '100px auto' }} />
  if (error) return <Alert type="error" message="加载失败" description={error} showIcon action={<Button onClick={load}>重试</Button>} />

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', alignItems: 'center', marginBottom: 16 }}>
        <Title level={3} style={{ margin: 0 }}>机器人列表</Title>
        <Button type="primary" icon={<PlusOutlined />} onClick={() => navigate('/bots/new')}>新增</Button>
      </div>
      {bots.length === 0 ? <Empty description="暂无机器人配置" /> : (
        <Row gutter={[16, 16]}>
          {bots.map(bot => (
            <Col key={bot.id} xs={24} sm={12} lg={8} xl={6}>
              <Card
                hoverable
                actions={[
                  <EyeOutlined key="sessions" onClick={() => navigate(`/bots/${bot.id}`)} title="会话" />,
                  <EditOutlined key="edit" onClick={() => navigate(`/bots/${bot.id}/edit`)} title="编辑" />,
                  bot.status === 'connected'
                    ? <PauseCircleOutlined key="stop" onClick={() => handleStop(bot.id)} title="停止" />
                    : <PlayCircleOutlined key="start" onClick={() => handleStart(bot.id)} title="启动" />,
                  <Popconfirm key="delete" title="确认删除？将删除所有关联数据" onConfirm={() => handleDelete(bot.id)}>
                    <DeleteOutlined />
                  </Popconfirm>,
                ]}
              >
                <div style={{ display: 'flex', alignItems: 'flex-start', justifyContent: 'space-between' }}>
                  <div style={{ flex: 1, minWidth: 0 }}>
                    <Card.Meta
                      title={
                        <Space>
                          <span
                            style={{ cursor: 'pointer', color: '#1677ff' }}
                            onClick={() => navigate(`/bots/${bot.id}`)}
                          >
                            {bot.name}
                          </span>
                          {(() => {
                            const statusMap: Record<string, { color: string; icon: React.ReactNode; text: string }> = {
                              connected: { color: 'green', icon: <CheckCircleOutlined />, text: '已连接' },
                              connecting: { color: 'blue', icon: <LoadingOutlined />, text: '连接中' },
                              disconnected: { color: 'default', icon: <CloseCircleOutlined />, text: '未连接' },
                              error: { color: 'red', icon: <ExclamationCircleOutlined />, text: '连接异常' },
                            }
                            const s = statusMap[bot.status] || statusMap.disconnected
                            return <Tag color={s.color} icon={s.icon}>{s.text}</Tag>
                          })()}
                        </Space>
                      }
                      description={
                        <div>
                          <Text type="secondary" style={{ display: 'block' }}>Provider: {bot.llm_provider_id}</Text>
                          <Text type="secondary" style={{ display: 'block' }}>模型: {bot.llm_model}</Text>
                          <Tooltip title={bot.status}>
                            <Text type="secondary" style={{ display: 'block' }}>连接状态: {bot.status}</Text>
                          </Tooltip>
                        </div>
                      }
                    />
                  </div>
                  {iconIdx[bot.id] !== undefined && (
                    <img
                      src={`/icon/${iconFiles[iconIdx[bot.id]]}`}
                      alt="表情"
                      style={{
                        width: 48,
                        height: 48,
                        borderRadius: 4,
                        marginLeft: 8,
                        flexShrink: 0,
                        cursor: 'pointer',
                      }}
                      onClick={() => {
                        if (Math.random() < 1 / 3) {
                          setIconIdx(prev => ({ ...prev, [bot.id]: Math.floor(Math.random() * iconFiles.length) }))
                        }
                      }}
                    />
                  )}
                </div>
              </Card>
            </Col>
          ))}
        </Row>
      )}
    </div>
  )
}
