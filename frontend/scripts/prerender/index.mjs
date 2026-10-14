// 构建期预渲染：拉 manifest → 渲染静态页 → 写进 dist/。
// fail-soft：任何一步失败都只告警并退出 0，绝不让部署挂掉。
// 静态页缺失时该 URL 会落回 SPA fallback（仍可用，只是收录效果打折）。
import { mkdirSync, writeFileSync, existsSync } from 'node:fs'
import { dirname, join } from 'node:path'
import { fileURLToPath } from 'node:url'
import { loadEnv } from 'vite'
import { pageToFile, renderDetailPage, renderHub, renderSitemap, renderRobots } from './render.mjs'

const ROOT = join(dirname(fileURLToPath(import.meta.url)), '..', '..')
const DIST = join(ROOT, 'dist')

// VITE_API_BASE_URL 只在 .env.<mode> 里，vite build 自己读了用于打包，
// 但**不会**把它导出到 npm script 的环境变量里 —— 直接读 process.env 永远是空，
// 结果就是每次构建都静默跳过预渲染（实测）。所以这里用 vite 的 loadEnv 自己读一遍；
// 构建脚本把 mode 作为第一个参数传进来（见 package.json）。
// PRERENDER_API_BASE 留给临时覆盖（本地指向别的后端）。
const MODE = process.argv[2] || 'production'
const FILE_ENV = loadEnv(MODE, ROOT, '')
const API = (process.env.PRERENDER_API_BASE || FILE_ENV.VITE_API_BASE_URL || '').replace(/\/$/, '')
// 站点基址兜底（api 未返回 site 时用）。开源版不自带站点，需部署方自配：
// 可用 PRERENDER_SITE 环境变量或 .env.<mode> 里的 VITE_SITE_URL 提供，如 https://example.com
const FALLBACK_SITE = (process.env.PRERENDER_SITE || FILE_ENV.VITE_SITE_URL || '').replace(/\/$/, '')

function warn(msg) {
  console.warn(`[prerender] ${msg}`)
}

async function getJson(url) {
  const res = await fetch(url, { headers: { accept: 'application/json' } })
  if (!res.ok) throw new Error(`${url} → HTTP ${res.status}`)
  return res.json()
}

// 翻页拉取，直到某页返回空数组。total 只用于进度显示。
// 响应经 response.Success 包了一层，真正的载荷在 data 里。
async function fetchManifest() {
  const all = []
  let site = ''
  let total = 0
  for (let page = 1; ; page++) {
    const res = await getJson(`${API}/prerender/manifest?page=${page}&page_size=200`)
    const data = res.data || {}
    site = data.site || site
    total = data.total || total
    const batch = data.pages || []
    if (batch.length === 0) break
    all.push(...batch)
    if (total && all.length >= total) break
    if (page > 50) {
      warn('翻页超过 50 页，疑似接口异常，提前停止')
      break
    }
  }
  return { site, total, pages: all }
}

// /api/subjects 返回 {code,message,data:[{id,name,...}]}，id 是数字，
// 与 manifest 里的 subjectId 同源（都是 subjects.id），可直接 join。
async function fetchSubjects() {
  try {
    const res = await getJson(`${API}/subjects`)
    // getJson 只看 HTTP 状态，而本仓库的业务失败恒返回 HTTP 200（response.Error）。
    // 不判 code 的话，/api/subjects 被维护中间件挡下的 {code:503} 会走到下面
    // `res.data || []` → 空 map 且**不留任何痕迹**，hub 页把全部条目归进「其它」
    // 照常发布。维护期间发版正是这个功能的头号用例，所以这里必须显式报警。
    // 与 manifest 同一定级：仍是 fail-soft，构建继续、hub 页照发，只是不再静默。
    if (res.code !== 0) throw new Error(`/subjects → code ${res.code}: ${res.message || ''}`)
    return new Map((res.data || []).map((s) => [s.id, s.name]))
  } catch (e) {
    warn(`取科目失败，hub 页将不分组：${e.message}`)
    return new Map()
  }
}

// 按 type 聚合出 hub 页的数据，按科目分组；没有科目名的归入「其它」。
function buildGroups(pages, subjects, type) {
  const bySubject = new Map()
  for (const p of pages.filter((p) => p.type === type)) {
    const name = subjects.get(p.subjectId) || '其它'
    if (!bySubject.has(name)) bySubject.set(name, [])
    bySubject.get(name).push({ path: p.path, h1: p.h1 })
  }
  return [...bySubject.entries()]
    .map(([subjectName, items]) => ({ subjectName, pages: items }))
    .sort((a, b) => b.pages.length - a.pages.length)
}

function write(relPath, content) {
  const full = join(DIST, relPath)
  mkdirSync(dirname(full), { recursive: true })
  writeFileSync(full, content, 'utf8')
}

async function main() {
  if (!existsSync(DIST)) {
    warn(`dist/ 不存在，跳过（先跑 vite build）`)
    return
  }
  if (!API) {
    warn('未取到 API 基址（PRERENDER_API_BASE / VITE_API_BASE_URL 均为空），跳过预渲染')
    return
  }

  const { site: apiSite, pages } = await fetchManifest()
  const site = (apiSite || FALLBACK_SITE).replace(/\/$/, '')
  if (!site) {
    warn('未取到站点基址（manifest 未返回且 PRERENDER_SITE / VITE_SITE_URL 为空），跳过预渲染')
    return
  }
  if (!pages.length) {
    warn('manifest 返回 0 页，跳过预渲染')
    return
  }
  const subjects = await fetchSubjects()

  for (const p of pages) {
    write(pageToFile(p.path), renderDetailPage(p, site))
  }

  const hubs = [
    { path: '/knowledge', type: 'knowledge' },
    { path: '/essays', type: 'essay' },
  ]
  for (const hub of hubs) {
    const groups = buildGroups(pages, subjects, hub.type)
    write(pageToFile(hub.path), renderHub({ type: hub.type, groups }, site))
  }

  const sitemapEntries = [...pages, ...hubs.map((h) => ({ path: h.path, updatedAt: '' }))]
  write('sitemap.xml', renderSitemap(sitemapEntries, site))
  write('robots.txt', renderRobots(site))

  const byType = (t) => pages.filter((p) => p.type === t).length
  console.log(
    `[prerender] ${pages.length} 页已生成` +
      `（知识点 ${byType('knowledge')} / 范文 ${byType('essay')}）+ ${hubs.length} hub + sitemap + robots`,
  )
}

main().catch((e) => {
  // fail-soft：预渲染只是增强，不能因为它让整个部署失败
  warn(`预渲染失败，已跳过，站点将以纯 SPA 形式发布：${e.message}`)
})
