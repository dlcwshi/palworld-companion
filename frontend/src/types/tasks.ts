export type TaskStatus = 'pending' | 'completed'

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
}

export interface TaskListResponse {
  tasks: Task[]
  total: number
}

export interface TaskCreateInput { title: string; notes: string }
export interface TaskUpdateInput { title?: string; notes?: string; status?: TaskStatus; sortOrder?: number }
