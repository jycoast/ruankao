import { describe, expect, it } from 'vitest'
import { resolveApiBaseUrl } from './apiBaseUrl'

describe('resolveApiBaseUrl', () => {
  it('构建期缺 VITE_API_BASE_URL 直接抛错，不静默兜底', () => {
    expect(() => resolveApiBaseUrl('build', undefined)).toThrow(/VITE_API_BASE_URL/)
  })

  it('空串与纯空白同样视为缺失', () => {
    expect(() => resolveApiBaseUrl('build', '')).toThrow(/VITE_API_BASE_URL/)
    expect(() => resolveApiBaseUrl('build', '   ')).toThrow(/VITE_API_BASE_URL/)
  })

  it('构建期提供了值就原样返回 —— 相对与绝对都合法', () => {
    expect(resolveApiBaseUrl('build', '/api')).toBe('/api')
    expect(resolveApiBaseUrl('build', 'https://ruankao.duckdns.org/api')).toBe(
      'https://ruankao.duckdns.org/api',
    )
  })

  it('dev 期缺失回退 /api —— 由 vite 代理转发到后端', () => {
    expect(resolveApiBaseUrl('serve', undefined)).toBe('/api')
  })

  it('dev 期给了绝对地址就返回它，代理据此推导 target', () => {
    expect(resolveApiBaseUrl('serve', 'https://api.example.com/api')).toBe(
      'https://api.example.com/api',
    )
  })
})
