import type { Task } from '@/types/tasks'

export interface HomeTaskGroups {
  personalTasks: Task[]
  sharedTasks: Task[]
  personalTotal: number
  sharedTotal: number
  totalIncompleteTasks: number
}

export function groupHomeTasks(
  tasks: readonly Task[],
  currentUserId: number | null | undefined,
  previewLimit = 5,
): HomeTaskGroups {
  const unique = new Map<number, Task>()
  for (const task of tasks) {
    if (task.status === 'pending' && !unique.has(task.id)) unique.set(task.id, task)
  }

  const personalTasks: Task[] = []
  const sharedTasks: Task[] = []
  for (const task of unique.values()) {
    if (task.visibility === 'shared') sharedTasks.push(task)
    else if (task.owner?.id === currentUserId) personalTasks.push(task)
  }

  const personalTotal = personalTasks.length
  const sharedTotal = sharedTasks.length
  const totalIncompleteTasks = personalTotal + sharedTotal
  return {
    personalTasks: personalTasks.slice(0, previewLimit),
    sharedTasks: sharedTasks.slice(0, previewLimit),
    personalTotal,
    sharedTotal,
    totalIncompleteTasks,
  }
}
