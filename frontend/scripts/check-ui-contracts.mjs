import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'

const account = await readFile(new URL('../src/pages/AccountPage.vue', import.meta.url), 'utf8')
const app = await readFile(new URL('../src/App.vue', import.meta.url), 'utf8')
const home = await readFile(new URL('../src/pages/HomePage.vue', import.meta.url), 'utf8')

for (const expected of ['v-if="passwordOpen"', 'aria-expanded="passwordOpen"', 'aria-controls="password-settings"', 'autocomplete="current-password"', 'autocomplete="new-password"', 'onBeforeUnmount(clearPasswordForm)', 'currentPasswordInput.value?.focus()']) {
  assert(account.includes(expected), `account settings contract is missing ${expected}`)
}
assert(account.indexOf('v-if="passwordOpen"') < account.indexOf('id="current-password"'), 'password inputs are not guarded by the collapsed panel')
assert(!account.includes('window.alert'), 'account page must not use browser alerts')
for (const name of ['home', 'tasks', 'breeding', 'map', 'account']) assert(app.includes(`<AppIcon name="${name}"`), `navigation icon is missing: ${name}`)
for (const obsolete of ['⌂', '✓', '∞', '⌖', '○']) assert(!app.includes(obsolete), `navigation still contains Unicode icon ${obsolete}`)
assert(!home.includes('signal-orbit'), 'home still renders the oversized signal ornament')
assert(!home.includes('当前服务器'), 'home still renders the redundant server label')
assert(!home.includes('server?.description'), 'home still reads the server description')
assert(!home.includes('class="description"'), 'home still renders a server description container')
for (const expected of ['status.label', 'server?.version', 'server?.name', 'server?.onlinePlayers', 'server?.maxPlayers']) {
  assert(home.includes(expected), `home server card is missing ${expected}`)
}
assert(home.includes('APP_VERSION_LABEL'), 'home does not use the shared version label')

console.log('UI contract checks passed')
