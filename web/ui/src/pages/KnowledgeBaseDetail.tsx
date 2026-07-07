import { useState, useEffect, useRef, useCallback } from 'react'
import { useParams, useNavigate } from 'react-router-dom'
import { Card, Table, Button, Upload, Tag, Progress, message, Descriptions, Space } from 'antd'
import { UploadOutlined, ArrowLeftOutlined } from '@ant-design/icons'
import { getKnowledgeBase, uploadFile, getImportTasks, KnowledgeBase, ImportTask } from '../api'

const WS_BASE = window.location.origin.replace(/^http/, 'ws') + '/ws/tasks'

export default function KnowledgeBaseDetail() {
  const { id } = useParams<{ id: string }>()
  const navigate = useNavigate()
  const [kb, setKB] = useState<KnowledgeBase | null>(null)
  const [tasks, setTasks] = useState<ImportTask[]>([])
  const [loading, setLoading] = useState(false)
  const [uploading, setUploading] = useState(false)
  const wsRef = useRef<WebSocket | null>(null)

  const loadKB = async () => {
    if (!id) return
    try {
      const res = await getKnowledgeBase(id)
      setKB(res.data)
    } catch { message.error('加载知识库失败') }
  }

  const loadTasks = async () => {
    if (!id) return
    setLoading(true)
    try {
      const res = await getImportTasks(id)
      setTasks(res.data)
    } finally { setLoading(false) }
  }

  useEffect(() => {
    loadKB()
    loadTasks()
  }, [id])

  useEffect(() => {
    wsRef.current = new WebSocket(WS_BASE)
    wsRef.current.onmessage = (event) => {
      try {
        const data = JSON.parse(event.data)
        setTasks(prev => prev.map(t =>
          t.id === data.task_id
            ? { ...t, status: data.status, total_chunks: data.total_chunks, processed_chunks: data.processed_chunks }
            : t
        ))
      } catch { /* ignore */ }
    }
    return () => { wsRef.current?.close() }
  }, [])

  const handleUpload = async (file: File) => {
    setUploading(true)
    try {
      const res = await uploadFile(id!, file)
      message.success('上传成功')
      loadTasks()
    } catch (err: any) {
      message.error(err.response?.data?.error || '上传失败')
    } finally {
      setUploading(false)
    }
  }

  const statusColors: Record<string, string> = {
    pending: 'default',
    parsing: 'processing',
    chunking: 'processing',
    indexing: 'processing',
    completed: 'success',
    failed: 'error',
  }

  const statusLabels: Record<string, string> = {
    pending: '等待中',
    parsing: '解析中',
    chunking: '分片中',
    indexing: '索引中',
    completed: '已完成',
    failed: '失败',
  }

  const taskColumns = [
    { title: '文件名', dataIndex: 'file_name', key: 'file_name' },
    { title: '大小', dataIndex: 'file_size', key: 'file_size', width: 100,
      render: (v: number) => v ? `${(v / 1024).toFixed(1)} KB` : '-',
    },
    {
      title: '状态', dataIndex: 'status', key: 'status', width: 120,
      render: (s: string, record: ImportTask) => (
        <Space>
          <Tag color={statusColors[s] || 'default'}>{statusLabels[s] || s}</Tag>
          {(s === 'indexing' || s === 'chunking' || s === 'parsing') && record.total_chunks > 0 && (
            <Progress type="circle" size={20} percent={Math.round(record.processed_chunks / record.total_chunks * 100)} />
          )}
        </Space>
      ),
    },
    { title: '进度', dataIndex: 'processed_chunks', key: 'processed_chunks', width: 120,
      render: (_: any, record: ImportTask) => record.total_chunks > 0
        ? `${record.processed_chunks}/${record.total_chunks}`
        : '-',
    },
    { title: '错误', dataIndex: 'error', key: 'error', ellipsis: true },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180,
      render: (v: string) => new Date(v).toLocaleString(),
    },
  ]

  return (
    <div>
      <Button type="link" icon={<ArrowLeftOutlined />} onClick={() => navigate('/knowledge-bases')} style={{ padding: 0, marginBottom: 16 }}>
        返回知识库列表
      </Button>

      {kb && (
        <Card title={`知识库: ${kb.name}`} style={{ marginBottom: 16 }}>
          <Descriptions column={2}>
            <Descriptions.Item label="ID">{kb.id}</Descriptions.Item>
            <Descriptions.Item label="文件数">{kb.file_count}</Descriptions.Item>
            <Descriptions.Item label="Embedding Provider">{kb.embedding_provider_id}</Descriptions.Item>
            <Descriptions.Item label="Embedding Model">{kb.embedding_model}</Descriptions.Item>
            <Descriptions.Item label="描述">{kb.description || '-'}</Descriptions.Item>
            <Descriptions.Item label="创建时间">{new Date(kb.created_at).toLocaleString()}</Descriptions.Item>
          </Descriptions>
        </Card>
      )}

      <Card title="上传文件" style={{ marginBottom: 16 }}>
        <Upload
          accept=".md,.pdf,.docx,.txt,.csv"
          showUploadList={false}
          beforeUpload={(file) => { handleUpload(file); return false }}
          disabled={uploading}
        >
          <Button icon={<UploadOutlined />} loading={uploading}>选择文件上传</Button>
          <span style={{ marginLeft: 12, color: '#999' }}>支持 .md .pdf .docx .txt .csv</span>
        </Upload>
      </Card>

      <Card title="导入任务">
        <Table dataSource={tasks} columns={taskColumns} rowKey="id" loading={loading} size="small" />
      </Card>
    </div>
  )
}
