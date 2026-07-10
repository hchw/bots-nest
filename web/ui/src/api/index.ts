import axios from 'axios'

const api = axios.create({
  baseURL: '/api',
})

export interface LLMProvider {
  id: string
  name: string
  endpoint: string
  models: string
  enabled: boolean
  created_at: string
  updated_at: string
}

export interface MCP {
  id: string
  name: string
  type: string
  endpoint: string
  command: string
  args: string
  env: string
  tools: string
  enabled: boolean
  created_at: string
}

export interface Bot {
  id: string
  name: string
  status: string
  llm_provider_id: string
  llm_model: string
  llm_temperature: number | null
  llm_max_tokens: number | null
  max_session_tokens: number
  enabled: boolean
  created_at: string
}

export interface Session {
  session_key: string
  bot_id: string
  user_id: string
  user_name: string
  conversation_type: string
  group_id: string
  created_at: string
  updated_at: string
}

export interface Message {
  id: number
  session_key: string
  role: string
  content: string
  tokens: number
  expired: boolean
  created_at: string
}

export interface Skill {
  id: number
  bot_id: string
  name: string
  description: string
  system_prompt: string
  tools: string
  enabled: boolean
}

export interface GoJudgeTool {
  id: number
  bot_id: string
  skill_id: number
  name: string
  language: string
  code: string
  input_params: string
  output_params: string
  prompt: string
  status: string
  created_at: string
  updated_at: string
}

export interface DebugResult {
  stdout: string
  stderr: string
  status: number
  error?: string
}

export const getLLMProviders = () => api.get<LLMProvider[]>('/llm-providers')
export const getProviderModels = (id: string) => api.get<{ models: string[] }>(`/llm-providers/${id}/models`)
export const createLLMProvider = (data: Partial<LLMProvider> & { name: string; endpoint: string; api_key: string }) =>
  api.post<LLMProvider>('/llm-providers', data)
export const updateLLMProvider = (id: string, data: Partial<LLMProvider>) =>
  api.put<LLMProvider>(`/llm-providers/${id}`, data)
export const deleteLLMProvider = (id: string) =>
  api.delete(`/llm-providers/${id}`)

export const getMCPs = () => api.get<MCP[]>('/mcps')
export const createMCP = (data: { id: string; name: string; type?: string; endpoint?: string; command?: string; args?: string[] }) =>
  api.post<MCP>('/mcps', data)
export const updateMCP = (id: string, data: Partial<MCP>) =>
  api.put<MCP>(`/mcps/${id}`, data)
export const deleteMCP = (id: string) =>
  api.delete(`/mcps/${id}`)

export const getBots = () => api.get<Bot[]>('/bots')
export const getBot = (id: string) => api.get<Bot>(`/bots/${id}`)
export const createBot = (data: any) => api.post<Bot>('/bots', data)
export const updateBot = (id: string, data: any) => api.put<Bot>(`/bots/${id}`, data)
export const deleteBot = (id: string) => api.delete(`/bots/${id}`)
export const startBot = (id: string) => api.post(`/bots/${id}/start`)
export const stopBot = (id: string) => api.post(`/bots/${id}/stop`)

export const getBotSessions = (id: string, page = 1, pageSize = 20) =>
  api.get<{ sessions: Session[]; total: number }>(`/bots/${id}/sessions`, { params: { page, page_size: pageSize } })
export const getBotSkills = (id: string) => api.get<Skill[]>(`/bots/${id}/skills`)
export const createSkill = (botId: string, data: { name: string; description: string; system_prompt: string; tools?: string }) =>
  api.post<Skill>(`/bots/${botId}/skills`, data)
export const updateSkill = (botId: string, skillId: number, data: Partial<Skill>) =>
  api.put<Skill>(`/bots/${botId}/skills/${skillId}`, data)
export const deleteSkill = (botId: string, skillId: number) =>
  api.delete(`/bots/${botId}/skills/${skillId}`)

export const getSession = (key: string) =>
  api.get<{ session: Session; messages: Message[] }>(`/sessions/${key}`)
export const expireSession = (key: string) => api.post(`/sessions/${key}/expire`)
export const deleteSession = (key: string) => api.delete(`/sessions/${key}`)

export const polishCode = (botId: string, data: { prompt: string; language: string; code?: string }) =>
  api.post<{ code: string }>(`/bots/${botId}/polish-code`, data)

export const getSkillTools = (botId: string, skillId: number) =>
  api.get<GoJudgeTool[]>(`/bots/${botId}/skills/${skillId}/tools`)
export const createSkillTool = (botId: string, skillId: number, data: { name: string; language: string; code?: string; input_params?: string; output_params?: string; prompt?: string; status?: string }) =>
  api.post<GoJudgeTool>(`/bots/${botId}/skills/${skillId}/tools`, data)
