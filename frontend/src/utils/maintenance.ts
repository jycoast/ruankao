// 维护模式下前端依然放行的路径。
//   /maintenance —— 否则无限重定向到自己
//   /admin/*     —— 后台归后端 mw.Auth + mw.Admin 把关，不该被前端维护页拦住。
//                   这也保证了「用户态读不到 role」时管理员仍有入口（见设计文档「已知边界」1）
//   /login       —— 管理员登录入口（它本来就在游客白名单里）
//
// 这张表只回答「要不要做维护改道」，**不**豁免游客块的白名单判定：
// 维护中的游客正是先被改道到 /maintenance，再由游客块拿 isGuestAllowedPath 判一次。
// 所以 /maintenance 也在 isGuestAllowedPath 里。两张表先后被查，删掉任何一处都会
// 立刻复活「维护块 ↔ 游客块」互踢的无限重定向。
//
// 两个消费者必须共用这一份实现，否则会漂移：
//   1. 路由守卫（router/index.ts）—— 导航时改道
//   2. axios 响应拦截器（api/index.ts）—— 用户已停在页面里、维护突然开启时按 503 改道
//
// 拦截器漏掉这条谓词会让 /login 自己把自己弹回维护页：登录页外壳 AuthLayout 会
// fire-and-forget 地拉 /exam-schedules（不在后端白名单），拿到 503 后拦截器整页跳维护页。
// 管理员因此连登录表单都看不到 —— 后端放行 /api/auth/login 的用意（「令牌一过期就永远
// 进不去」）被前端这一层抵消。调用方的 .catch 拦不住：拦截器在 reject 之前就已经跳了。
export function isMaintenanceBypassPath(path: string): boolean {
  return path === '/maintenance' || path.startsWith('/admin') || path === '/login'
}
