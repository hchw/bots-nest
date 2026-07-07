import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Table, Button, Modal, Input, Select, Space, message, Popconfirm } from 'antd'
import { PlusOutlined, DatabaseOutlined } from '@ant-design/icons'
import { getKnowledgeBases, createKnowledgeBase, deleteKnowledgeBase, getLLMProviders, getProviderModels, KnowledgeBase, LLMProvider } from '../api'

export default function KnowledgeBases() {
  const [kbs, setKBs] = useState<KnowledgeBase[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [newKB, setNewKB] = useState({ id: '', name: '', description: '', embedding_provider_id: '', embedding_model: '' })
  const [providers, setProviders] = useState<LLMProvider[]>([])
  const [models, setModels] = useState<string[]>([])
  const navigate = useNavigate()

  const load = async () => {
    setLoading(true)
    try {
      const res = await getKnowledgeBases()
      setKBs(res.data)
    } finally {
      setLoading(false)
    }
  }

  useEffect(() => { load() }, [])

  const openCreateModal = async () => {
    const res = await getLLMProviders()
    setProviders(res.data)
    setModels([])
    setNewKB({ id: '', name: '', description: '', embedding_provider_id: '', embedding_model: '' })
    setModalOpen(true)
  }

  const handleProviderChange = async (providerId: string) => {
    setNewKB({ ...newKB, embedding_provider_id: providerId, embedding_model: '' })
    if (providerId) {
      try {
        const res = await getProviderModels(providerId)
        setModels(res.data.models || [])
      } catch {
        setModels([])
      }
    } else {
      setModels([])
    }
  }

  const handleCreate = async () => {
    if (!newKB.id || !newKB.name) {
      message.error('请填写 ID 和名称')
      return
    }
    if (!newKB.embedding_provider_id || !newKB.embedding_model) {
      message.error('请选择 Embedding Provider 和模型')
      return
    }
    try {
      await createKnowledgeBase(newKB)
      message.success('创建成功')
      setModalOpen(false)
      load()
    } catch (err: any) {
      message.error(err.response?.data?.error || '创建失败')
    }
  }

  const handleDelete = async (id: string) => {
    try {
      await deleteKnowledgeBase(id)
      message.success('已删除')
      load()
    } catch (err: any) {
      message.error(err.response?.data?.error || '删除失败')
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id' },
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: 'Embedding Provider', dataIndex: 'embedding_provider_id', key: 'embedding_provider_id', width: 140 },
    { title: 'Embedding Model', dataIndex: 'embedding_model', key: 'embedding_model', width: 200 },
    { title: '文件数', dataIndex: 'file_count', key: 'file_count', width: 80 },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180,
      render: (v: string) => new Date(v).toLocaleString(),
    },
    {
      title: '操作', key: 'action', width: 200,
      render: (_: any, record: KnowledgeBase) => (
        <Space>
          <Button type="link" onClick={() => navigate(`/knowledge-bases/${record.id}`)}>详情</Button>
          <Popconfirm title="确定删除?" onConfirm={() => handleDelete(record.id)}>
            <Button type="link" danger>删除</Button>
          </Popconfirm>
        </Space>
      ),
    },
  ]

  return (
    <div>
      <div style={{ display: 'flex', justifyContent: 'space-between', marginBottom: 16 }}>
        <h2><DatabaseOutlined /> 知识库</h2>
        <Button type="primary" icon={<PlusOutlined />} onClick={openCreateModal}>新建知识库</Button>
      </div>

      <Table dataSource={kbs} columns={columns} rowKey="id" loading={loading} />

      <Modal title="新建知识库" open={modalOpen} onOk={handleCreate} onCancel={() => setModalOpen(false)}>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Input placeholder="ID (唯一标识)" value={newKB.id} onChange={e => setNewKB({ ...newKB, id: e.target.value })} />
          <Input placeholder="名称" value={newKB.name} onChange={e => setNewKB({ ...newKB, name: e.target.value })} />
          <Input.TextArea placeholder="描述" value={newKB.description} onChange={e => setNewKB({ ...newKB, description: e.target.value })} />
          <Select placeholder="选择 Embedding Provider" value={newKB.embedding_provider_id || undefined} onChange={handleProviderChange} style={{ width: '100%' }}>
            {providers.map(p => <Select.Option key={p.id} value={p.id}>{p.name} ({p.id})</Select.Option>)}
          </Select>
          <Select placeholder="选择 Embedding 模型" value={newKB.embedding_model || undefined} onChange={v => setNewKB({ ...newKB, embedding_model: v })} style={{ width: '100%' }}>
            {models.map(m => <Select.Option key={m} value={m}>{m}</Select.Option>)}
          </Select>
        </Space>
      </Modal>
    </div>
  )
}
