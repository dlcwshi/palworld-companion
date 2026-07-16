import assert from 'node:assert/strict'
import { access, readFile } from 'node:fs/promises'
import { resolve } from 'node:path'

const dist = resolve(import.meta.dirname, '../../web/dist')
const index = await readFile(resolve(dist, 'index.html'), 'utf8')
const assetPaths = [...index.matchAll(/(?:src|href)="\/(assets\/[^"?]+)(?:\?[^"?]*)?"/g)].map((match) => match[1])
assert(assetPaths.some((path) => path.endsWith('.js')), 'index.html does not reference a JavaScript bundle')
assert(assetPaths.some((path) => path.endsWith('.css')), 'index.html does not reference a CSS bundle')
await Promise.all(assetPaths.map((path) => access(resolve(dist, path))))

const textAssets = await Promise.all(assetPaths.filter((path) => /\.(?:js|css)$/.test(path)).map((path) => readFile(resolve(dist, path), 'utf8')))
const bundle = textAssets.join('\n')
for (const expected of ['TASKS', 'PLAYERS', 'V0.4.2 DEV', '个人任务', '共享任务']) {
  assert(bundle.includes(expected), `production bundle is missing ${expected}`)
}
for (const obsolete of ['TONIGHT', '今晚任务', '在线玩家', '缓存命中', '显示缓存数据', '安全边界', 'V0.4.1 DEV']) {
  assert(!bundle.includes(obsolete), `production bundle still contains ${obsolete}`)
}

const worker = await readFile(resolve(dist, 'sw.js'), 'utf8')
for (const assetPath of assetPaths) assert(worker.includes(assetPath), `service worker does not precache ${assetPath}`)
assert(worker.includes('skipWaiting'), 'service worker does not activate updates immediately')
assert(worker.includes('clientsClaim'), 'service worker does not claim open clients')
assert(!/\{url:["']\/api\//.test(worker), 'service worker must not precache API routes')

console.log(`frontend build verified (${assetPaths.join(', ')})`)
