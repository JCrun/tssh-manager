<script lang="ts">
  import { onMount } from 'svelte'
  import { api, EventsOn, errText, endpoint, type Host, type State, type CheckResult } from './lib/api'
  import HostDetail from './lib/HostDetail.svelte'
  import HostForm from './lib/HostForm.svelte'
  import KeyWizard from './lib/KeyWizard.svelte'
  import SettingsDialog from './lib/SettingsDialog.svelte'
  import BackupsDialog from './lib/BackupsDialog.svelte'
  import SyncDialog from './lib/SyncDialog.svelte'
  import { syncSummary, type SyncResult } from './lib/api'

  let st = $state<State | null>(null)
  let loadError = $state('')
  let selKey = $state<string | null>(null)
  let mode = $state<'view' | 'edit' | 'new' | 'key'>('view')
  let dialog = $state<'' | 'settings' | 'backups' | 'sync'>('')
  let syncOn = $state(false)
  let syncState = $state<'' | 'running' | 'ok' | 'conflict' | 'error'>('')
  let syncMsg = $state('')
  let syncResult = $state<SyncResult | null>(null)
  let query = $state('')
  let filter = $state<'all' | 'plain' | 'failed'>('all')
  let checks = $state<Record<string, CheckResult>>({})
  let checking = $state<Record<string, boolean>>({})
  let toast = $state<{ msg: string; tone: 'ok' | 'bad' } | null>(null)
  let toastTimer: number | undefined
  let searchEl: HTMLInputElement | undefined = $state()

  const keyOf = (h: Host) => (h.wildcard || h.patterns.length !== 1 ? `#${h.line}` : h.alias)

  function notify(msg: string, tone: 'ok' | 'bad' = 'ok') {
    toast = { msg, tone }
    clearTimeout(toastTimer)
    toastTimer = window.setTimeout(() => (toast = null), 3500)
  }

  async function load(select?: string | null) {
    try {
      st = await api.Load()
      loadError = ''
      if (select !== undefined) selKey = select
      if (selKey && !st.hosts.some((h) => keyOf(h) === selKey)) selKey = null
      if (!selKey) selKey = st.hosts.find((h) => !h.wildcard)?.alias ?? null
    } catch (e) {
      loadError = errText(e)
    }
  }

  onMount(() => {
    load()
    const offs = [
      EventsOn('check:start', (a: string) => (checking[a] = true)),
      EventsOn('check:result', (r: CheckResult) => { checks[r.alias] = r; checking[r.alias] = false }),
      EventsOn('sync:running', () => (syncState = 'running')),
      EventsOn('sync:result', (ev: { result: SyncResult | null; error: string }) => {
        if (ev.error) { syncState = 'error'; syncMsg = ev.error; notify('自动同步失败：' + ev.error, 'bad'); return }
        if (ev.result) synced(ev.result, true)
      }),
    ]
    api.GetSync().then((v) => (syncOn = !!v.backend))
    const onKey = (e: KeyboardEvent) => {
      if ((e.ctrlKey || e.metaKey) && e.key === 'f') { e.preventDefault(); searchEl?.focus() }
      if ((e.ctrlKey || e.metaKey) && e.key === 'r') { e.preventDefault(); load() }
    }
    window.addEventListener('keydown', onKey)
    return () => { offs.forEach((f) => f()); window.removeEventListener('keydown', onKey) }
  })

  const hosts = $derived(st?.hosts ?? [])
  const visible = $derived.by(() => {
    const q = query.trim().toLowerCase()
    return hosts.filter((h) => {
      if (filter === 'plain' && !h.plainPassword) return false
      if (filter === 'failed') {
        const c = checks[h.alias]
        if (h.wildcard || !c || c.login === 'ok') return false
      }
      if (!q) return true
      return [h.patterns.join(' '), h.hostName, h.user, h.section, ...(h.groups ?? []), h.effective?.HostName ?? '']
        .some((s) => (s ?? '').toLowerCase().includes(q))
    })
  })
  const groups = $derived.by(() => {
    const m = new Map<string, Host[]>()
    for (const h of visible) {
      const g = h.wildcard ? '全局规则' : h.section || '未分组'
      if (!m.has(g)) m.set(g, [])
      m.get(g)!.push(h)
    }
    const order = [...m.keys()].sort((a, b) => (a === '全局规则' ? 1 : b === '全局规则' ? -1 : 0))
    return order.map((g) => ({ name: g, hosts: m.get(g)! }))
  })
  const selected = $derived(hosts.find((h) => keyOf(h) === selKey) ?? null)
  const realHosts = $derived(hosts.filter((h) => !h.wildcard && h.patterns.length === 1))
  const plainCount = $derived(realHosts.filter((h) => h.plainPassword).length)
  const failedCount = $derived(realHosts.filter((h) => checks[h.alias] && checks[h.alias].login !== 'ok').length)
  const anyChecking = $derived(Object.values(checking).some(Boolean))

  function check(aliases: string[]) { api.Check(aliases).catch((e) => notify(errText(e), 'bad')) }
  function checkAll() { check(visible.filter((h) => !h.wildcard && h.patterns.length === 1).map((h) => h.alias)) }
  function checkOne(a: string) { check([a]) }

  function dotClass(h: Host) {
    if (h.wildcard) return 'none'
    if (checking[h.alias]) return 'run'
    const c = checks[h.alias]
    if (!c) return ''
    return c.login === 'ok' ? 'ok' : c.tcp === 'fail' || c.login === 'fail' ? 'bad' : 'warn'
  }

  async function connectSel() {
    const h = visible.find((x) => !x.wildcard && x.patterns.length === 1)
    if (!h) return
    try { await api.Connect(h.alias); notify(`已在终端打开 tssh ${h.alias}`) } catch (e) { notify(errText(e), 'bad') }
  }

  function synced(r: SyncResult, auto = false) {
    syncResult = r
    syncMsg = syncSummary(r)
    syncState = r.status === 'conflict' ? 'conflict' : 'ok'
    if (r.status === 'pulled' || r.status === 'merged') load()
    if (r.status === 'conflict') notify(syncMsg + '，点「同步」处理', 'bad')
    else if (!auto || r.status !== 'uptodate' && r.status !== 'pushed') notify(syncMsg)
  }

  function saved(msg: string, select?: string | null) {
    mode = 'view'
    notify(msg)
    load(select)
  }
