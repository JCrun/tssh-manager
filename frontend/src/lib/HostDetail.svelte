<script lang="ts">
  import { api, errText, authLabel, endpoint, problemText, type Host, type CheckResult } from './api'

  let { host, check, checking, oncheck, onedit, onkey, onchanged, notify }: {
    host: Host
    check?: CheckResult
    checking: boolean
    oncheck: () => void
    onedit: () => void
    onkey: () => void
    onchanged: (msg: string, select?: string | null) => void
    notify: (msg: string, tone?: 'ok' | 'bad') => void
  } = $props()

  let raw = $state('')
  let rawMode = $state(false)
  let busy = $state(false)
  let error = $state('')
  let confirmDelete = $state(false)

  const auth = $derived(authLabel(host))
  const eff = $derived(host.effective || {})

  function startRaw() { raw = host.raw; rawMode = true; error = '' }

  async function run(fn: () => Promise<void>) {
    busy = true; error = ''
    try { await fn() } catch (e) { error = errText(e) } finally { busy = false }
  }
  const connect = () => run(async () => { await api.Connect(host.alias); notify(`已在终端打开 tssh ${host.alias}`) })
  const saveRaw = () => run(async () => {
    await api.SaveRaw(host.alias, raw)
    const m = raw.match(/^\s*host\s+(\S+)/i)
    rawMode = false
    onchanged('已保存原文', m ? m[1] : host.alias)
  })
  const del = () => run(async () => { await api.DeleteHost(host.alias); onchanged(`已删除 ${host.alias}`, null) })
  const removePw = () => run(async () => { await api.RemovePassword(host.alias); onchanged('已删除保存的密码') })

  function fmtMs(ms: number) { return ms >= 1000 ? (ms / 1000).toFixed(1) + 's' : ms + 'ms' }
</script>

