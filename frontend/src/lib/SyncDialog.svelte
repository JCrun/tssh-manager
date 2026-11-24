<script lang="ts">
  import { onMount } from 'svelte'
  import { api, errText, syncSummary, type SyncResult, type SyncView } from './api'

  let { initial = null, onclose, onsynced }: {
    initial?: SyncResult | null
    onclose: () => void
    onsynced: (r: SyncResult) => void
  } = $props()

  let v = $state<SyncView | null>(null)
  let form = $state({ backend: '', folder: '', webdavURL: '', webdavUser: '', webdavPassword: '', passphrase: '', passphrase2: '', syncKeys: false, auto: true, device: '' })
  let result = $state<SyncResult | null>(null)
  let pick = $state<Record<string, 'local' | 'remote'>>({})
  let error = $state('')
  let busy = $state(false)
  let saved = $state(false)
  let confirmReset = $state(false)

  async function refresh() {
    v = await api.GetSync()
    form = { backend: v.backend, folder: v.folder, webdavURL: v.webdavURL, webdavUser: v.webdavUser, webdavPassword: '', passphrase: '', passphrase2: '', syncKeys: v.syncKeys, auto: v.backend ? v.auto : true, device: v.device }
  }
  onMount(() => { result = initial; refresh() })

  const conflicts = $derived(result?.status === 'conflict' ? result.conflicts ?? [] : [])
  const allPicked = $derived(conflicts.every((c) => pick[c.id]))

  async function run(fn: () => Promise<void>) {
    busy = true; error = ''
    try { await fn() } catch (e) { error = errText(e) } finally { busy = false }
  }

  const save = () => run(async () => {
    if (form.passphrase !== form.passphrase2) throw new Error('两次输入的同步口令不一致')
    if (form.backend && !v?.hasPassphrase && !form.passphrase) throw new Error('请设置同步口令')
    await api.SaveSync({ ...form })
    await refresh()
    saved = true
    setTimeout(() => (saved = false), 2000)
  })

  const syncNow = (resolve: Record<string, string> = {}) => run(async () => {
    const r = await api.SyncNow(resolve)
    result = r
    if (r.status !== 'conflict') pick = {}
    await refresh()
    onsynced(r)
  })

  const reset = () => run(async () => { await api.SyncReset(); confirmReset = false; result = null; await refresh() })
  const setAll = (side: 'local' | 'remote') => { for (const c of conflicts) pick[c.id] = side }
</script>

