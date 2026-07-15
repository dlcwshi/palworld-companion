<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { useRoute } from 'vue-router'
import { useTaskStore } from '@/stores/tasks'
import { useAuthStore } from '@/stores/auth'
import type { Task, TaskStatus } from '@/types/tasks'

const route = useRoute()
const store = useTaskStore()
const auth = useAuthStore()
const filter = ref<TaskStatus | 'all'>('all')
const scope = ref<'mine'|'shared'>('mine')
const visibility = ref<'personal'|'shared'>('personal')
const editingId = ref<number | null>(null)
const formOpen = ref(false)
const title = ref('')
const notes = ref('')
const sortOrder = ref(0)
const formError = ref<string | null>(null)

const setFilter = async (value: TaskStatus | 'all') => {
  filter.value = value
  await store.load(value,100,scope.value)
}
const setScope=async(value:'mine'|'shared')=>{scope.value=value;await store.load(filter.value,100,value)}
const resetForm = () => {
  formOpen.value = false
  editingId.value = null
  title.value = ''
  notes.value = ''
  sortOrder.value = 0
  formError.value = null
}
const startNew = () => {
  resetForm()
  formOpen.value = true
}
const startEdit = (task: Task) => {
  editingId.value = task.id
  title.value = task.title
  notes.value = task.notes
  sortOrder.value = task.sortOrder
  formError.value = null
  formOpen.value = true
  window.scrollTo({ top: 0, behavior: 'smooth' })
}
const submit = async () => {
  const cleanTitle = title.value.trim()
  if (!cleanTitle) {
    formError.value = '请输入任务标题'
    return
  }
  const ok = editingId.value == null
    ? await store.create({ title: cleanTitle, notes: notes.value.trim(), visibility: visibility.value })
    : await store.update(editingId.value, { title: cleanTitle, notes: notes.value.trim(), sortOrder: sortOrder.value })
  if (ok) resetForm()
}
const toggle = async (task: Task) => {
  await store.update(task.id, { status: task.status === 'pending' ? 'completed' : 'pending' })
}
const remove = async (task: Task) => {
  if (!window.confirm(`确认删除“${task.title}”吗？此操作无法撤销。`)) return
  if (await store.remove(task.id) && editingId.value === task.id) resetForm()
}

onMounted(async () => {
  await auth.refresh()
  if(auth.authenticated) await store.load('all',100,scope.value)
  if (route.query.new === '1') startNew()
})
</script>

<template>
  <div class="tasks-page">
    <header class="task-page-header">
      <div><p class="eyebrow">TONIGHT · SQLITE</p><h1>今晚任务</h1><p>把今晚要完成的事放在这里，重启后仍会保留。</p></div>
      <button class="round-add-button" type="button" aria-label="新建任务" @click="startNew">＋</button>
    </header>
    <section v-if="!auth.authenticated" class="prose-card"><h2>使用 Steam 登录</h2><p>首次登录前，请先进入本 Palworld 服务器并保持角色在线。</p><button class="primary-button" type="button" @click="auth.login('/tasks')">使用 Steam 登录</button></section>
    <template v-if="auth.authenticated">
      <div class="task-filter"><button :class="{active:scope==='mine'}" @click="setScope('mine')">我的任务</button><button :class="{active:scope==='shared'}" @click="setScope('shared')">共享任务</button></div>

      <form v-if="formOpen" class="task-form" @submit.prevent="submit">
        <div class="section-heading"><h2>{{ editingId == null ? '新建任务' : '编辑任务' }}</h2><button type="button" class="text-button" @click="resetForm">取消</button></div>
        <label>任务标题<input v-model="title" maxlength="200" autocomplete="off" placeholder="例如：准备精炼金属锭材料" /></label>
        <label>备注<textarea v-model="notes" maxlength="4000" rows="3" placeholder="可选：记录材料、地点或步骤" /></label>
        <label v-if="editingId==null">可见范围<select v-model="visibility"><option value="personal">仅自己可见</option><option value="shared">全服共享</option></select></label>
        <label v-if="editingId != null">排序值<input v-model.number="sortOrder" type="number" inputmode="numeric" /><small>数值越小越靠前</small></label>
        <p v-if="formError" class="form-error">{{ formError }}</p>
        <button class="primary-button" type="submit" :disabled="store.saving">{{ store.saving ? '正在保存…' : editingId == null ? '添加到今晚' : '保存修改' }}</button>
      </form>

      <div class="task-filter" role="group" aria-label="筛选任务">
        <button v-for="item in [{ value: 'all', label: '全部' }, { value: 'pending', label: '未完成' }, { value: 'completed', label: '已完成' }]" :key="item.value" type="button" :class="{ active: filter === item.value }" @click="setFilter(item.value as TaskStatus | 'all')">{{ item.label }}</button>
      </div>

      <div v-if="store.error" class="notice danger">{{ store.error }} <button class="inline-retry" type="button" @click="store.load(filter)">重试</button></div>
      <div v-if="store.loading" class="empty-state">正在加载任务…</div>
      <div v-else-if="store.tasks.length === 0" class="task-empty">
        <span>✓</span><h2>这里还没有任务</h2><p>添加第一件今晚想完成的事。</p><button class="secondary-button" type="button" @click="startNew">新建任务</button>
      </div>
      <section v-else class="task-list" aria-label="今晚任务列表">
        <article v-for="task in store.tasks" :key="task.id" class="task-row" :class="{ completed: task.status === 'completed' }">
          <template v-if="task.canManage">
            <button class="task-check" type="button" :aria-label="task.status === 'pending' ? '标记完成' : '恢复未完成'" :disabled="store.saving" @click="toggle(task)">{{ task.status === 'completed' ? '✓' : '' }}</button>
            <div class="task-content"><strong>{{ task.title }}</strong><p v-if="task.notes">{{ task.notes }}</p><small>{{ task.status === 'completed' ? '已完成' : `排序 ${task.sortOrder}` }}</small></div>
            <div class="task-actions"><button type="button" :disabled="store.saving" @click="startEdit(task)">编辑</button><button type="button" class="danger-text" :disabled="store.saving" @click="remove(task)">删除</button></div>
          </template><template v-else><span class="task-check readonly">·</span><div class="task-content"><strong>{{ task.title }}</strong><p v-if="task.notes">{{ task.notes }}</p><small>共享 · {{ task.owner?.characterName ?? '历史任务' }}</small></div></template>
        </article>
      </section>
    </template>
  </div>
</template>
