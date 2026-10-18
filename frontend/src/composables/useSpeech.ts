// Web Speech API 薄封装：只负责朗读给定文本，不自动播放、不排队。
// 不支持的环境（老浏览器/隐私模式）由 isSupported 让调用方隐藏入口。
import { pickEnglishVoice } from '@/utils/speech'

export function useSpeech() {
  const isSupported = typeof window !== 'undefined' && 'speechSynthesis' in window

  function speak(text: string): void {
    if (!isSupported || !text) return
    // 连点喇叭时不排队：先停掉正在播的，再读最后一次点击的词
    window.speechSynthesis.cancel()
    const utterance = new SpeechSynthesisUtterance(text)
    utterance.lang = 'en-US'
    utterance.rate = 0.9 // 略慢，便于跟读
    // 每次现取而不是缓存：音色列表在浏览器里是异步加载的，首次可能拿到空数组；
    // 拿不到英语音色也不影响，只设 lang 时浏览器会按语言选默认音色。
    const voice = pickEnglishVoice(window.speechSynthesis.getVoices())
    if (voice) utterance.voice = voice
    window.speechSynthesis.speak(utterance)
  }

  return { isSupported, speak }
}