</script>

<div class="app">
  <header class="bar">
    <div class="brand">tssh-manager</div>
    <input
      class="search"
      bind:this={searchEl}
      bind:value={query}
      placeholder="搜索别名、地址、用户、分组…（Ctrl+F，回车连接第一个）"
      onkeydown={(e) => { if (e.key === 'Enter') connectSel(); if (e.key === 'Escape') query = '' }}
    />
    <div class="tools">
      <button onclick={checkAll} disabled={anyChecking || !st}>{anyChecking ? '检查中…' : '全部检查'}</button>
      <button onclick={() => { mode = 'new' }}>新建</button>
      <button class="ghost" onclick={() => load()} title="重新读取配置（Ctrl+R）">刷新</button>
      <button class="ghost sync" onclick={() => (dialog = 'sync')} title={syncMsg || (syncOn ? '多端同步' : '设置多端同步')}>
        {#if syncOn}<span class="dot {syncState === 'running' ? 'run' : syncState === 'ok' ? 'ok' : syncState === 'conflict' ? 'warn' : syncState === 'error' ? 'bad' : ''}"></span>{/if}同步{syncState === 'conflict' ? '（冲突）' : ''}
      </button>
      <button class="ghost" onclick={() => (dialog = 'backups')}>备份</button>
      <button class="ghost" onclick={() => (dialog = 'settings')}>设置</button>
    </div>
  </header>

  {#if loadError}
    <div class="fatal">读取配置失败：{loadError}<button onclick={() => load()}>重试</button></div>
  {/if}

  <div class="main">
    <aside>
      <div class="filters">
        <button class:on={filter === 'all'} onclick={() => (filter = 'all')}>全部 {realHosts.length}</button>
        <button class:on={filter === 'plain'} onclick={() => (filter = 'plain')} disabled={!plainCount}>明文密码 {plainCount}</button>
        <button class:on={filter === 'failed'} onclick={() => (filter = 'failed')} disabled={!failedCount}>异常 {failedCount}</button>
      </div>
      <div class="list">
        {#each groups as g (g.name)}
          <div class="group">{g.name}<span>{g.hosts.length}</span></div>
          {#each g.hosts as h (keyOf(h))}
            <button
              class="item"
              class:sel={keyOf(h) === selKey}
              onclick={() => { selKey = keyOf(h); mode = 'view' }}
              ondblclick={() => !h.wildcard && h.patterns.length === 1 && api.Connect(h.alias).then(() => notify(`已在终端打开 tssh ${h.alias}`), (e) => notify(errText(e), 'bad'))}
              title={h.wildcard ? '' : '双击连接'}
            >
              <span class="dot {dotClass(h)}"></span>
              <span class="name">{h.patterns.join(' ')}</span>
              {#if !h.wildcard && h.plainPassword}<span class="badge warn">明文</span>{:else if !h.wildcard && h.encPassword}<span class="badge warn">弱加密</span>{/if}
              <span class="ep">{h.wildcard ? '' : endpoint(h)}</span>
            </button>
          {/each}
        {:else}
          <p class="empty">{st ? '没有匹配的主机' : '读取中…'}</p>
        {/each}
      </div>
      {#if st}<div class="foot" title={st.configPath}>{st.configPath}</div>{/if}
    </aside>

    <main>
      {#each st?.warnings ?? [] as w}<div class="warnline">{w}</div>{/each}
      {#if mode === 'new'}
        <HostForm sections={st?.sections ?? []} onsaved={(a) => saved(`已新建 ${a}`, a)} oncancel={() => (mode = 'view')} />
      {:else if selected && mode === 'edit'}
        {#key selKey}<HostForm host={selected} onsaved={(a) => saved('已保存', a)} oncancel={() => (mode = 'view')} />{/key}
      {:else if selected && mode === 'key'}
        {#key selKey}<KeyWizard host={selected} onchanged={() => { notify('已改用密钥登录'); load() }} onclose={() => (mode = 'view')} />{/key}
      {:else if selected}
        {#key selKey}
          <HostDetail
            host={selected}
            check={checks[selected.alias]}
            checking={!!checking[selected.alias]}
            oncheck={() => checkOne(selected.alias)}
            onedit={() => (mode = 'edit')}
            onkey={() => (mode = 'key')}
            onchanged={saved}
            {notify}
          />
        {/key}
      {:else if st}
        <div class="placeholder">左侧选择一台主机，或点「新建」。</div>
      {/if}
    </main>
  </div>

  {#if dialog}
    <div class="scrim" role="presentation" onclick={(e) => e.target === e.currentTarget && (dialog = '')}>
      <div class="dialog" class:wide={dialog === 'sync'}>
        {#if dialog === 'settings'}
          <SettingsDialog onclose={() => (dialog = '')} onsaved={() => { dialog = ''; notify('设置已保存'); load() }} />
        {:else if dialog === 'sync'}
          <SyncDialog
            initial={syncState === 'conflict' ? syncResult : null}
            onclose={() => { dialog = ''; api.GetSync().then((v) => (syncOn = !!v.backend)) }}
            onsynced={(r) => synced(r)}
          />
        {:else}
          <BackupsDialog onclose={() => (dialog = '')} onrestored={(m) => { dialog = ''; notify(m); load() }} />
        {/if}
      </div>
    </div>
  {/if}

  {#if toast}<div class="toast {toast.tone}">{toast.msg}</div>{/if}
</div>

<style>
  .app { height: 100%; display: flex; flex-direction: column; }
  .bar { display: flex; gap: 12px; align-items: center; padding: 10px 14px; border-bottom: 1px solid var(--line); background: var(--panel); }
  .brand { font-weight: 700; letter-spacing: .2px; }
  .search { flex: 1; max-width: 520px; }
  .tools { display: flex; gap: 6px; margin-left: auto; }
  .fatal { padding: 10px 14px; color: var(--bad); display: flex; gap: 10px; align-items: center; }
  .main { flex: 1; display: flex; min-height: 0; }
  aside { width: 320px; flex-shrink: 0; border-right: 1px solid var(--line); display: flex; flex-direction: column; min-height: 0; background: var(--panel); }
  .filters { display: flex; gap: 4px; padding: 8px 10px; border-bottom: 1px solid var(--line); }
  .filters button { font-size: 12px; padding: 2px 9px; background: transparent; border-color: transparent; color: var(--muted); }
  .filters button.on { background: var(--panel-2); border-color: var(--line); color: var(--text); }
  .list { flex: 1; overflow: auto; padding: 4px 6px 10px; }
  .group { display: flex; justify-content: space-between; color: var(--faint); font-size: 11.5px; font-weight: 600; text-transform: uppercase; letter-spacing: .4px; padding: 10px 8px 4px; }
  .item { display: grid; grid-template-columns: 12px 1fr auto; grid-template-rows: auto auto; column-gap: 8px; width: 100%; text-align: left; background: transparent; border: 1px solid transparent; padding: 6px 8px; border-radius: 6px; }
  .item:hover { background: var(--panel-2); }
  .item.sel { background: color-mix(in srgb, var(--accent) 16%, transparent); border-color: color-mix(in srgb, var(--accent) 35%, transparent); }
  .item .dot { grid-row: 1 / 3; align-self: center; }
  .name { font-weight: 600; overflow: hidden; text-overflow: ellipsis; }
  .ep { grid-column: 2 / 4; color: var(--muted); font-family: var(--mono); font-size: 11.5px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  .dot { width: 8px; height: 8px; border-radius: 50%; background: var(--line); }
  .dot.none { background: transparent; }
  .dot.ok { background: var(--ok); } .dot.bad { background: var(--bad); } .dot.warn { background: var(--warn); }
  .dot.run { background: var(--accent); animation: pulse 1s infinite; }
  @keyframes pulse { 50% { opacity: .3; } }
  .empty { color: var(--faint); padding: 12px; }
  .foot { border-top: 1px solid var(--line); padding: 6px 10px; color: var(--faint); font-size: 11px; font-family: var(--mono); overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
  main { flex: 1; overflow: auto; min-width: 0; }
  .warnline { margin: 12px 20px 0; padding: 8px 10px; border-radius: 6px; color: var(--warn); background: color-mix(in srgb, var(--warn) 9%, transparent); }
  .placeholder { color: var(--faint); padding: 40px 20px; }
  .scrim { position: fixed; inset: 0; background: rgb(0 0 0 / .45); display: grid; place-items: center; z-index: 10; }
  .sync { display: inline-flex; align-items: center; gap: 6px; }
  .dialog.wide { width: min(820px, calc(100vw - 32px)); }
  .dialog { width: min(560px, calc(100vw - 32px)); max-height: calc(100vh - 60px); overflow: auto; background: var(--panel); border: 1px solid var(--line); border-radius: 10px; box-shadow: 0 20px 60px rgb(0 0 0 / .4); }
  .toast { position: fixed; bottom: 16px; left: 50%; transform: translateX(-50%); background: var(--panel-2); border: 1px solid var(--line); padding: 8px 14px; border-radius: 8px; box-shadow: 0 8px 30px rgb(0 0 0 / .3); z-index: 20; max-width: 70vw; }
  .toast.bad { color: var(--bad); border-color: color-mix(in srgb, var(--bad) 45%, transparent); }
</style>
