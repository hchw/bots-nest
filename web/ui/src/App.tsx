import { Routes, Route, Navigate } from 'react-router-dom'
import { Layout, Menu } from 'antd'
import { useNavigate, useLocation } from 'react-router-dom'
import {
  RobotOutlined,
  ApiOutlined,
  ToolOutlined,
  DatabaseOutlined,
  ClockCircleOutlined,
} from '@ant-design/icons'
import LLMProviders from './pages/LLMProviders'
import MCPs from './pages/MCPs'
import Bots from './pages/Bots'
import BotNew from './pages/BotNew'
import BotEdit from './pages/BotEdit'
import BotDetail from './pages/BotDetail'
import KnowledgeBases from './pages/KnowledgeBases'
import KnowledgeBaseDetail from './pages/KnowledgeBaseDetail'
import ScheduledTasks from './pages/ScheduledTasks'

const { Sider, Content } = Layout

const menuItems = [
  {
    key: '/llm-providers',
    icon: <ApiOutlined />,
    label: 'LLM Providers',
  },
  {
    key: '/mcps',
    icon: <ToolOutlined />,
    label: 'MCPs',
  },
  {
    key: '/bots',
    icon: <RobotOutlined />,
    label: '机器人',
  },
  {
    key: '/knowledge-bases',
    icon: <DatabaseOutlined />,
    label: '知识库',
  },
  {
    key: '/scheduled-tasks',
    icon: <ClockCircleOutlined />,
    label: '定时任务',
  },
]

export default function App() {
  const navigate = useNavigate()
  const location = useLocation()

  const selectedKey = '/' + location.pathname.split('/')[1]

  return (
    <Layout style={{ minHeight: '100vh' }}>
      <Sider width={220} theme="dark">
        <div style={{
          height: 64,
          display: 'flex',
          alignItems: 'center',
          justifyContent: 'center',
          borderBottom: '1px solid rgba(255,255,255,0.1)',
        }}>
          <img src="/logo.png" alt="BotsNest" style={{ height: 28, width: 28, marginRight: 8 }} />
          <span style={{ color: '#fff', fontSize: 18, fontWeight: 600 }}>BotsNest</span>
        </div>
        <Menu
          theme="dark"
          mode="inline"
          selectedKeys={[selectedKey]}
          items={menuItems}
          onClick={({ key }) => navigate(key)}
        />
      </Sider>
      <Layout>
        <Content style={{
          margin: 24,
          padding: 24,
          borderRadius: 8,
          ...(selectedKey === '/bots'
            ? {
                background: 'linear-gradient(rgba(255,255,255,0.98), rgba(255,255,255,0.98)), url(/bots-nest1.jpeg) center / cover no-repeat',
              }
            : { background: '#fff' }),
        }}>
          <Routes>
            <Route path="/" element={<Navigate to="/llm-providers" replace />} />
            <Route path="/llm-providers" element={<LLMProviders />} />
            <Route path="/mcps" element={<MCPs />} />
            <Route path="/bots" element={<Bots />} />
            <Route path="/bots/new" element={<BotNew />} />
            <Route path="/bots/:id/edit" element={<BotEdit />} />
            <Route path="/bots/:id" element={<BotDetail />} />
            <Route path="/knowledge-bases" element={<KnowledgeBases />} />
            <Route path="/knowledge-bases/:id" element={<KnowledgeBaseDetail />} />
            <Route path="/scheduled-tasks" element={<ScheduledTasks />} />
          </Routes>
        </Content>
      </Layout>
    </Layout>
  )
}
