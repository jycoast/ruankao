import { describe, expect, it } from 'vitest'
import { pickEnglishVoice } from '@/utils/speech'

// node 环境没有 SpeechSynthesisVoice 构造器，用最小对象冒充即可
const voice = (lang: string) => ({ lang, name: lang }) as SpeechSynthesisVoice

describe('pickEnglishVoice', () => {
  it('有 en-US 时优先选 en-US（即使排在其他英语音色后面）', () => {
    const voices = [voice('zh-CN'), voice('en-GB'), voice('en-US')]
    expect(pickEnglishVoice(voices)?.lang).toBe('en-US')
  })

  it('没有 en-US 时回退到任意 en-* 音色', () => {
    const voices = [voice('zh-CN'), voice('en-GB')]
    expect(pickEnglishVoice(voices)?.lang).toBe('en-GB')
  })

  it('只有非英语音色时返回 null', () => {
    expect(pickEnglishVoice([voice('zh-CN'), voice('ja-JP')])).toBeNull()
  })

  it('空数组返回 null', () => {
    expect(pickEnglishVoice([])).toBeNull()
  })

  it('lang 大小写不敏感', () => {
    expect(pickEnglishVoice([voice('EN-us')])?.lang).toBe('EN-us')
    expect(pickEnglishVoice([voice('en-US'), voice('EN-gb')])?.lang).toBe('en-US')
  })

  it('不修改入参', () => {
    const voices = [voice('en-GB'), voice('en-US')]
    const before = voices.map((v) => v.lang)
    pickEnglishVoice(voices)
    expect(voices.map((v) => v.lang)).toEqual(before)
    expect(voices).toHaveLength(2)
  })
})
