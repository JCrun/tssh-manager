<script lang="ts">
  import { api, errText, type Host, type Change } from './api'

  let { host = null, sections = [], onsaved, oncancel }: {
    host?: Host | null
    sections?: string[]
    onsaved: (alias: string, backup: string) => void
    oncancel: () => void
  } = $props()

  // svelte-ignore state_referenced_locally
  const isNew = !host
  // svelte-ignore state_referenced_locally
  const orig = {
    alias: host?.alias ?? '',
    HostName: host?.hostName ?? '',
    Port: host?.port ?? '',
    User: host?.user ?? '',
    IdentityFile: host?.identityFile ?? '',
    IdentitiesOnly: host?.identitiesOnly ?? false,
    ProxyJump: host?.proxyJump ?? '',
    GroupLabels: (host?.groups ?? []).join(' '),
  }
  let f = $state({ ...orig })
  // svelte-ignore state_referenced_locally
  let section = $state(sections[sections.length - 1] ?? '')
  let newSection = $state('')
  let busy = $state(false)
  let error = $state('')

  const aliasOk = $derived(/^[A-Za-z0-9._@-]+$/.test(f.alias))
  const portOk = $derived(f.Port === '' || (/^\d+$/.test(f.Port) && +f.Port > 0 && +f.Port < 65536))
  const canSave = $derived(aliasOk && portOk && (!isNew || f.HostName.trim() !== '') && !busy)

  function changes(): Change[] {
    const out: Change[] = []
    for (const k of ['HostName', 'Port', 'User', 'IdentityFile', 'ProxyJump'] as const) {
      const v = f[k].trim()
      if (isNew ? v !== '' : v !== orig[k]) out.push({ key: k, value: v, ext: false })
    }
    if (f.IdentitiesOnly !== orig.IdentitiesOnly || (isNew && f.IdentitiesOnly))
      out.push({ key: 'IdentitiesOnly', value: f.IdentitiesOnly ? 'yes' : '', ext: false })
    const g = f.GroupLabels.trim().split(/\s+/).filter(Boolean).join(' ')
    if (isNew ? g !== '' : g !== orig.GroupLabels) out.push({ key: 'GroupLabels', value: g, ext: true })
    return out
  }

  async function save() {
    busy = true
    error = ''
    try {
      const sec = section === '__new__' ? newSection.trim() : section
      const backup = await api.SaveHost({ origAlias: orig.alias, alias: f.alias.trim(), section: isNew ? sec : '', changes: changes() } as any)
      onsaved(f.alias.trim(), backup)
    } catch (e) {
      error = errText(e)
    } finally {
      busy = false
    }
  }
</script>

<form class="form" onsubmit={(e) => { e.preventDefault(); if (canSave) save() }}>
  <h2>{isNew ? '新建主机' : `编辑 ${orig.alias}`}</h2>
  <div class="grid">
    <div class="wide">
      <label for="f-alias">别名（Host）</label>
      <input id="f-alias" bind:value={f.alias} placeholder="例如 VPS-LD-9929" class:bad={f.alias !== '' && !aliasOk} autocomplete="off" />
      {#if f.alias !== '' && !aliasOk}<small class="err">只能用字母、数字和 . _ @ -，不能有空格或通配符</small>{/if}
    </div>
    {#if isNew}
      <div class="wide">
        <label for="f-sec">分组（配置里的 # 注释标题）</label>
        <select id="f-sec" bind:value={section}>
          {#each sections as s}<option value={s}>{s}</option>{/each}
          <option value="">（不分组，放到文件末尾）</option>
          <option value="__new__">新建分组…</option>
        </select>
        {#if section === '__new__'}<input class="mt" bind:value={newSection} placeholder="分组名，例如 London" />{/if}
      </div>
    {/if}
    <div class="wide">
      <label for="f-hn">地址（HostName）</label>
      <input id="f-hn" bind:value={f.HostName} placeholder="IP 或域名" autocomplete="off" />
    </div>
    <div>
      <label for="f-port">端口</label>
      <input id="f-port" bind:value={f.Port} placeholder="22" class:bad={!portOk} inputmode="numeric" />
    </div>
    <div>
      <label for="f-user">用户</label>
      <input id="f-user" bind:value={f.User} placeholder="root" autocomplete="off" />
    </div>
    <div class="wide">
      <label for="f-key">私钥（IdentityFile）</label>
      <input id="f-key" bind:value={f.IdentityFile} placeholder="~/.ssh/tabby-keys/xxx_ed25519" class="mono" />
      <label class="check"><input type="checkbox" bind:checked={f.IdentitiesOnly} /> 只用这把私钥（IdentitiesOnly yes）</label>
    </div>
    <div class="wide">
      <label for="f-pj">跳板（ProxyJump，可空）</label>
      <input id="f-pj" bind:value={f.ProxyJump} placeholder="例如 Tencent-NJ" />
    </div>
    <div class="wide">
      <label for="f-gl">标签（#!! GroupLabels，空格分隔，tssh 菜单里可筛选）</label>
      <input id="f-gl" bind:value={f.GroupLabels} placeholder="例如 美国 落地" />
    </div>
  </div>
  {#if !isNew && (host?.plainPassword || host?.encPassword)}
    <p class="note">这台主机存有{host?.plainPassword ? '明文' : '加密'}密码。表单不显示也不修改密码；可以在详情页「改用密钥」或删除它。</p>
  {/if}
  {#if error}<p class="err box">{error}</p>{/if}
  <div class="actions">
    <button type="button" onclick={oncancel} disabled={busy}>取消</button>
    <button type="submit" class="primary" disabled={!canSave}>{busy ? '保存中…' : '保存'}</button>
  </div>
</form>

<style>
  .form { padding: 18px 20px; max-width: 640px; }
  h2 { margin: 0 0 14px; font-size: 16px; }
  .grid { display: grid; grid-template-columns: 1fr 1fr; gap: 12px 14px; }
  .wide { grid-column: 1 / -1; }
  .check { display: flex; gap: 6px; align-items: center; margin-top: 6px; color: var(--muted); }
  .check input { width: auto; }
  .mt { margin-top: 6px; }
  input.bad { border-color: var(--bad); }
  .err { color: var(--bad); }
  .box { background: color-mix(in srgb, var(--bad) 10%, transparent); padding: 8px 10px; border-radius: 6px; white-space: pre-wrap; }
  small { display: block; margin-top: 4px; }
  .note { color: var(--muted); font-size: 12.5px; }
  .actions { display: flex; justify-content: flex-end; gap: 8px; margin-top: 16px; }
</style>
