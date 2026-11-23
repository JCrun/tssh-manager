<script lang="ts">
  import { api, errText, type Host, type KeyInfo } from './api'

  let { host, onchanged, onclose }: { host: Host; onchanged: (backup: string) => void; onclose: () => void } = $props()

  let key = $state<KeyInfo | null>(null)
  let installed = $state<'' | 'done' | 'terminal'>('')
  let verified = $state(false)
  let busy = $state('')
  let error = $state('')
  let finished = $state(false)
  // svelte-ignore state_referenced_locally
  const stored = host.plainPassword || host.encPassword

  async function step(name: string, fn: () => Promise<void>) {
    busy = name
    error = ''
    try { await fn() } catch (e) { error = errText(e) } finally { busy = '' }
  }
  const gen = () => step('gen', async () => { key = await api.GenerateKey(host.alias) })
  const install = () => step('install', async () => { installed = (await api.InstallKey(host.alias, key!.path, stored)) as any })
  const verify = () => step('verify', async () => { await api.VerifyKey(host.alias, key!.path); verified = true })
  const swap = () => step('swap', async () => { const b = await api.SwitchToKey(host.alias, key!.path); finished = true; onchanged(b) })
</script>

<div class="wiz">
  <h2>改用密钥登录：{host.alias}</h2>
  <ol>
    <li class:done={key}>
      <div class="row">
        <span>生成专用密钥（ed25519，无口令）</span>
        <button onclick={gen} disabled={!!busy || !!key}>{busy === 'gen' ? '生成中…' : key ? '已就绪' : '生成'}</button>
      </div>
      {#if key}
        <p class="sub"><code>{key.display}</code> {key.created ? '（新建）' : '（已存在，直接复用）'}</p>
      {/if}
    </li>
    <li class:done={installed}>
      <div class="row">
        <span>把公钥装到服务器</span>
        <button onclick={install} disabled={!key || !!busy || installed === 'done'}>{busy === 'install' ? '安装中…' : installed === 'done' ? '已安装' : '安装'}</button>
      </div>
      <p class="sub">
        {#if stored}用 tssh 保存的密码在后台登录一次，把公钥追加到 <code>~/.ssh/authorized_keys</code>（已有则不重复）。
        {:else}这台没有保存密码：会打开终端运行 <code>ssh-copy-id</code>，在终端里输入一次密码。{/if}
      </p>
      {#if installed === 'terminal'}<p class="sub warn">在打开的终端里完成密码输入后，再点下一步验证。</p>{/if}
    </li>
    <li class:done={verified}>
      <div class="row">
        <span>只用这把密钥试登录</span>
        <button onclick={verify} disabled={!installed || !!busy || verified}>{busy === 'verify' ? '验证中…' : verified ? '通过' : '验证'}</button>
      </div>
      <p class="sub">不带密码、不用其他密钥和 agent，确认新密钥本身能登录。</p>
    </li>
    <li class:done={finished}>
      <div class="row">
        <span>写入配置</span>
        <button class="primary" onclick={swap} disabled={!verified || !!busy || finished}>{busy === 'swap' ? '保存中…' : finished ? '已完成' : '切换'}</button>
      </div>
      <p class="sub">设置 <code>IdentityFile</code> 和 <code>IdentitiesOnly yes</code>{stored ? '，删除配置里保存的密码' : ''}。服务器端是否关闭密码登录不在这里改动。</p>
    </li>
  </ol>
  {#if error}<p class="err">{error}</p>{/if}
  <div class="actions"><button onclick={onclose}>{finished ? '完成' : '关闭'}</button></div>
</div>

<style>
  .wiz { padding: 18px 20px; max-width: 640px; }
  h2 { margin: 0 0 12px; font-size: 16px; }
  ol { margin: 0; padding-left: 20px; display: grid; gap: 12px; }
  li::marker { color: var(--faint); }
  li.done::marker { color: var(--ok); }
  .row { display: flex; align-items: center; justify-content: space-between; gap: 12px; }
  .sub { margin: 4px 0 0; color: var(--muted); font-size: 12.5px; }
  .warn { color: var(--warn); }
  .err { color: var(--bad); white-space: pre-wrap; background: color-mix(in srgb, var(--bad) 10%, transparent); padding: 8px 10px; border-radius: 6px; }
  .actions { display: flex; justify-content: flex-end; margin-top: 16px; }
</style>
