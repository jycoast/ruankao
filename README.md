# 软考星图

> 面向软考考生的在线刷题备考平台：历年真题、间隔重复背题、知识点导图、AI 助教与模拟考试。

![Go](https://img.shields.io/badge/Go-1.24-00ADD8?style=flat&logo=go&logoColor=white) ![Vue](https://img.shields.io/badge/Vue-3.5-4FC08D?style=flat&logo=vue.js&logoColor=white) ![TypeScript](https://img.shields.io/badge/TypeScript-5.6-3178C6?style=flat&logo=typescript&logoColor=white) ![MySQL](https://img.shields.io/badge/MySQL-8.0-4479A1?style=flat&logo=mysql&logoColor=white) ![Redis](https://img.shields.io/badge/Redis-7-DC382D?style=flat&logo=redis&logoColor=white) ![License](https://img.shields.io/badge/License-MIT-green)

**[在线体验 →](https://ruankao.dpdns.org)**

<!-- 截图占位：把图片放入 docs/images/ 后取消注释即可
![首页](docs/images/home.png)
![练习](docs/images/practice.png)
![知识导图](docs/images/knowledge.png)
建议宽度 1200px
-->

> **本仓库只含应用代码，不含题库数据。** 克隆后执行一次数据库迁移即可启动 —— 表结构完整，可注册登录，但没有题目，也没有预置管理员账号（需手动把某个用户的 `role` 改成 1）。详见[数据说明](#数据说明)。

## 功能特性

**刷题与练习**

- **模拟考试** —— 上午综合知识 / 下午案例与论文分卷作答，倒计时交卷、自动评分，交卷后逐题看解析
- **多维练习** —— 按科目、年份、章节、知识点、题型自由筛选
- **每日一练** —— 每天一组题，交卷后生成学习报告
- **错题本** —— 自动收录错题，支持搜索、待复习筛选、一键重做
- **间隔重复** —— 简化版 SM-2 算法排程复习，答错回落重来

**学习资料**

- **历年真题** —— 单选 / 多选 / 案例分析 / 论文，覆盖初、中、高级多个科目
- **知识点导图** —— 层级知识树 + Markmap 思维导图，支持导出 PDF / 长图
- **论文范文库** —— 分类范文、写作模板与评分标准
- **笔记** —— 富文本编辑器，支持图片粘贴上传，可按题目或知识点记录

**AI 助教**

- **题目解析** —— 流式输出讲解，可追问
- **出题与组卷** —— 按知识点生成练习题、生成模拟卷
- **论文与案例批改** —— 生成范文，按评分标准批改并给修改建议
- **学习诊断报告** —— 汇总答题数据，给出薄弱环节分析
- **自带模型** —— 底层是 OpenAI 兼容接口，站点可在后台配置全局模型；会员也可接入自己的 Key，优先于全局配置生效

**社区与反馈** —— 题目纠错上报、评论与举报、意见反馈、站内消息通知，均有对应的后台处理入口

**会员与支付** —— Freemium 模式，非会员可试用 AI（次数由后台配置）；按套餐开通（月 / 季 / 考季 / 年），支持易支付多通道回调、卡密与邀请码

**平台特性**

- **游客模式** —— 未登录可浏览试卷、真题、知识点导图、论文范文及冲刺只读页，写操作再引导登录
- **PWA** —— 应用壳离线可用，发版自动更新 Service Worker 缓存
- **SEO 预渲染** —— 构建期产出静态内容页，并生成 `sitemap.xml` 与 `robots.txt`
- **后台管理** —— 题库、试卷、知识点、文章、用户、订单、评论、AI 配置等一站式管理

## 技术栈

| 层级   | 技术                                                                        |
| ------ | --------------------------------------------------------------------------- |
| 前端   | Vue 3 + TypeScript + Vite + Pinia + Vue Router + Tailwind CSS + Element Plus |
| 后端   | Go 1.24 + Gin + GORM + JWT + Viper                                          |
| 数据库 | MySQL 8.0 + Redis 7                                                         |
| 富文本 | Tiptap + marked + DOMPurify（渲染前统一净化）                               |
| 导出   | markmap（思维导图）+ jsPDF / html2canvas（PDF 与长图）                      |
| 部署   | Docker Compose + Nginx；前端亦可托管于任意静态服务                          |

## 快速开始

### 环境要求

Go >= 1.24、Node.js >= 18、MySQL >= 8.0、Redis >= 7，Docker 可选。

### 方式一：Docker Compose

需先准备好 `.env` 与 `backend/config/config.yaml` 两个文件（compose 与镜像分别读取它们），然后：

```bash
docker compose up -d --build
```

compose 定义三个服务：MySQL（宿主机 **33060**）、Redis（**63790**）、后端（**8080**）。MySQL 首次初始化时会自动执行 `backend/migrations/` 下的建库基线。

### 方式二：本地开发

```bash
# 准备配置：填数据库密码与 jwt.secret
cp backend/config/config.example.yaml backend/config/config.yaml
cp frontend/.env.example frontend/.env.development

# 初始化数据库：建表 + 写入维表种子
mysql -u root -p -e "CREATE DATABASE software_exam DEFAULT CHARSET utf8mb4"
cd backend && go run ./cmd/migrate

# 启动后端（:8080）
go run ./cmd/server

# 启动前端（:3000，/api 代理到后端）
cd frontend && npm install && npm run dev
```

## 配置说明

后端配置来自 `backend/config/config.yaml`（模板见 `config.example.yaml`）：

```yaml
server:
  port: 8080
  mode: release # debug / test / release
  site_url: "" # 站点根地址，留空时按请求推断
database:
  host: localhost
  port: 3306
  user: root
  password: ""
  dbname: software_exam
redis:
  host: localhost
  port: 6379
jwt:
  secret: "" # 必填，release 模式下不少于 32 个字符
  expire_hour: 72
```

敏感项均可用环境变量覆盖（`SOFTWARE_EXAM_` 前缀 + 下划线分层，如 `SOFTWARE_EXAM_DATABASE_PASSWORD`），完整列表见 `backend/config/config.go` 的 `BindEnv` 调用。

前端只有一个变量 `VITE_API_BASE_URL`，它唯一决定产物里的 API 地址：同源部署填 `/api`，前后端分离部署填后端完整地址（如 `https://api.example.com/api`）。**生产构建缺失该变量会直接失败**，避免静默产出坏包。

## 项目结构

```
software-exam/
├── backend/
│   ├── cmd/server/              # HTTP 服务入口
│   ├── cmd/migrate/             # 数据库迁移工具
│   ├── internal/
│   │   ├── api/                 # handler / middleware / router
│   │   ├── application/service/ # 业务逻辑
│   │   └── domain/              # model 与 repository
│   ├── pkg/                     # aiclient / auth / database / redisclient / online / response
│   ├── config/                  # 配置模板
│   └── migrations/              # 合并建库基线（000 结构 + 001 种子）
├── frontend/
│   ├── build/                   # 构建期校验
│   ├── scripts/prerender/       # SEO 预渲染
│   └── src/                     # api / components / composables / router / stores / views
├── deploy/                      # Nginx 配置与 MySQL 备份 / 恢复脚本
└── docker-compose.yml
```

## 数据说明

本仓库不含题库数据。`backend/migrations/` 顶层只有两个合并基线，均幂等，重复执行无害：`000_baseline_schema.sql` 建全部表结构，`001_baseline_seed.sql` 写科目、会员套餐等配置型维表。

自行导入题目时，按建出的表结构写入即可，约定：

- 纯补充，不修改已有数据；按题干（而非题号）查重
- `knowledge_point` 格式为 `主类>子类`，主类沿用已有分类，不要新建

## 部署

前端为纯静态产物，`npm run build`（含类型检查与 SEO 预渲染）后可托管在任意静态服务；后端与 MySQL / Redis 部署在服务器，由 Nginx 反代。本项目线上即采用「静态托管前端 + 独立域名后端 API」的拓扑。

## 贡献

欢迎提交 Issue 与 Pull Request。开发环境准备、分支与提交约定、数据库迁移规范等请先阅读 [CONTRIBUTING.md](CONTRIBUTING.md)；参与讨论请遵守 [CODE_OF_CONDUCT.md](CODE_OF_CONDUCT.md)。

## 版权与免责声明

- 本仓库只包含代码，不包含任何题库数据；历年真题为历届软考公开试题，需使用者自行获取并导入。
- 导入时请自行确认数据来源的版权与使用许可。
- 若认为本项目的任何内容侵犯了您的权益，请在仓库提 Issue 联系下架。

## 许可证

本项目基于 [MIT License](LICENSE) 开源。
