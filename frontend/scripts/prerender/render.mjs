// 预渲染静态页的纯渲染函数。无 I/O、无网络，便于 node --test 直接断言。
import { Marked } from 'marked'

const SITE_NAME = '软考星图'

export function esc(s) {
  return String(s ?? '')
    .replace(/&/g, '&amp;')
    .replace(/</g, '&lt;')
    .replace(/>/g, '&gt;')
    .replace(/"/g, '&quot;')
}

// 渲染器配置与 App 内 utils/markdown.ts 对齐（gfm + breaks），
// 让静态页与 SPA 的呈现一致 —— 爬虫和用户看到同一份内容。
//
// 关键：内容（知识点正文、用户投稿的范文）可能含原始 HTML，而 marked 默认放行它。
// 前端在渲染出口用 DOMPurify 兜底（见 utils/markdown.ts:10-11 的注释），静态页没有
// 这一层，不处理就等于把可注入的 HTML 烧进给所有人看的页面 —— 存储型 XSS。
// 这里覆盖 html 渲染器：任何「原始 HTML」令牌统一转义成纯文本。
//
// 两个实测得到的坑（marked@18.0.5，改前务必重跑验证）：
//   1. 必须用 new Marked() + use() 合并。写成 marked.parse(md, { renderer }) 会把整个
//      renderer 替换成只含 html 的对象，导致 paragraph/heading 全部
//      "this.renderer.xxx is not a function"。
//   2. 覆盖 html 不影响代码块与正文里的裸 `<`：`<<include>>`、`O(1)<O(logn)` 走的是
//      codespan / text 渲染器，不经过 html。生产库中含 `<` 的内容全是这两类，故无损。
const md = new Marked({ gfm: true, breaks: true })
md.use({ renderer: { html: (token) => esc(token.text) } })

// 路由路径 → dist 下的文件路径。Cloudflare Pages 会把 foo.html 以 /foo 提供。
export function pageToFile(path) {
  return path.replace(/^\//, '') + '.html'
}

// 内联样式：静态内容页不依赖任何外部资源，无 JS 也能立即完整渲染。
const STYLE = `
:root{color-scheme:light dark}
*{box-sizing:border-box}
body{margin:0;font:16px/1.75 -apple-system,BlinkMacSystemFont,"Segoe UI","PingFang SC","Hiragino Sans GB","Microsoft YaHei",sans-serif;color:#0f172a;background:#fff}
a{color:#2563eb}
.site-head{display:flex;align-items:center;justify-content:space-between;gap:16px;padding:14px 20px;border-bottom:1px solid #e2e8f0;position:sticky;top:0;background:inherit}
.brand{font-weight:700;font-size:17px;text-decoration:none;color:inherit}
.site-head nav a{margin-left:16px;font-size:14px;text-decoration:none}
main{max-width:760px;margin:0 auto;padding:28px 20px 64px}
h1{font-size:26px;line-height:1.4;margin:0 0 20px}
h2{font-size:20px;margin:32px 0 12px}
h3{font-size:17px;margin:24px 0 10px}
img{max-width:100%;height:auto}
code{background:#f1f5f9;padding:2px 5px;border-radius:4px;font-size:14px}
pre{background:#f1f5f9;padding:14px;border-radius:8px;overflow:auto}
blockquote{margin:16px 0;padding-left:14px;border-left:3px solid #cbd5e1;color:#475569}
table{border-collapse:collapse;width:100%}
th,td{border:1px solid #e2e8f0;padding:8px 10px;text-align:left}
.cta{margin-top:48px;padding:20px;border:1px solid #e2e8f0;border-radius:12px;background:#f8fafc}
.cta p{margin:0 0 12px}
.btn{display:inline-block;padding:10px 18px;border-radius:8px;background:#2563eb;color:#fff;text-decoration:none;font-weight:600}
.site-foot{max-width:760px;margin:0 auto;padding:0 20px 48px;color:#64748b;font-size:14px}
.group{margin:32px 0 0}
.group h2{font-size:18px;border-bottom:1px solid #e2e8f0;padding-bottom:8px}
.group ul{list-style:none;padding:0;margin:12px 0;display:grid;grid-template-columns:repeat(auto-fill,minmax(220px,1fr));gap:8px 20px}
.group li{font-size:15px}
@media (prefers-color-scheme:dark){
  body{color:#e2e8f0;background:#0f172a}
  .site-head{border-color:#1e293b}
  a{color:#60a5fa}
  .cta{background:#1e293b;border-color:#334155}
  .btn{background:#2563eb}
  code,pre{background:#1e293b}
  blockquote{border-color:#334155;color:#94a3b8}
  th,td{border-color:#334155}
  .group h2{border-color:#1e293b}
  .site-foot{color:#94a3b8}
}
`

function head({ site, path, title, description, ogType }) {
  const canonical = site + path
  return `<meta charset="UTF-8">
<meta name="viewport" content="width=device-width, initial-scale=1.0">
<title>${esc(title)}</title>
<meta name="description" content="${esc(description)}">
<link rel="canonical" href="${esc(canonical)}">
<meta property="og:type" content="${esc(ogType)}">
<meta property="og:title" content="${esc(title)}">
<meta property="og:description" content="${esc(description)}">
<meta property="og:url" content="${esc(canonical)}">
<link rel="icon" type="image/png" href="/favicon.png">
<style>${STYLE}</style>`
}

const HEADER = `<header class="site-head">
<a class="brand" href="/">${SITE_NAME}</a>
<nav><a href="/knowledge">知识导图</a><a href="/essays">论文范文</a></nav>
</header>`

const FOOTER = `<footer class="site-foot"><p>${SITE_NAME} — 软考知识导图与论文范文</p></footer>`

// 详情页：自包含内容页，不挂 SPA。爬虫与用户拿到逐字节相同的 HTML。
export function renderDetailPage(page, site) {
  const bodyHtml = md.parse(page.body || '')
  return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
${head({ site, path: page.path, title: page.title, description: page.description, ogType: 'article' })}
</head>
<body>
${HEADER}
<main>
<h1>${esc(page.h1)}</h1>
<article>${bodyHtml}</article>
<aside class="cta">
<p>登录${SITE_NAME}，把这些知识点关联到真题练习。</p>
<a class="btn" href="/login?redirect=${encodeURIComponent(page.path)}">进入${SITE_NAME}</a>
</aside>
</main>
${FOOTER}
</body>
</html>`
}

// hub 页：跨科目全量索引。SPA 的 /knowledge 按本地 subject_id 过滤，
// 静态 hub 对所有人都是同一份，爬虫与用户看到的内容完全一致。
export function renderHub({ type, groups }, site) {
  const isKnowledge = type === 'knowledge'
  const path = isKnowledge ? '/knowledge' : '/essays'
  const title = isKnowledge ? `软考知识点大全 - ${SITE_NAME}` : `软考论文范文大全 - ${SITE_NAME}`
  const description = isKnowledge
    ? '按科目整理的全部软考知识点，覆盖系统架构设计师、软件设计师等。'
    : '软考论文范文合集，含历年真题论文与高分范文。'
  const h1 = isKnowledge ? '软考知识点' : '软考论文范文'

  const sections = groups
    .map(
      (g) => `<section class="group">
<h2>${esc(g.subjectName)}</h2>
<ul>${g.pages.map((p) => `<li><a href="${esc(p.path)}">${esc(p.h1)}</a></li>`).join('')}</ul>
</section>`,
    )
    .join('\n')

  return `<!DOCTYPE html>
<html lang="zh-CN">
<head>
${head({ site, path, title, description, ogType: 'website' })}
</head>
<body>
${HEADER}
<main>
<h1>${esc(h1)}</h1>
${sections}
</main>
${FOOTER}
</body>
</html>`
}

export function renderSitemap(pages, site) {
  const urls = pages
    .map(
      (p) =>
        `  <url>\n    <loc>${esc(site + p.path)}</loc>\n` +
        (p.updatedAt ? `    <lastmod>${esc(p.updatedAt)}</lastmod>\n` : '') +
        `  </url>`,
    )
    .join('\n')
  return `<?xml version="1.0" encoding="UTF-8"?>
<urlset xmlns="http://www.sitemaps.org/schemas/sitemap/0.9">
${urls}
</urlset>
`
}

// 只放行公开内容；做题、后台、个人页一律 Disallow —— 这些对未登录爬虫
// 只会渲染成登录跳转，收录了反而稀释权重。
export function renderRobots(site) {
  const disallow = [
    '/api/', '/admin', '/practice', '/questions', '/exam', '/exams',
    '/daily', '/wrong', '/review', '/favorites', '/report',
    '/profile', '/membership', '/login', '/register', '/oauth',
  ]
  return (
    'User-agent: *\n' +
    'Allow: /knowledge\n' +
    'Allow: /essays\n' +
    disallow.map((p) => `Disallow: ${p}`).join('\n') +
    `\n\nSitemap: ${site}/sitemap.xml\n`
  )
}
