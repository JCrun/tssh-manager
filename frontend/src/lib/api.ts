import * as Go from '../../wailsjs/go/main/App'
import { main, sshconf, settings } from '../../wailsjs/go/models'
import { EventsOn } from '../../wailsjs/runtime/runtime'

export type Host = main.HostView
export type State = main.State
export type KeyInfo = main.KeyInfo
export type Settings = settings.Settings
export type Change = sshconf.Change

export interface CheckResult {
  alias: string
  target: string
  tcp: 'ok' | 'fail' | 'skipped'
  tcpMs: number
  login: 'ok' | 'fail' | 'skipped'
  loginMs: number
  problem: string
  message: string
  finished: number
}

export const api = Go
export { EventsOn }

export function errText(e: unknown): string {
  if (e instanceof Error) return e.message
  return String(e)
}

export const problemText: Record<string, string> = {
  dns: '域名解析失败',
  refused: '端口拒绝连接',
  timeout: '超时',
  unreachable: '网络不可达',
  hostkey: '主机密钥不匹配或未知',
  auth: '认证失败',
  closed: '连接被关闭',
  config: '配置无法解析',
  other: '其他错误',
}

export function authLabel(h: Host): { text: string; tone: 'ok' | 'warn' | 'muted' } {
  if (h.plainPassword) return { text: h.auth === 'key+password' ? '密钥 + 明文密码' : '明文密码', tone: 'warn' }
  if (h.encPassword) return { text: h.auth === 'key+password' ? '密钥 + 弱加密密码' : '弱加密密码', tone: 'warn' }
  if (h.auth === 'key') return { text: '密钥', tone: 'ok' }
  return { text: '默认', tone: 'muted' }
}

export function endpoint(h: Host): string {
  const e = h.effective || {}
  const host = h.hostName || e.HostName || h.alias
  const port = h.port || e.Port || '22'
  const user = h.user || e.User || ''
  return `${user ? user + '@' : ''}${host}${port === '22' ? '' : ':' + port}`
}