export const updateSkillTool = (botId: string, skillId: number, toolId: number, data: any) =>
  api.put<GoJudgeTool>(`/bots/${botId}/skills/${skillId}/tools/${toolId}`, data)
export const deleteSkillTool = (botId: string, skillId: number, toolId: number) =>
  api.delete(`/bots/${botId}/skills/${skillId}/tools/${toolId}`)
export const polishSkillTool = (botId: string, skillId: number, toolId: number) =>
  api.post<{ code: string; prompt: string }>(`/bots/${botId}/skills/${skillId}/tools/${toolId}/polish`)
export const debugSkillTool = (botId: string, skillId: number, toolId: number, data?: { language?: string; code?: string }) =>
	api.post<DebugResult>(`/bots/${botId}/skills/${skillId}/tools/${toolId}/debug`, data)

export interface KnowledgeBase {
  id: string
  name: string
  description: string
  auto_summary: string
  embedding_mode: string
  embedding_provider_id: string
  embedding_model: string
  file_count: number
  created_at: string
  updated_at: string
}

export interface ImportTask {
  id: string
  kb_id: string
  file_name: string
  file_size: number
  status: string
  total_chunks: number
  processed_chunks: number
  error: string
  created_at: string
  updated_at: string
}

export interface BotBinding {
  id: number
  bot_id: string
  kb_id: string
}

export const getKnowledgeBases = () => api.get<KnowledgeBase[]>('/knowledge-bases')
export const createKnowledgeBase = (data: { id: string; name: string; description?: string; embedding_mode?: string; embedding_provider_id?: string; embedding_model?: string }) =>
  api.post<KnowledgeBase>('/knowledge-bases', data)
export const getKnowledgeBase = (id: string) => api.get<KnowledgeBase>(`/knowledge-bases/${id}`)
export const updateKnowledgeBase = (id: string, data: { name?: string; description?: string; embedding_mode?: string; embedding_provider_id?: string; embedding_model?: string }) =>
  api.put<KnowledgeBase>(`/knowledge-bases/${id}`, data)
export const deleteKnowledgeBase = (id: string) => api.delete(`/knowledge-bases/${id}`)
export const uploadFile = (kbId: string, file: File) => {
  const formData = new FormData()
  formData.append('file', file)
  return api.post<{ task_id: string }>(`/knowledge-bases/${kbId}/upload`, formData)
}
export const getImportTasks = (kbId: string) => api.get<ImportTask[]>(`/knowledge-bases/${kbId}/tasks`)
export const getImportTask = (taskId: string) => api.get<ImportTask>(`/import-tasks/${taskId}`)
export const deleteImportTask = (kbId: string, taskId: string) => api.delete(`/knowledge-bases/${kbId}/tasks/${taskId}`)
export const reimportFile = (kbId: string, taskId: string) => api.post(`/knowledge-bases/${kbId}/tasks/${taskId}/reload`)
export const getBotBindings = (botId: string) => api.get<BotBinding[]>(`/bots/${botId}/bindings`)
export const updateBotBindings = (botId: string, data: { kb_ids: string[] }) =>
  api.put<BotBinding[]>(`/bots/${botId}/bindings`, data)

export interface TaskPlugin {
  id: string
  name: string
  type: string
  description: string
  enabled: boolean
  created_at: string
}

export interface GlobalTask {
  id: string
  name: string
  task_type: string
  cron_expr: string
  interval_sec: number
  route: string
  content: string
  enabled: boolean
  bot_ids?: string[]
  created_at: string
  updated_at: string
}

export interface TaskBinding {
  id: string
  task_id: string
  bot_id: string
  created_at: string
}

export interface TaskExecutionLog {
  id: string
  task_id: string
  task_type: string
  bot_id: string
  session_key: string
  status: string
  result: string
  trigger_type: string
  executed_at: string
}

export const getTaskPlugins = () => api.get<TaskPlugin[]>('/tasks/plugins')
export const getGlobalTasks = () => api.get<GlobalTask[]>('/tasks/global-tasks')
export const createGlobalTask = (data: any) => api.post<GlobalTask>('/tasks/global-tasks', data)
export const updateGlobalTask = (id: string, data: any) => api.put(`/tasks/global-tasks/${id}`, data)
export const deleteGlobalTask = (id: string) => api.delete(`/tasks/global-tasks/${id}`)
export const getTaskBindings = (taskId: string) => api.get<TaskBinding[]>(`/tasks/global-tasks/${taskId}/bindings`)
export const updateTaskBindings = (taskId: string, data: { bot_ids: string[] }) => api.post(`/tasks/global-tasks/${taskId}/bindings`, data)
export const getExecutionLogs = (taskId?: string) => api.get<TaskExecutionLog[]>('/tasks/execution-logs', { params: { task_id: taskId } })
export const parseLLMTask = (description: string) => api.post<{ parsed: any }>('/tasks/parse-llm-task', { description })
