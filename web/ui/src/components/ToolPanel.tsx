import { useEffect, useState, useRef, useCallback } from 'react'
import { Card, Button, Select, Input, message, Space, Tag, Typography, Modal } from 'antd'
import { PlusOutlined, DeleteOutlined, EditOutlined, ThunderboltOutlined, ExperimentOutlined } from '@ant-design/icons'
import { EditorView, basicSetup } from 'codemirror'
import { EditorState } from '@codemirror/state'
import { javascript } from '@codemirror/lang-javascript'
import { python } from '@codemirror/lang-python'
import { java } from '@codemirror/lang-java'
import { cpp } from '@codemirror/lang-cpp'
import { getSkillTools, createSkillTool, updateSkillTool, deleteSkillTool, polishSkillTool, debugSkillTool, GoJudgeTool, DebugResult } from '../api'

const { TextArea } = Input
const { Text } = Typography

const langOptions = [
  { value: 'c', label: 'C' },
  { value: 'cpp', label: 'C++' },
  { value: 'go', label: 'Go' },
  { value: 'java', label: 'Java' },
  { value: 'python3', label: 'Python 3' },
  { value: 'javascript', label: 'JavaScript' },
]

interface ToolPanelProps {
  botId: string
  skillId: number
}

export default function ToolPanel({ botId, skillId }: ToolPanelProps) {
  const [tools, setTools] = useState<GoJudgeTool[]>([])
  const [editingTool, setEditingTool] = useState<GoJudgeTool | null>(null)
  const [creating, setCreating] = useState(false)

  const load = useCallback(async () => {
    try {
      const res = await getSkillTools(botId, skillId)
      setTools(res.data)
    } catch {
      message.error('加载工具列表失败')
    }
  }, [botId, skillId])

  useEffect(() => { load() }, [load])

  const handleCreate = () => {
    setEditingTool(null)
    setCreating(true)
  }

  const handleEdit = (tool: GoJudgeTool) => {
    setEditingTool({ ...tool })
    setCreating(true)
  }

  const handleDelete = async (toolId: number) => {
    try {
      await deleteSkillTool(botId, skillId, toolId)
      message.success('已删除')
      load()
    } catch {
      message.error('删除失败')
    }
  }

  const handleSave = async () => {
    if (!editingTool) return
    try {
      if (editingTool.id) {
        await updateSkillTool(botId, skillId, editingTool.id, editingTool)
        message.success('已保存')
      } else {
        await createSkillTool(botId, skillId, editingTool)
        message.success('已创建')
      }
      setCreating(false)
      setEditingTool(null)
      load()
    } catch {
      message.error('保存失败')
    }
  }

  const handlePolish = async (tool: GoJudgeTool) => {
    if (!tool.prompt) {
      message.warning('请先输入想法')
      return
    }
    try {
      const res = await polishSkillTool(botId, skillId, tool.id)
      setEditingTool({ ...tool, code: res.data.code })
      message.success('润色完成')
    } catch {
      message.error('润色失败')
    }
  }

  const handleDebug = async (tool: GoJudgeTool) => {
    if (!tool.code) {
      message.warning('请先编写代码')
      return
    }
    try {
      const res = await debugSkillTool(botId, skillId, tool.id)
      const result = res.data
      let content = ''
      if (result.stdout) content += `stdout:\n${result.stdout}\n`
      if (result.stderr) content += `stderr:\n${result.stderr}\n`
      content += `status: ${result.status}`
      if (result.error) content += `\nerror: ${result.error}`

      Modal.info({
        title: `${tool.name} - 执行结果`,
        width: 600,
        content: (
          <div>
            {result.stdout && (
              <div style={{ background: '#f6ffed', padding: 12, borderRadius: 6, marginBottom: 8, whiteSpace: 'pre-wrap', fontFamily: 'monospace' }}>
                <Text type="success">stdout:</Text>
                <div>{result.stdout}</div>
              </div>
            )}
            {result.stderr && (
              <div style={{ background: '#fff2f0', padding: 12, borderRadius: 6, marginBottom: 8, whiteSpace: 'pre-wrap', fontFamily: 'monospace' }}>
                <Text type="danger">stderr:</Text>
                <div>{result.stderr}</div>
              </div>
            )}
            <div style={{ padding: 8, background: '#fafafa', borderRadius: 6 }}>
              <Text>exit code: {result.status}</Text>
            </div>
            {result.error && (
              <div style={{ padding: 8, marginTop: 8, background: '#fff2f0', borderRadius: 6 }}>
                <Text type="danger">error: {result.error}</Text>
              </div>
            )}
          </div>
        ),
      })
    } catch {
      message.error('调试执行失败')
    }
  }

  if (!creating) {
    return (
      <div>
        {tools.length === 0 ? (
          <Text type="secondary">暂无工具，请添加</Text>
        ) : (
          tools.map(t => (
            <Card
              key={t.id}
              size="small"
              style={{ marginBottom: 8 }}
              title={
                <Space>
                  <Tag color="blue">{t.language}</Tag>
                  <span>{t.name}</span>
                </Space>
              }
              extra={
                <Space>
                  <Button type="link" icon={<ExperimentOutlined />} onClick={() => handleDebug(t)}>执行</Button>
                  <Button type="link" icon={<EditOutlined />} onClick={() => handleEdit(t)}>编辑</Button>
                  <Button type="link" danger icon={<DeleteOutlined />} onClick={() => handleDelete(t.id)}>删除</Button>
                </Space>
              }
            >
              <Typography.Paragraph ellipsis={{ rows: 2 }} style={{ margin: 0 }}>{t.prompt || '(无描述)'}</Typography.Paragraph>
            </Card>
          ))
        )}
        <Button type="dashed" icon={<PlusOutlined />} block style={{ marginTop: 8 }} onClick={handleCreate}>
          添加 Tool
        </Button>
      </div>
    )
  }

  return (
    <ToolEditor
      tool={editingTool}
      botId={botId}
      skillId={skillId}
      onSave={handleSave}
      onCancel={() => { setCreating(false); setEditingTool(null) }}
      onPolish={handlePolish}
      onDebug={handleDebug}
      onChange={(t) => setEditingTool(t)}
    />
  )
}

