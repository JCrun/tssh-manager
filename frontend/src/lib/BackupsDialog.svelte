<script lang="ts">
  import { onMount } from 'svelte'
  import { api, errText } from './api'
  import type { main } from '../../wailsjs/go/models'

  let { onclose, onrestored }: { onclose: () => void; onrestored: (msg: string) => void } = $props()
  let list = $state<main.Backup[]>([])
  let error = $state('')
  let confirm = $state('')

  onMount(async () => { try { list = (await api.ListBackups()) ?? [] } catch (e) { error = errText(e) } })

  async function restore(p: string) {
    try { await api.RestoreBackup(p); onrestored('已恢复备份（恢复前的配置也已备份）') } catch (e) { error = errText(e) }
  }
</script>

<div class="body">
  <h2>配置备份</h2>
  <p class="faint">每次保存前自动备份，保留最近 30 份。</p>
  {#if list.length === 0}<p class="faint">还没有备份。</p>{/if}
  <ul>
    {#each list as b}
      <li>
        <span>{new Date(b.time).toLocaleString()}</span>
        <span class="faint">{(b.size / 1024).toFixed(1)} KB</span>
        {#if confirm === b.path}
          <button class="danger" onclick={() => restore(b.path)}>确认恢复</button>
          <button onclick={() => (confirm = '')}>取消</button>
        {:else}
          <button onclick={() => (confirm = b.path)}>恢复</button>
        {/if}
      </li>
    {/each}
  </ul>
  {#if error}<p class="err">{error}</p>{/if}
  <div class="actions"><button onclick={onclose}>关闭</button></div>
</div>

<style>
  .body { padding: 18px 20px; }
  h2 { margin: 0 0 6px; font-size: 16px; }
  .faint { color: var(--faint); margin: 0 0 10px; }
  ul { list-style: none; margin: 0; padding: 0; max-height: 50vh; overflow: auto; }
  li { display: flex; gap: 10px; align-items: center; padding: 6px 0; border-bottom: 1px solid var(--line); }
  li span:first-child { flex: 1; }
  .err { color: var(--bad); }
  .actions { display: flex; justify-content: flex-end; margin-top: 14px; }
</style>
