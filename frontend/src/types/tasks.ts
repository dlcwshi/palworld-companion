export type TaskStatus = 'pending' | 'completed'
export type TaskVisibility = 'personal' | 'shared'

export interface Task {
  id: number
  title: string
  notes: string
  status: TaskStatus
  sortOrder: number
  sourceType: 'manual' | 'crafting_plan'
  sourceId: number | null
  createdAt: string
  updatedAt: string
  completedAt: string | null
	visibility: TaskVisibility
	owner: { id:number; characterName:string; status:string } | null
	canManage: boolean
}

export interface TaskListResponse {
  tasks: Task[]
  total: number
}

export interface TaskCreateInput { title: string; notes: string; visibility:TaskVisibility }
export interface TaskUpdateInput { title?: string; notes?: string; status?: TaskStatus; sortOrder?: number }
