import { test } from 'node:test'
import assert from 'node:assert/strict'
import {
  esc,
  pageToFile,
  renderDetailPage,
  renderHub,
  renderSitemap,
  renderRobots,
} from './render.mjs'

const SITE = 'https://example.com'

test('pageToFile 把路由路径映射成 dist 下的文件路径', () => {
  assert.equal(pageToFile('/knowledge/310'), 'knowledge/310.html')
  assert.equal(pageToFile('/essays/3'), 'essays/3.html')
})

test('esc 转义 HTML 特殊字符', () => {
  assert.equal(esc('a<b>&"c"'), 'a&lt;b&gt;&amp;&quot;c&quot;')
  assert.equal(esc(null), '')
})

test('详情页含 title/description/canonical/og 与正文', () => {
  const html = renderDetailPage(
    {
      path: '/knowledge/310',
      title: '开放-封闭原则 - 软考知识点 - 软考星图',
      description: '开放-封闭原则的说明',
      h1: '开放-封闭原则',
      body: '## 定义\n\n软件实体应该对扩展开放。',
    },
    SITE,
  )

  assert.ok(html.includes('<title>开放-封闭原则 - 软考知识点 - 软考星图</title>'))
  assert.ok(html.includes('content="开放-封闭原则的说明"'))
  assert.ok(html.includes(`<link rel="canonical" href="${SITE}/knowledge/310">`))
  assert.ok(html.includes('property="og:url"'))
  // markdown 必须被渲染成标签，而不是原样露出 ##
  assert.ok(html.includes('<h2'))
  assert.ok(!html.includes('## 定义'))
  // 不含应用壳标记，避免被 SW 误认成壳
  assert.ok(!html.includes('data-app-shell'))
})

test('详情页转义原始 HTML，防存储型 XSS', () => {
  const html = renderDetailPage(
    {
      path: '/knowledge/1',
      title: 't',
      description: 'd',
      h1: 'h',
      body: 'a <script>alert(1)</script> b\n\nsee <img src=x onerror=alert(1)> here',
    },
    SITE,
  )
  // marked 默认放行原始 HTML（前端靠 DOMPurify 兜底，静态页没有这一层），
  // 这里必须已经转义成文本，不能出现可执行的标签
  assert.ok(!html.includes('<script>'))
  assert.ok(!html.includes('onerror=alert(1)>'))
  assert.ok(html.includes('&lt;script&gt;'))
  assert.ok(html.includes('&lt;img src=x onerror=alert(1)&gt;'))
})

test('代码块与数学比较符不被 HTML 转义破坏', () => {
  // 生产库实测：含 `<` 的内容全是这类，转义不能把它们弄坏
  const html = renderDetailPage(
    {
      path: '/knowledge/188',
      title: 't',
      description: 'd',
      h1: 'h',
      body: '用例关系用 `<<include>>` 表示\n\n复杂度 O(1)<O(logn)<O(n)',
    },
    SITE,
  )
  // 行内代码里的 <<include>> 由 marked 正常转义后放进 <code>
  assert.ok(html.includes('<code>&lt;&lt;include&gt;&gt;</code>'))
  // 正文里的裸 < 同样是转义文本，不会变成标签
  assert.ok(html.includes('O(1)&lt;O(logn)&lt;O(n)'))
})

test('markdown 结构照常渲染', () => {
  const html = renderDetailPage(
    { path: '/knowledge/1', title: 't', description: 'd', h1: 'h', body: '## 定义\n\n**重点**\n\n| a | b |\n|---|---|\n| 1 | 2 |' },
    SITE,
  )
  assert.ok(html.includes('<h2>定义</h2>'))
  assert.ok(html.includes('<strong>重点</strong>'))
  assert.ok(html.includes('<table>'))
})

test('hub 页按科目分组列出链接', () => {
  const html = renderHub(
    {
      type: 'knowledge',
      groups: [
        {
          subjectName: '系统架构设计师',
          pages: [{ path: '/knowledge/310', h1: '开放-封闭原则' }],
        },
      ],
    },
    SITE,
  )
  assert.ok(html.includes('系统架构设计师'))
  assert.ok(html.includes('href="/knowledge/310"'))
  assert.ok(html.includes('开放-封闭原则'))
  assert.ok(html.includes(`<link rel="canonical" href="${SITE}/knowledge">`))
})

test('sitemap 是合法 XML 且用绝对 URL', () => {
  const xml = renderSitemap(
    [
      { path: '/knowledge/310', updatedAt: '2026-01-02' },
      { path: '/essays/3', updatedAt: '2026-02-03' },
    ],
    SITE,
  )
  assert.ok(xml.startsWith('<?xml version="1.0" encoding="UTF-8"?>'))
  assert.ok(xml.includes('<loc>https://example.com/knowledge/310</loc>'))
  assert.ok(xml.includes('<lastmod>2026-01-02</lastmod>'))
  assert.ok(xml.includes('xmlns="http://www.sitemaps.org/schemas/sitemap/0.9"'))
})

test('sitemap 不含需要登录的根路径', () => {
  const xml = renderSitemap([{ path: '/knowledge', updatedAt: '2026-01-01' }], SITE)
  assert.ok(!xml.includes('<loc>https://example.com/</loc>'))
})

test('robots 放行公开内容、屏蔽应用路径并声明 sitemap', () => {
  const txt = renderRobots(SITE)
  assert.ok(txt.includes('User-agent: *'))
  assert.ok(txt.includes('Allow: /knowledge'))
  assert.ok(txt.includes('Allow: /essays'))
  assert.ok(txt.includes('Disallow: /admin'))
  assert.ok(txt.includes('Disallow: /api/'))
  assert.ok(txt.includes(`Sitemap: ${SITE}/sitemap.xml`))
})
