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
  endpoint: string
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

export const getLLMProviders = () => api.get<LLMProvider[]>('/llm-providers')
export const getProviderModels = (id: string) => api.get<{ models: string[] }>(`/llm-providers/${id}/models`)
export const createLLMProvider = (data: Partial<LLMProvider> & { name: string; endpoint: string; api_key: string }) =>
  api.post<LLMProvider>('/llm-providers', data)
export const updateLLMProvider = (id: string, data: Partial<LLMProvider>) =>
  api.put<LLMProvider>(`/llm-providers/${id}`, data)
export const deleteLLMProvider = (id: string) =>
  api.delete(`/llm-providers/${id}`)

export const getMCPs = () => api.get<MCP[]>('/mcps')
export const createMCP = (data: { id: string; name: string; endpoint: string }) =>
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
