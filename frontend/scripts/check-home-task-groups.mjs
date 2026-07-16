import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { Buffer } from 'node:buffer'
import ts from 'typescript'

const source = await readFile(new URL('../src/utils/homeTasks.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText
const { groupHomeTasks } = await import(`data:text/javascript;base64,${Buffer.from(compiled).toString('base64')}`)

const task = (id, visibility, ownerId, title = `Task ${id}`) => ({
  id,
  title,
  status: 'pending',
  visibility,
  owner: ownerId == null ? null : { id: ownerId },
})
const ids = (tasks) => tasks.map((item) => item.id)

let result = groupHomeTasks([task(1, 'shared', 7), task(2, 'shared', 7)], 7)
assert.equal(result.totalIncompleteTasks, 2)
assert.equal(result.personalTotal, 0)
assert.deepEqual(ids(result.sharedTasks), [1, 2])

result = groupHomeTasks([task(1, 'personal', 7), task(2, 'shared', 7), task(3, 'shared', 8)], 7)
assert.equal(result.totalIncompleteTasks, 3)
assert.deepEqual(ids(result.personalTasks), [1])
assert.deepEqual(ids(result.sharedTasks), [2, 3])

result = groupHomeTasks([task(1, 'personal', 8), task(2, 'shared', 8)], 7)
assert.equal(result.personalTotal, 0)
assert.deepEqual(ids(result.sharedTasks), [2])

result = groupHomeTasks([task(1, 'shared', 7, 'Same'), task(2, 'shared', 7, 'Same')], 7)
assert.deepEqual(ids(result.sharedTasks), [1, 2])

result = groupHomeTasks([task(1, 'personal', 7), task(1, 'personal', 7), task(2, 'shared', 7)], 7)
assert.equal(result.totalIncompleteTasks, 2)
assert.deepEqual([...ids(result.personalTasks), ...ids(result.sharedTasks)], [1, 2])

console.log('home task grouping checks passed')
