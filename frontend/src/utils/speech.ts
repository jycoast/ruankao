// 从浏览器提供的音色列表中挑一个英语音色：优先美音，其次任意英语音色。
// 抽成纯函数便于在 node 环境下单测，不触碰 window。
export function pickEnglishVoice(
  voices: readonly SpeechSynthesisVoice[],
): SpeechSynthesisVoice | null {
  const langOf = (v: SpeechSynthesisVoice) => v.lang.toLowerCase()
  return (
    voices.find((v) => langOf(v) === 'en-us') ??
    voices.find((v) => langOf(v).startsWith('en')) ??
    null
  )
}
