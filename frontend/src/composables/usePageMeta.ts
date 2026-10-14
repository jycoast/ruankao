// 详情页的动态 title / description。
// 路由 meta 里的 title 是静态的，知识点和范文的标题只有拿到数据才知道；
// 没有这一步，所有 /knowledge/:id 在搜索结果里都显示同一个名字。
//
// 只做首屏可见的那部分（title + description + canonical）。爬虫拿到的完整 HTML
// 由后端 /api/prerender/* 直出，这里是给分享卡片和浏览器标签用的。

const SITE_NAME = '软考星图'

function setMeta(selector: string, attr: 'name' | 'property', key: string, content: string) {
  let el = document.head.querySelector<HTMLMetaElement>(selector)
  if (!el) {
    el = document.createElement('meta')
    el.setAttribute(attr, key)
    document.head.appendChild(el)
  }
  el.content = content
}

function stripMarkdown(s: string) {
  return s
    .replace(/!?\[[^\]]*\]\([^)]*\)/g, '')
    .replace(/\[图片\]/g, '')
    .replace(/[#*_>`]/g, '')
    .replace(/\s+/g, ' ')
    .trim()
}

export function usePageMeta(title: string, body = '') {
  const full = `${title} - ${SITE_NAME}`
  document.title = full

  // 按 rune 截断，中文按字节切会留下半个字
  const plain = stripMarkdown(body)
  const desc = [...plain].slice(0, 150).join('') + (plain.length > 150 ? '…' : '')

  setMeta('meta[name="description"]', 'name', 'description', desc)
  setMeta('meta[property="og:title"]', 'property', 'og:title', full)
  setMeta('meta[property="og:description"]', 'property', 'og:description', desc)
  setMeta('meta[property="og:type"]', 'property', 'og:type', 'article')

  let link = document.head.querySelector<HTMLLinkElement>('link[rel="canonical"]')
  if (!link) {
    link = document.createElement('link')
    link.rel = 'canonical'
    document.head.appendChild(link)
  }
  link.href = window.location.origin + window.location.pathname
}
