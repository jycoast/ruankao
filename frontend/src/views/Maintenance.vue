<!-- 系统维护页。刻意做成一个不依赖任何接口、任何 store 的静态页 ——
     它要在站点最不正常的时候正常显示出来。 -->
<template>
  <div class="flex min-h-screen flex-col items-center justify-center bg-gray-50 px-6 text-center dark:bg-gray-900">
    <el-icon class="mb-6 text-6xl text-blue-500"><Tools /></el-icon>
    <h1 class="text-2xl font-bold text-gray-900 dark:text-gray-100">系统维护中</h1>
    <p class="mt-4 max-w-md text-gray-600 dark:text-gray-400">
      本站正在进行系统维护，预计很快恢复。给你带来不便，敬请谅解。
    </p>
    <p class="mt-2 max-w-md text-sm text-gray-400 dark:text-gray-500">
      维护期间所有功能暂时不可用，请稍后再试。
    </p>
    <!-- 留一个不起眼的管理员入口：维护期间后台仍可访问，
         没有这个入口管理员得自己敲 URL 才能进去关开关 -->
    <router-link
      to="/login"
      class="mt-10 text-sm text-gray-400 underline hover:text-gray-600 dark:hover:text-gray-300"
    >
      管理员登录
    </router-link>
  </div>
</template>