<div class="body">
  <h2>多端同步</h2>
  <p class="faint">
    配置（可选连同密钥目录）用你的同步口令端到端加密成一个文件 <code>tssh-manager.sync.age</code>，放在同步文件夹或 WebDAV 上，远端看不到内容。
    每台设备按主机逐个三方合并：不同设备改了不同主机会自动合并，同一台主机两边都改了才需要你选择。
  </p>

  {#if v?.backend}
    <div class="status">
      <span>本机 <b>{v.device}</b> · 上次同步 {v.lastSync ? new Date(v.lastSync).toLocaleString() : '从未'}</span>
      <button class="primary" onclick={() => syncNow()} disabled={busy}>{busy ? '同步中…' : '立即同步'}</button>
    </div>
  {/if}

  {#if result && result.status !== 'conflict'}
    <div class="result">
      <div>{syncSummary(result)}</div>
      {#each result.changes?.fromRemote ?? [] as n}<div class="mono">↓ {n}</div>{/each}
      {#each result.changes?.fromLocal ?? [] as n}<div class="mono">↑ {n}</div>{/each}
      {#each result.warnings ?? [] as w}<div class="warn">{w}</div>{/each}
    </div>
  {/if}

  {#if conflicts.length}
    <div class="conflicts">
      <div class="chead">
        <b>{conflicts.length} 处冲突</b>（本机和远端都改了同一项，还没有做任何修改）
        <span class="sp"></span>
        <button class="small" onclick={() => setAll('local')}>全部用本机</button>
        <button class="small" onclick={() => setAll('remote')}>全部用远端</button>
      </div>
      {#each conflicts as c (c.id)}
        <div class="conflict">
          <div class="cname">{c.kind === 'key' ? '密钥 ' : ''}{c.name}{#if c.note}<span class="badge warn">{c.note}</span>{/if}</div>
          <div class="sides">
            {#each [['local', '本机', c.local], ['remote', '远端' + (result?.remoteDevice ? `（${result.remoteDevice}）` : ''), c.remote]] as [side, label, text]}
              <button class="side" class:on={pick[c.id] === side} onclick={() => (pick[c.id] = side as 'local' | 'remote')}>
                <div class="slabel">{label}</div>
                <pre>{text || '（已删除）'}</pre>
              </button>
            {/each}
          </div>
        </div>
      {/each}
      <div class="actions"><button class="primary" disabled={!allPicked || busy} onclick={() => syncNow(pick)}>按选择合并并同步</button></div>
    </div>
  {/if}

  <h3>设置</h3>
  <label for="sy-b">同步方式</label>
  <select id="sy-b" bind:value={form.backend}>
    <option value="">关闭</option>
    <option value="folder">文件夹（Syncthing、iCloud、OneDrive、Dropbox、NAS 挂载…）</option>
    <option value="webdav">WebDAV（坚果云、Nextcloud、群晖…）</option>
  </select>

  {#if form.backend === 'folder'}
    <label for="sy-f">文件夹（每台设备填自己机器上的路径）</label>
    <input id="sy-f" class="mono" bind:value={form.folder} placeholder="~/Sync/tssh" />
  {:else if form.backend === 'webdav'}
    <label for="sy-u">WebDAV 目录地址</label>
    <input id="sy-u" class="mono" bind:value={form.webdavURL} placeholder="https://dav.jianguoyun.com/dav/tssh/" />
    <div class="two">
      <div><label for="sy-n">用户名</label><input id="sy-n" bind:value={form.webdavUser} /></div>
      <div><label for="sy-p">密码 / 应用密码</label><input id="sy-p" type="password" bind:value={form.webdavPassword} placeholder={v?.hasWebdavPassword ? '已保存，留空不变' : ''} /></div>
    </div>
  {/if}

  {#if form.backend}
    <div class="two">
      <div><label for="sy-k">同步口令（所有设备相同，至少 8 位）</label><input id="sy-k" type="password" bind:value={form.passphrase} placeholder={v?.hasPassphrase ? '已保存，留空不变' : ''} /></div>
      <div><label for="sy-k2">再输入一次</label><input id="sy-k2" type="password" bind:value={form.passphrase2} /></div>
    </div>
    <p class="hint">口令忘了无法找回远端内容；它和 WebDAV 密码以 0600 权限保存在本机配置目录（与 ~/.ssh/config 里的明文密码同等保护）。</p>
    <label for="sy-d">本机名称</label>
    <input id="sy-d" bind:value={form.device} />
    <label class="check"><input type="checkbox" bind:checked={form.auto} /> 自动同步：启动时和每次保存后</label>
    <label class="check"><input type="checkbox" bind:checked={form.syncKeys} /> 同时同步密钥目录 <code>{v?.keyDir}</code> 里的私钥</label>
    {#if form.syncKeys}<p class="hint warn">私钥会加密后上传。远端删除的密钥在本机不会直接删掉，而是移到备份目录。</p>{/if}
  {/if}

  {#if error}<p class="err">{error}</p>{/if}
  <div class="actions">
    {#if v?.backend}
      {#if confirmReset}
        <span class="faint small">下次同步时两边不同的主机都会列为冲突</span>
        <button class="danger" onclick={reset}>确认重置</button>
        <button onclick={() => (confirmReset = false)}>取消</button>
      {:else}
        <button class="ghost small" onclick={() => (confirmReset = true)} title="忘记上次同步的状态">重置同步状态</button>
      {/if}
    {/if}
    <span class="sp"></span>
    {#if saved}<span class="ok">已保存</span>{/if}
    <button onclick={onclose}>关闭</button>
    <button class="primary" onclick={save} disabled={busy}>保存设置</button>
  </div>
</div>

<style>
  .body { padding: 18px 20px; display: grid; gap: 4px; }
  h2 { margin: 0 0 4px; font-size: 16px; }
  h3 { margin: 14px 0 0; font-size: 13.5px; }
  .faint { color: var(--faint); margin: 0 0 6px; }
  label { margin-top: 8px; }
  .check { display: flex; gap: 8px; align-items: center; color: var(--text); font-size: 13px; }
  .check input { width: auto; }
  .two { display: grid; grid-template-columns: 1fr 1fr; gap: 10px; }
  .hint { color: var(--faint); font-size: 12px; margin: 4px 0 0; }
  .warn { color: var(--warn); }
  .ok { color: var(--ok); }
  .err { color: var(--bad); white-space: pre-wrap; }
  .status { display: flex; align-items: center; justify-content: space-between; gap: 10px; padding: 8px 10px; background: var(--panel-2); border-radius: 6px; }
  .result { padding: 6px 10px; border-left: 3px solid var(--ok); display: grid; gap: 2px; }
  .conflicts { border: 1px solid color-mix(in srgb, var(--warn) 45%, transparent); border-radius: 8px; padding: 10px; display: grid; gap: 10px; margin-top: 6px; }
  .chead { display: flex; align-items: center; gap: 6px; flex-wrap: wrap; }
  .cname { font-weight: 600; margin-bottom: 4px; display: flex; gap: 8px; align-items: center; }
  .sides { display: grid; grid-template-columns: 1fr 1fr; gap: 8px; }
  .side { text-align: left; padding: 6px 8px; background: var(--bg); white-space: normal; }
  .side.on { border-color: var(--accent); outline: 2px solid color-mix(in srgb, var(--accent) 35%, transparent); }
  .slabel { font-size: 11.5px; color: var(--muted); margin-bottom: 2px; }
  pre { margin: 0; font-family: var(--mono); font-size: 11.5px; white-space: pre-wrap; word-break: break-all; max-height: 180px; overflow: auto; }
  .actions { display: flex; align-items: center; gap: 8px; margin-top: 14px; }
  .sp { flex: 1; }
  .small { font-size: 12px; padding: 2px 8px; }
</style>
