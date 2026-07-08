import { useState, useEffect } from 'react'
import { useNavigate } from 'react-router-dom'
import { Table, Button, Modal, Input, Select, Radio, Space, message, Popconfirm, Typography } from 'antd'
import { PlusOutlined, DatabaseOutlined } from '@ant-design/icons'
import { getKnowledgeBases, createKnowledgeBase, updateKnowledgeBase, deleteKnowledgeBase, getLLMProviders, getProviderModels, KnowledgeBase, LLMProvider } from '../api'

export default function KnowledgeBases() {
  const [kbs, setKBs] = useState<KnowledgeBase[]>([])
  const [loading, setLoading] = useState(false)
  const [modalOpen, setModalOpen] = useState(false)
  const [newKB, setNewKB] = useState({ id: '', name: '', description: '', embedding_mode: 'provider', embedding_provider_id: '', embedding_model: '' })
  const [providers, setProviders] = useState<LLMProvider[]>([])
  const [models, setModels] = useState<string[]>([])
  const [editModalOpen, setEditModalOpen] = useState(false)
  const [editingKB, setEditingKB] = useState<KnowledgeBase | null>(null)
  const [editForm, setEditForm] = useState({ name: '', description: '', embedding_mode: 'provider', embedding_provider_id: '', embedding_model: '' })
  const [editModels, setEditModels] = useState<string[]>([])
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
    setNewKB({ id: '', name: '', description: '', embedding_mode: 'provider', embedding_provider_id: '', embedding_model: '' })
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
    if (newKB.embedding_mode === 'provider' && (!newKB.embedding_provider_id || !newKB.embedding_model)) {
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

  const openEditModal = async (kb: KnowledgeBase) => {
    const res = await getLLMProviders()
    setProviders(res.data)
    setEditingKB(kb)
    setEditForm({
      name: kb.name,
      description: kb.description,
      embedding_mode: kb.embedding_mode,
      embedding_provider_id: kb.embedding_provider_id,
      embedding_model: kb.embedding_model,
    })
    if (kb.embedding_provider_id) {
      try {
        const mres = await getProviderModels(kb.embedding_provider_id)
        setEditModels(mres.data.models || [])
      } catch {
        setEditModels([])
      }
    } else {
      setEditModels([])
    }
    setEditModalOpen(true)
  }

  const handleEditProviderChange = async (providerId: string) => {
    setEditForm({ ...editForm, embedding_provider_id: providerId, embedding_model: '' })
    if (providerId) {
      try {
        const res = await getProviderModels(providerId)
        setEditModels(res.data.models || [])
      } catch {
        setEditModels([])
      }
    } else {
      setEditModels([])
    }
  }

  const handleEdit = async () => {
    if (!editingKB) return
    if (!editForm.name) {
      message.error('名称不能为空')
      return
    }
    if (editForm.embedding_mode === 'provider' && (!editForm.embedding_provider_id || !editForm.embedding_model)) {
      message.error('请选择 Embedding Provider 和模型')
      return
    }
    try {
      await updateKnowledgeBase(editingKB.id, editForm)
      message.success('更新成功')
      setEditModalOpen(false)
      setEditingKB(null)
      load()
    } catch (err: any) {
      message.error(err.response?.data?.error || '更新失败')
    }
  }

  const columns = [
    { title: 'ID', dataIndex: 'id', key: 'id' },
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '描述', dataIndex: 'description', key: 'description', ellipsis: true },
    { title: '自动描述', dataIndex: 'auto_summary', key: 'auto_summary', ellipsis: true,
      render: (v: string) => v || <Typography.Text type="secondary">-</Typography.Text>,
    },
    { title: 'Embedding 方式', dataIndex: 'embedding_mode', key: 'embedding_mode', width: 120,
      render: (v: string) => v === 'builtin' ? '内置模型' : 'LLM Provider',
    },
    { title: 'Embedding Provider', dataIndex: 'embedding_provider_id', key: 'embedding_provider_id', width: 140 },
    { title: 'Embedding Model', dataIndex: 'embedding_model', key: 'embedding_model', width: 200 },
    { title: '文件数', dataIndex: 'file_count', key: 'file_count', width: 80 },
    { title: '创建时间', dataIndex: 'created_at', key: 'created_at', width: 180,
      render: (v: string) => new Date(v).toLocaleString(),
    },
    {
      title: '操作', key: 'action', width: 260,
      render: (_: any, record: KnowledgeBase) => (
        <Space>
          <Button type="link" onClick={() => navigate(`/knowledge-bases/${record.id}`)}>详情</Button>
          <Button type="link" onClick={() => openEditModal(record)}>编辑</Button>
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

      <Modal title="编辑知识库" open={editModalOpen} onOk={handleEdit} onCancel={() => { setEditModalOpen(false); setEditingKB(null) }}>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Input placeholder="名称" value={editForm.name} onChange={e => setEditForm({ ...editForm, name: e.target.value })} />
          <Input.TextArea placeholder="描述" value={editForm.description} onChange={e => setEditForm({ ...editForm, description: e.target.value })} />
          <Typography.Text type="secondary">自动生成描述（绑定机器人后自动生成，不可编辑）：</Typography.Text>
          <Input.TextArea value={editingKB?.auto_summary || ''} readOnly placeholder="暂无自动生成描述" autoSize={{ minRows: 2, maxRows: 6 }} />
          <div>Embedding 方式</div>
          <Radio.Group value={editForm.embedding_mode} onChange={e => setEditForm({ ...editForm, embedding_mode: e.target.value, embedding_provider_id: '', embedding_model: '' })}>
            <Radio value="provider">LLM Provider</Radio>
            <Radio value="builtin">内置模型</Radio>
          </Radio.Group>
          {editForm.embedding_mode === 'provider' && (
            <>
              <Select placeholder="选择 Embedding Provider" value={editForm.embedding_provider_id || undefined} onChange={handleEditProviderChange} style={{ width: '100%' }}>
                {providers.map(p => <Select.Option key={p.id} value={p.id}>{p.name} ({p.id})</Select.Option>)}
              </Select>
              <Select placeholder="选择 Embedding 模型" value={editForm.embedding_model || undefined} onChange={v => setEditForm({ ...editForm, embedding_model: v })} style={{ width: '100%' }}>
                {editModels.map(m => <Select.Option key={m} value={m}>{m}</Select.Option>)}
              </Select>
            </>
          )}
        </Space>
      </Modal>

      <Modal title="新建知识库" open={modalOpen} onOk={handleCreate} onCancel={() => setModalOpen(false)}>
        <Space direction="vertical" style={{ width: '100%' }}>
          <Input placeholder="ID (唯一标识)" value={newKB.id} onChange={e => setNewKB({ ...newKB, id: e.target.value })} />
          <Input placeholder="名称" value={newKB.name} onChange={e => setNewKB({ ...newKB, name: e.target.value })} />
          <Input.TextArea placeholder="描述" value={newKB.description} onChange={e => setNewKB({ ...newKB, description: e.target.value })} />
          <div>Embedding 方式</div>
          <Radio.Group value={newKB.embedding_mode} onChange={e => setNewKB({ ...newKB, embedding_mode: e.target.value, embedding_provider_id: '', embedding_model: '' })}>
            <Radio value="provider">LLM Provider</Radio>
            <Radio value="builtin">内置模型</Radio>
          </Radio.Group>
          {newKB.embedding_mode === 'provider' && (
            <>
              <Select placeholder="选择 Embedding Provider" value={newKB.embedding_provider_id || undefined} onChange={handleProviderChange} style={{ width: '100%' }}>
                {providers.map(p => <Select.Option key={p.id} value={p.id}>{p.name} ({p.id})</Select.Option>)}
              </Select>
              <Select placeholder="选择 Embedding 模型" value={newKB.embedding_model || undefined} onChange={v => setNewKB({ ...newKB, embedding_model: v })} style={{ width: '100%' }}>
                {models.map(m => <Select.Option key={m} value={m}>{m}</Select.Option>)}
              </Select>
            </>
          )}
        </Space>
      </Modal>
    </div>
  )
}