function getLangExtension(lang: string) {
  switch (lang) {
    case 'javascript': return javascript()
    case 'python3': return python()
    case 'java': return java()
    case 'c':
    case 'cpp': return cpp()
    default: return javascript()
  }
}

interface ToolEditorProps {
  tool: GoJudgeTool | null
  botId: string
  skillId: number
  onSave: () => void
  onCancel: () => void
  onPolish: (tool: GoJudgeTool) => void
  onDebug: (tool: GoJudgeTool) => void
  onChange: (tool: GoJudgeTool) => void
}

function ToolEditor({ tool, botId, skillId, onSave, onCancel, onPolish, onDebug, onChange }: ToolEditorProps) {
  const editorRef = useRef<HTMLDivElement>(null)
  const viewRef = useRef<EditorView | null>(null)
  const [debugResult, setDebugResult] = useState<DebugResult | null>(null)
  const [debugLoading, setDebugLoading] = useState(false)
  const [polishLoading, setPolishLoading] = useState(false)
  const [saving, setSaving] = useState(false)

  useEffect(() => {
    if (!editorRef.current || !tool) return
    if (viewRef.current) {
      viewRef.current.destroy()
    }
    const state = EditorState.create({
      doc: tool.code || '',
      extensions: [
        basicSetup,
        getLangExtension(tool.language),
        EditorView.updateListener.of((update) => {
          if (update.docChanged) {
            onChange({ ...tool, code: update.state.doc.toString() })
          }
        }),
      ],
    })
    viewRef.current = new EditorView({ state, parent: editorRef.current })
    return () => {
      if (viewRef.current) {
        viewRef.current.destroy()
        viewRef.current = null
      }
    }
  }, [tool?.language])

  const handlePolish = async () => {
    if (!tool || !tool.id) return
    setPolishLoading(true)
    try {
      const res = await polishSkillTool(botId, skillId, tool.id)
      if (viewRef.current) {
        viewRef.current.dispatch({
          changes: { from: 0, to: viewRef.current.state.doc.length, insert: res.data.code }
        })
      }
      onChange({ ...tool, code: res.data.code })
      message.success('润色完成')
    } catch {
      message.error('润色失败')
    } finally {
      setPolishLoading(false)
    }
  }

  const handleDebug = async () => {
    if (!tool || !tool.id) return
    setDebugLoading(true)
    setDebugResult(null)
    try {
      const res = await debugSkillTool(botId, skillId, tool.id)
      setDebugResult(res.data)
    } catch {
      message.error('调试执行失败')
    } finally {
      setDebugLoading(false)
    }
  }

  const handleSave = async () => {
    setSaving(true)
    try {
      if (tool!.id) {
        await updateSkillTool(botId, skillId, tool!.id, tool!)
      } else {
        await createSkillTool(botId, skillId, tool!)
      }
      message.success('已保存')
      onSave()
    } catch {
      message.error('保存失败')
    } finally {
      setSaving(false)
    }
  }

  if (!tool) return null

  return (
    <div>
      <Space direction="vertical" style={{ width: '100%' }}>
        <Space>
          <Select
            value={tool.language || 'python3'}
            style={{ width: 140 }}
            options={langOptions}
            onChange={(v) => onChange({ ...tool, language: v })}
          />
          <Input
            placeholder="Tool 名称"
            style={{ width: 200 }}
            value={tool.name}
            onChange={(e) => onChange({ ...tool, name: e.target.value })}
          />
        </Space>

        <Space style={{ width: '100%', alignItems: 'flex-start' }}>
          <TextArea
            placeholder="输入想法，点击润色生成代码..."
            rows={2}
            style={{ flex: 1 }}
            value={tool.prompt}
            onChange={(e) => onChange({ ...tool, prompt: e.target.value })}
          />
          <Button
            type="primary"
            icon={<ExperimentOutlined />}
            loading={polishLoading}
            onClick={handlePolish}
            disabled={!tool.id || !tool.prompt}
          >
            润色
          </Button>
        </Space>

        <div ref={editorRef} style={{ border: '1px solid #d9d9d9', borderRadius: 6, overflow: 'hidden' }} />

        <Space>
          <Button
            icon={<ThunderboltOutlined />}
            loading={debugLoading}
            onClick={handleDebug}
            disabled={!tool.id || !tool.code}
          >
            调试执行
          </Button>
        </Space>

        {debugResult && (
          <DebugResultDisplay result={debugResult} />
        )}

        <Space>
          <Button type="primary" loading={saving} onClick={handleSave}>保存</Button>
          <Button onClick={onCancel}>取消</Button>
        </Space>
      </Space>
    </div>
  )
}

function DebugResultDisplay({ result }: { result: DebugResult }) {
  return (
    <div>
      {result.stdout && (
        <div style={{ background: '#f6ffed', padding: 12, borderRadius: 6, marginBottom: 8, whiteSpace: 'pre-wrap', fontFamily: 'monospace' }}>
          <Text type="success">stdout:</Text>
          <div>{result.stdout}</div>
        </div>
      )}
      {result.stderr && (
        <div style={{ background: '#fff2f0', padding: 12, borderRadius: 6, marginBottom: 8, whiteSpace: 'pre-wrap', fontFamily: 'monospace' }}>
          <Text type="danger">stderr:</Text>
          <div>{result.stderr}</div>
        </div>
      )}
      <div style={{ padding: 8, background: '#fafafa', borderRadius: 6 }}>
        <Text>exit code: {result.status}</Text>
      </div>
      {result.error && (
        <div style={{ padding: 8, marginTop: 8, background: '#fff2f0', borderRadius: 6 }}>
          <Text type="danger">error: {result.error}</Text>
        </div>
      )}
    </div>
  )
}
