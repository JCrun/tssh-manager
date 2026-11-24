<script lang="ts">
  import { onMount } from 'svelte'
  import { api, errText, type Settings } from './api'

  let { onclose, onsaved }: { onclose: () => void; onsaved: () => void } = $props()
  let s = $state<Settings | null>(null)
  let error = $state('')
  let busy = $state(false)
  let version = $state('')

  onMount(async () => { s = await api.GetSettings(); version = await api.Version() })

  let presets = $state<string[]>([])
  onMount(async () => { presets = (await api.TerminalPresets()) ?? [] })
  const presetName = (p: string) => { const w = p.split(' '); return w[0] === 'open' ? w[2] : w[0] === '@terminal-app' ? 'Terminal.app' : w[0] }

  async function save() {
    busy = true; error = ''
    try { await api.SaveSettings(s!); onsaved() } catch (e) { error = errText(e) } finally { busy = false }
  }
</script>

<div class="body">
  <h2>设置</h2>
  {#if s}
    <label for="s-term">终端命令（<code>{'{host}'}</code> 会替换成别名，不经过 shell）</label>
    <input id="s-term" class="mono" bind:value={s.terminal} />
    <div class="presets">{#each presets as p}<button class="ghost small" title={p} onclick={() => (s!.terminal = p)}>{presetName(p)}</button>{/each}</div>
    <label for="s-tssh">tssh 程序</label>
    <input id="s-tssh" class="mono" bind:value={s.tssh} />
    <label for="s-cfg">SSH 配置文件</label>
    <input id="s-cfg" class="mono" bind:value={s.configPath} />
    <label for="s-key">新密钥存放目录</label>
    <input id="s-key" class="mono" bind:value={s.keyDir} />
  {/if}
  {#if error}<p class="err">{error}</p>{/if}
  <div class="actions">
    <span class="ver">tssh-manager {version}</span>
    <button onclick={onclose}>取消</button>
    <button class="primary" onclick={save} disabled={!s || busy}>保存</button>
  </div>
</div>

<style>
  .body { padding: 18px 20px; display: grid; gap: 4px; }
  h2 { margin: 0 0 10px; font-size: 16px; }
  label { margin-top: 10px; }
  .presets { display: flex; gap: 4px; flex-wrap: wrap; margin-top: 4px; }
  .small { font-size: 12px; padding: 2px 8px; color: var(--muted); }
  .err { color: var(--bad); }
  .actions { display: flex; justify-content: flex-end; align-items: center; gap: 8px; margin-top: 16px; }
  .ver { margin-right: auto; color: var(--faint); font-size: 12px; }
</style>
