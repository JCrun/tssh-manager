<script lang="ts">
  import { onDestroy } from 'svelte'
  import { api, errText, type Host } from './api'

  let { host, onchanged, notify }: {
    host: Host
    onchanged: (msg: string) => void
    notify: (msg: string, tone?: 'ok' | 'bad') => void
  } = $props()

  const HIDE_AFTER = 20
  let shown = $state<string | null>(null)
  let left = $state(0)
  let timer: number | undefined
  let editing = $state(false)
  let draft = $state('')
  let showDraft = $state(false)
  let confirmRemove = $state(false)
  let busy = $state(false)
  let error = $state('')

  function hide() { shown = null; left = 0; clearInterval(timer) }
  onDestroy(hide)

  async function run(fn: () => Promise<void>) {
    busy = true; error = ''
    try { await fn() } catch (e) { error = errText(e) } finally { busy = false }
  }
  const reveal = () => run(async () => {
    shown = await api.RevealPassword(host.alias)
    left = HIDE_AFTER
    clearInterval(timer)
    timer = window.setInterval(() => { if (--left <= 0) hide() }, 1000)
  })
  const copy = () => run(async () => {
    const s = await api.CopyPassword(host.alias)
    notify(`密码已复制，${s} 秒后自动清除剪贴板`)
  })
  function startEdit(prefill: boolean) {
    hide(); error = ''; confirmRemove = false; showDraft = false
    draft = ''
    editing = true
    if (prefill) run(async () => { draft = await api.RevealPassword(host.alias) })
  }
  const save = () => run(async () => {
    await api.SetPassword(host.alias, draft)
    editing = false; draft = ''
    onchanged(host.plainPassword ? '密码已更新' : '密码已保存')
  })
  const remove = () => run(async () => {
    await api.SetPassword(host.alias, '')
    confirmRemove = false
    onchanged('已删除明文密码')
  })
</script>

<section class="pw">
  <div class="head">
    <h3>密码</h3>
    {#if host.encPassword}<span class="badge warn" title="tssh 用公开的固定密钥加密，拿到配置就能解出">弱加密</span>{/if}
  </div>

  {#if editing}
    <div class="edit">
      <div class="inputrow">
        <input type={showDraft ? 'text' : 'password'} bind:value={draft} placeholder="新密码" autocomplete="new-password" spellcheck="false"
          onkeydown={(e) => { if (e.key === 'Enter' && draft) save(); if (e.key === 'Escape') editing = false }} />
        <button class="ghost" onclick={() => (showDraft = !showDraft)}>{showDraft ? '隐藏' : '显示'}</button>
      </div>
      <p class="faint">以明文写入 <code>#!! Password</code>，保存后会用 tssh 的解析方式读回核对。不能包含 <code>#</code>。</p>
      <div class="row">
        <button onclick={() => (editing = false)} disabled={busy}>取消</button>
        <button class="primary" onclick={save} disabled={busy || !draft}>保存</button>
      </div>
    </div>
  {:else if host.plainPassword}
    <div class="value">
      <code class="secret">{shown ?? '••••••••••'}</code>
      {#if shown !== null}<span class="faint">{left}s 后隐藏</span>{/if}
    </div>
    <div class="row">
      {#if shown === null}<button onclick={reveal} disabled={busy}>显示</button>{:else}<button onclick={hide}>隐藏</button>{/if}
      <button onclick={copy} disabled={busy}>复制</button>
      <button onclick={() => startEdit(true)} disabled={busy}>修改</button>
      {#if confirmRemove}
        <button class="danger" onclick={remove} disabled={busy}>确认删除</button>
        <button onclick={() => (confirmRemove = false)}>取消</button>
      {:else}
        <button class="ghost danger" onclick={() => (confirmRemove = true)} disabled={busy}>删除</button>
      {/if}
    </div>
  {:else if host.encPassword}
    <p class="faint">配置里是 <code>#!! encPassword</code>。tssh 用写在源码里的固定密钥加密，任何拿到配置文件的人都能解出，安全性和明文相同。</p>
  {:else}
    <div class="row"><span class="faint">没有保存密码。</span><button onclick={() => startEdit(false)}>设置密码</button></div>
  {/if}

  {#if error}<p class="err">{error}</p>{/if}
</section>

<style>
  .pw { border: 1px solid var(--line); border-radius: var(--radius); padding: 10px 12px; background: var(--panel); display: grid; gap: 8px; }
  .head { display: flex; gap: 8px; align-items: center; }
  h3 { margin: 0; font-size: 13px; color: var(--muted); font-weight: 600; }
  .value { display: flex; gap: 10px; align-items: center; }
  .secret { font-size: 14px; letter-spacing: .5px; user-select: text; word-break: break-all; }
  .row, .inputrow { display: flex; gap: 8px; align-items: center; flex-wrap: wrap; }
  .inputrow input { flex: 1; font-family: var(--mono); }
  .edit { display: grid; gap: 6px; }
  .faint { color: var(--faint); font-size: 12.5px; margin: 0; }
  .err { color: var(--bad); margin: 0; white-space: pre-wrap; }
</style>
