import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { Buffer } from 'node:buffer'
import ts from 'typescript'

const source = await readFile(new URL('../src/utils/postLogin.ts', import.meta.url), 'utf8')
const compiled = ts.transpileModule(source, {
  compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
}).outputText
const { resolvePostLoginPath } = await import('data:text/javascript;base64,' + Buffer.from(compiled).toString('base64'))

for (const input of [undefined, '', '/account', '/login', '/register', '/setup', '/settings', '/unknown', 'https://example.com', 'http://example.com', '//example.com', 'javascript:alert(1)', '/\\example.com']) {
  assert.equal(resolvePostLoginPath(input), '/', 'unsafe post-login target accepted: ' + String(input))
}

assert.equal(resolvePostLoginPath('/tasks'), '/tasks')
assert.equal(resolvePostLoginPath('/tasks?new=1#top'), '/tasks?new=1#top')
assert.equal(resolvePostLoginPath('/admin/users?status=pending'), '/admin/users?status=pending')

console.log('post-login redirect checks passed')