<div class="detail">
  <header>
    <div>
      <h2>{host.patterns.join(' ')}</h2>
      <div class="sub">
        {#if host.wildcard}<span class="badge">通配规则</span>{:else}<code>{endpoint(host)}</code>{/if}
        {#if !host.wildcard}<span class="badge {auth.tone}">{auth.text}</span>{/if}
        {#each host.groups as g}<span class="badge">{g}</span>{/each}
        <span class="line">第 {host.line} 行</span>
      </div>
    </div>
    {#if !host.wildcard}
      <div class="btns">
        <button onclick={oncheck} disabled={checking}>检查</button>
        <button class="primary" onclick={connect} disabled={busy} title="在终端里运行 tssh {host.alias}">连接</button>
      </div>
    {/if}
  </header>

  {#if !host.wildcard}
    <section class="status">
      {#if checking}
        <span class="dot run"></span> 检查中…
      {:else if check}
        <span class="dot {check.login === 'ok' ? 'ok' : check.tcp === 'fail' || check.login === 'fail' ? 'bad' : 'warn'}"></span>
        {#if check.login === 'ok'}
          可登录 · TCP {check.tcp === 'ok' ? fmtMs(check.tcpMs) : '经跳板'} · 登录 {fmtMs(check.loginMs)}
        {:else}
          {problemText[check.problem] ?? check.problem}{check.tcp === 'ok' ? `（TCP 正常 ${fmtMs(check.tcpMs)}）` : ''}
          <div class="msg mono">{check.message}</div>
        {/if}
        <span class="faint">· {check.target} · {new Date(check.finished).toLocaleTimeString()}</span>
      {:else}
        <span class="faint">还没检查。点上方「检查」或工具栏「全部检查」。</span>
      {/if}
    </section>

    {#if host.plainPassword}
      <section class="alert">
        配置里存着<b>明文密码</b>（<code>#!! Password</code>）。建议改用密钥。
        <div class="row"><button onclick={onkey}>改用密钥…</button><button class="ghost" onclick={removePw} disabled={busy}>只删除密码</button></div>
      </section>
    {/if}

    <section class="kv">
      <div><span>地址</span><code>{host.hostName || eff.HostName || host.alias}</code></div>
      <div><span>端口</span><code>{host.port || eff.Port || '22'}</code></div>
      <div><span>用户</span><code>{host.user || eff.User || '（本机用户名）'}</code></div>
      <div><span>私钥</span><code>{host.identityFile || (eff.IdentityFile ? eff.IdentityFile + '（默认）' : '—')}</code></div>
      {#if host.proxyJump || eff.ProxyJump}<div><span>跳板</span><code>{host.proxyJump || eff.ProxyJump}</code></div>{/if}
      {#if host.identitiesOnly}<div><span>IdentitiesOnly</span><code>yes</code></div>{/if}
    </section>
  {/if}

  <section>
    <div class="row between">
      <h3>配置原文</h3>
      {#if !rawMode && host.patterns.length === 1 && !host.wildcard}
        <div class="row">
          <button onclick={onedit}>编辑</button>
          <button onclick={startRaw}>编辑原文</button>
          {#if !host.plainPassword && host.auth !== 'key'}<button onclick={onkey}>改用密钥…</button>{/if}
        </div>
      {/if}
    </div>
    {#if rawMode}
      <textarea rows={Math.max(6, raw.split('\n').length + 2)} bind:value={raw} spellcheck="false"></textarea>
      <p class="faint small">密码显示为 <code>******</code>，保持不动会保留原值。保存前会用 tssh 解析器和 <code>ssh -G</code> 校验。</p>
      <div class="row end">
        <button onclick={() => (rawMode = false)} disabled={busy}>取消</button>
        <button class="primary" onclick={saveRaw} disabled={busy}>保存原文</button>
      </div>
    {:else}
      <pre class="mono">{host.raw}</pre>
      {#if host.wildcard}<p class="faint small">通配和 Match 规则请直接在配置文件里修改。</p>{/if}
    {/if}
  </section>

  {#if !host.wildcard && host.patterns.length === 1}
    <section class="danger-zone">
      {#if confirmDelete}
        确定删除 <b>{host.alias}</b>？会先自动备份配置。
        <button class="danger" onclick={del} disabled={busy}>删除</button>
        <button onclick={() => (confirmDelete = false)}>取消</button>
      {:else}
        <button class="ghost danger" onclick={() => (confirmDelete = true)}>删除这台主机…</button>
      {/if}
    </section>
  {/if}

  {#if error}<p class="err">{error}</p>{/if}
</div>

<style>
  .detail { padding: 18px 20px 28px; display: grid; gap: 16px; max-width: 760px; }
  header { display: flex; justify-content: space-between; gap: 12px; align-items: flex-start; }
  h2 { margin: 0 0 6px; font-size: 18px; word-break: break-all; }
  h3 { margin: 0; font-size: 13px; color: var(--muted); font-weight: 600; }
  .sub { display: flex; gap: 8px; flex-wrap: wrap; align-items: center; }
  .line { color: var(--faint); font-size: 12px; }
  .btns { display: flex; gap: 8px; }
  .status { background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px 12px; }
  .msg { margin-top: 4px; color: var(--muted); white-space: pre-wrap; word-break: break-all; }
  .faint { color: var(--faint); }
  .small { font-size: 12px; margin: 6px 0; }
  .dot { display: inline-block; width: 8px; height: 8px; border-radius: 50%; margin-right: 6px; vertical-align: 1px; background: var(--faint); }
  .dot.ok { background: var(--ok); } .dot.bad { background: var(--bad); } .dot.warn { background: var(--warn); }
  .dot.run { background: var(--accent); animation: pulse 1s infinite; }
  @keyframes pulse { 50% { opacity: .3; } }
  .alert { border: 1px solid color-mix(in srgb, var(--warn) 45%, transparent); background: color-mix(in srgb, var(--warn) 9%, transparent); border-radius: var(--radius); padding: 10px 12px; }
  .alert .row { margin-top: 8px; }
  .kv { display: grid; grid-template-columns: repeat(auto-fill, minmax(220px, 1fr)); gap: 8px 16px; }
  .kv div { display: flex; flex-direction: column; gap: 2px; min-width: 0; }
  .kv span { color: var(--faint); font-size: 12px; }
  .kv code { word-break: break-all; }
  pre { margin: 8px 0 0; background: var(--panel); border: 1px solid var(--line); border-radius: var(--radius); padding: 10px 12px; overflow: auto; white-space: pre; }
  textarea { margin-top: 8px; }
  .row { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
  .between { justify-content: space-between; }
  .end { justify-content: flex-end; }
  .danger-zone { display: flex; gap: 8px; align-items: center; color: var(--muted); }
  .err { color: var(--bad); white-space: pre-wrap; background: color-mix(in srgb, var(--bad) 10%, transparent); padding: 8px 10px; border-radius: 6px; margin: 0; }
</style>
