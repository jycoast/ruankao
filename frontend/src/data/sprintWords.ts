// 软考（系统架构设计师）专业英语高频术语。前端静态内置，用于词表 + 翻卡背诵。
// 音标为美音 IPA，基于 Wikipedia/有道/剑桥词典；用户合 main 前抽样核对兜底。

export interface Word {
  word: string
  phonetic?: string
  meaning: string
  category: string
  example?: string
  exampleZh?: string
}

export const SPRINT_WORDS: Word[] = [
  // 架构与设计
  { word: 'architecture', phonetic: '/ˌɑːrkɪˈtektʃər/', meaning: '架构；体系结构', category: '架构设计', example: 'The system architecture defines how components interact.', exampleZh: '系统架构定义了各组件如何交互。' },
  { word: 'component', phonetic: '/kəmˈpoʊnənt/', meaning: '构件；组件', category: '架构设计', example: 'Each component can be developed and tested independently.', exampleZh: '每个组件都可以独立开发和测试。' },
  { word: 'connector', phonetic: '/kəˈnektər/', meaning: '连接件', category: '架构设计', example: 'A connector manages the interaction between components.', exampleZh: '连接件管理组件之间的交互。' },
  { word: 'module', phonetic: '/ˈmɑːdʒuːl/', meaning: '模块', category: '架构设计', example: 'The payment module handles all transactions.', exampleZh: '支付模块处理所有交易。' },
  { word: 'framework', phonetic: '/ˈfreɪmwɜːrk/', meaning: '框架', category: '架构设计', example: 'We built the application on a web framework.', exampleZh: '我们在一个 Web 框架上构建了应用。' },
  { word: 'pattern', phonetic: '/ˈpætərn/', meaning: '模式', category: '架构设计', example: 'This design solves the problem using a well-known pattern.', exampleZh: '该设计使用一种知名模式来解决这个问题。' },
  { word: 'design pattern', phonetic: '/dɪˈzaɪn ˈpætərn/', meaning: '设计模式', category: '架构设计', example: 'The Observer is a common design pattern.', exampleZh: '观察者是一种常见的设计模式。' },
  { word: 'layered architecture', phonetic: '/ˈleɪərd ˌɑːrkɪˈtektʃər/', meaning: '分层架构', category: '架构设计', example: 'A layered architecture separates presentation from data access.', exampleZh: '分层架构将表示层与数据访问层分离。' },
  { word: 'microservice', phonetic: '/ˈmaɪkroʊˌsɜːrvɪs/', meaning: '微服务', category: '架构设计', example: 'Each microservice owns its own database.', exampleZh: '每个微服务拥有自己的数据库。' },
  { word: 'middleware', phonetic: '/ˈmɪdəlwer/', meaning: '中间件', category: '架构设计', example: 'Message middleware decouples the sender from the receiver.', exampleZh: '消息中间件使发送方与接收方解耦。' },
  { word: 'coupling', phonetic: '/ˈkʌplɪŋ/', meaning: '耦合', category: '架构设计', example: 'Low coupling makes a system easier to change.', exampleZh: '低耦合使系统更易于修改。' },
  { word: 'cohesion', phonetic: '/koʊˈhiːʒən/', meaning: '内聚', category: '架构设计', example: 'High cohesion means a module has a single, clear purpose.', exampleZh: '高内聚意味着模块具有单一、明确的职责。' },
  { word: 'abstraction', phonetic: '/əbˈstrækʃən/', meaning: '抽象', category: '架构设计', example: 'Abstraction hides unnecessary implementation details.', exampleZh: '抽象隐藏了不必要的实现细节。' },
  { word: 'encapsulation', phonetic: '/ɪnˌkæpsjuˈleɪʃən/', meaning: '封装', category: '架构设计', example: 'Encapsulation protects an object\'s internal state.', exampleZh: '封装保护对象的内部状态。' },
  { word: 'inheritance', phonetic: '/ɪnˈherɪtəns/', meaning: '继承', category: '架构设计', example: 'A subclass acquires behavior through inheritance.', exampleZh: '子类通过继承获得行为。' },
  { word: 'polymorphism', phonetic: '/ˌpɑːliˈmɔːrfɪzəm/', meaning: '多态', category: '架构设计', example: 'Polymorphism lets one interface serve different types.', exampleZh: '多态让一个接口服务于不同的类型。' },
  { word: 'interface', phonetic: '/ˈɪntərfeɪs/', meaning: '接口', category: '架构设计', example: 'The interface specifies what a component must provide.', exampleZh: '接口规定了组件必须提供什么。' },
  { word: 'refactoring', phonetic: '/riˈfæktərɪŋ/', meaning: '重构', category: '架构设计', example: 'Refactoring improves the code without changing its behavior.', exampleZh: '重构在不改变行为的前提下改进代码。' },
  { word: 'decoupling', phonetic: '/diːˈkʌplɪŋ/', meaning: '解耦', category: '架构设计', example: 'Decoupling modules reduces the ripple effect of changes.', exampleZh: '模块解耦可减少变更带来的连锁影响。' },
  { word: 'topology', phonetic: '/təˈpɑːlədʒi/', meaning: '拓扑（结构）', category: '架构设计', example: 'The network topology describes how nodes are connected.', exampleZh: '网络拓扑描述了节点是如何连接的。' },

  // 质量属性
  { word: 'quality attribute', phonetic: '/ˈkwɑːləti ˈætrɪbjuːt/', meaning: '质量属性', category: '质量属性', example: 'Security is a critical quality attribute of the system.', exampleZh: '安全性是系统的关键质量属性。' },
  { word: 'scalability', phonetic: '/ˌskeɪləˈbɪləti/', meaning: '可扩展性', category: '质量属性', example: 'Good scalability lets the system handle more users.', exampleZh: '良好的可扩展性让系统能承载更多用户。' },
  { word: 'availability', phonetic: '/əˌveɪləˈbɪləti/', meaning: '可用性', category: '质量属性', example: 'The service guarantees 99.9% availability.', exampleZh: '该服务保证 99.9% 的可用性。' },
  { word: 'reliability', phonetic: '/rɪˌlaɪəˈbɪləti/', meaning: '可靠性', category: '质量属性', example: 'Reliability means the system rarely fails.', exampleZh: '可靠性意味着系统很少发生故障。' },
  { word: 'maintainability', phonetic: '/meɪnˌteɪnəˈbɪləti/', meaning: '可维护性', category: '质量属性', example: 'Clean code improves maintainability.', exampleZh: '整洁的代码可提高可维护性。' },
  { word: 'modifiability', phonetic: '/ˌmɑːdɪfaɪəˈbɪləti/', meaning: '可修改性', category: '质量属性', example: 'Modifiability measures how easily the system can be changed.', exampleZh: '可修改性衡量系统被修改的难易程度。' },
  { word: 'portability', phonetic: '/ˌpɔːrtəˈbɪləti/', meaning: '可移植性', category: '质量属性', example: 'Portability lets the software run on different platforms.', exampleZh: '可移植性让软件能在不同平台上运行。' },
  { word: 'usability', phonetic: '/ˌjuːzəˈbɪləti/', meaning: '易用性', category: '质量属性', example: 'Usability testing revealed a confusing menu.', exampleZh: '易用性测试发现了一个令人困惑的菜单。' },
  { word: 'testability', phonetic: '/ˌtestəˈbɪləti/', meaning: '可测试性', category: '质量属性', example: 'Dependency injection improves testability.', exampleZh: '依赖注入可提高可测试性。' },
  { word: 'interoperability', phonetic: '/ˌɪntərˌɑːpərəˈbɪləti/', meaning: '互操作性', category: '质量属性', example: 'Open standards ensure interoperability between systems.', exampleZh: '开放标准确保系统之间的互操作性。' },
  { word: 'performance', phonetic: '/pərˈfɔːrməns/', meaning: '性能', category: '质量属性', example: 'Caching greatly improved the performance.', exampleZh: '缓存极大地提升了性能。' },
  { word: 'throughput', phonetic: '/ˈθruːpʊt/', meaning: '吞吐量', category: '质量属性', example: 'The server\'s throughput reaches 5000 requests per second.', exampleZh: '服务器的吞吐量达到每秒 5000 个请求。' },
  { word: 'latency', phonetic: '/ˈleɪtənsi/', meaning: '延迟', category: '质量属性', example: 'A CDN reduces latency for distant users.', exampleZh: 'CDN 可降低远端用户的延迟。' },
  { word: 'robustness', phonetic: '/roʊˈbʌstnəs/', meaning: '健壮性', category: '质量属性', example: 'Robustness means the system tolerates invalid input.', exampleZh: '健壮性意味着系统能容忍无效输入。' },
  { word: 'redundancy', phonetic: '/rɪˈdʌndənsi/', meaning: '冗余', category: '质量属性', example: 'Redundancy provides a backup when a node fails.', exampleZh: '冗余在节点失效时提供备份。' },
  { word: 'fault tolerance', phonetic: '/fɔːlt ˈtɑːlərəns/', meaning: '容错', category: '质量属性', example: 'Fault tolerance keeps the system running despite failures.', exampleZh: '容错让系统在出现故障时仍能运行。' },
  { word: 'resilience', phonetic: '/rɪˈzɪliəns/', meaning: '弹性；韧性', category: '质量属性', example: 'Resilience lets the system recover quickly from faults.', exampleZh: '韧性让系统能从故障中快速恢复。' },

  // 软件工程
  { word: 'requirement', phonetic: '/rɪˈkwaɪərmənt/', meaning: '需求', category: '软件工程', example: 'The requirement must be clear and testable.', exampleZh: '需求必须清晰且可测试。' },
  { word: 'specification', phonetic: '/ˌspesɪfɪˈkeɪʃən/', meaning: '规格说明', category: '软件工程', example: 'The specification describes what the system should do.', exampleZh: '规格说明描述了系统应当做什么。' },
  { word: 'prototype', phonetic: '/ˈproʊtətaɪp/', meaning: '原型', category: '软件工程', example: 'We built a prototype to validate the idea.', exampleZh: '我们构建了一个原型来验证想法。' },
  { word: 'iteration', phonetic: '/ˌɪtəˈreɪʃən/', meaning: '迭代', category: '软件工程', example: 'Each iteration delivers a working increment.', exampleZh: '每次迭代都交付一个可运行的增量。' },
  { word: 'increment', phonetic: '/ˈɪŋkrəmənt/', meaning: '增量', category: '软件工程', example: 'The team ships a small increment every two weeks.', exampleZh: '团队每两周交付一个小增量。' },
  { word: 'agile', phonetic: '/ˈædʒəl/', meaning: '敏捷', category: '软件工程', example: 'Agile methods welcome changing requirements.', exampleZh: '敏捷方法欢迎需求的变化。' },
  { word: 'waterfall model', phonetic: '/ˈwɔːtərfɔːl ˈmɑːdəl/', meaning: '瀑布模型', category: '软件工程', example: 'The waterfall model proceeds in strict sequential phases.', exampleZh: '瀑布模型按严格的顺序阶段推进。' },
  { word: 'lifecycle', phonetic: '/ˈlaɪfˌsaɪkəl/', meaning: '生命周期', category: '软件工程', example: 'The software lifecycle spans from analysis to retirement.', exampleZh: '软件生命周期从分析一直延续到退役。' },
  { word: 'maintenance', phonetic: '/ˈmeɪntənəns/', meaning: '维护', category: '软件工程', example: 'Most software cost comes from maintenance.', exampleZh: '软件的大部分成本来自维护。' },
  { word: 'verification', phonetic: '/ˌverɪfɪˈkeɪʃən/', meaning: '验证', category: '软件工程', example: 'Verification checks whether we built the product right.', exampleZh: '验证检查我们是否正确地构建了产品。' },
  { word: 'validation', phonetic: '/ˌvælɪˈdeɪʃən/', meaning: '确认', category: '软件工程', example: 'Validation checks whether we built the right product.', exampleZh: '确认检查我们是否构建了正确的产品。' },
  { word: 'test case', phonetic: '/test keɪs/', meaning: '测试用例', category: '软件工程', example: 'Each test case covers one specific scenario.', exampleZh: '每个测试用例覆盖一个特定场景。' },
  { word: 'coverage', phonetic: '/ˈkʌvərɪdʒ/', meaning: '覆盖率', category: '软件工程', example: 'The tests reached 85% code coverage.', exampleZh: '测试达到了 85% 的代码覆盖率。' },
  { word: 'defect', phonetic: '/ˈdiːfekt/', meaning: '缺陷', category: '软件工程', example: 'The reviewer found a defect in the login logic.', exampleZh: '评审者在登录逻辑中发现了一个缺陷。' },
  { word: 'regression test', phonetic: '/rɪˈɡreʃən test/', meaning: '回归测试', category: '软件工程', example: 'A regression test ensures old features still work.', exampleZh: '回归测试确保旧功能仍然可用。' },
  { word: 'deployment', phonetic: '/dɪˈplɔɪmənt/', meaning: '部署', category: '软件工程', example: 'Automated deployment reduces human error.', exampleZh: '自动化部署可减少人为错误。' },
  { word: 'integration', phonetic: '/ˌɪntɪˈɡreɪʃən/', meaning: '集成', category: '软件工程', example: 'Continuous integration merges code changes frequently.', exampleZh: '持续集成会频繁地合并代码变更。' },
  { word: 'traceability', phonetic: '/ˌtreɪsəˈbɪləti/', meaning: '可追溯性', category: '软件工程', example: 'Traceability links each requirement to its test.', exampleZh: '可追溯性将每条需求与其测试关联起来。' },

  // 数据与数据库
  { word: 'database', phonetic: '/ˈdeɪtəbeɪs/', meaning: '数据库', category: '数据库', example: 'All user records are stored in the database.', exampleZh: '所有用户记录都存储在数据库中。' },
  { word: 'schema', phonetic: '/ˈskiːmə/', meaning: '模式；架构', category: '数据库', example: 'The schema defines the tables and their relationships.', exampleZh: '模式定义了表及其关系。' },
  { word: 'transaction', phonetic: '/trænˈzækʃən/', meaning: '事务', category: '数据库', example: 'A transaction either fully succeeds or fully fails.', exampleZh: '事务要么完全成功，要么完全失败。' },
  { word: 'consistency', phonetic: '/kənˈsɪstənsi/', meaning: '一致性', category: '数据库', example: 'Consistency ensures data follows all defined rules.', exampleZh: '一致性确保数据遵循所有既定规则。' },
  { word: 'concurrency', phonetic: '/kənˈkɜːrənsi/', meaning: '并发', category: '数据库', example: 'Locks manage concurrency between transactions.', exampleZh: '锁用于管理事务之间的并发。' },
  { word: 'index', phonetic: '/ˈɪndeks/', meaning: '索引', category: '数据库', example: 'An index speeds up query lookups.', exampleZh: '索引可加快查询检索速度。' },
  { word: 'normalization', phonetic: '/ˌnɔːrmələˈzeɪʃən/', meaning: '规范化', category: '数据库', example: 'Normalization reduces data redundancy.', exampleZh: '规范化可减少数据冗余。' },
  { word: 'redundancy (data)', phonetic: '/rɪˈdʌndənsi ˈdeɪtə/', meaning: '数据冗余', category: '数据库', example: 'Poor design leads to data redundancy.', exampleZh: '糟糕的设计会导致数据冗余。' },
  { word: 'replication', phonetic: '/ˌreplɪˈkeɪʃən/', meaning: '复制', category: '数据库', example: 'Replication copies data across multiple servers.', exampleZh: '复制会将数据拷贝到多台服务器上。' },
  { word: 'sharding', phonetic: '/ˈʃɑːrdɪŋ/', meaning: '分片', category: '数据库', example: 'Sharding splits a large table across many nodes.', exampleZh: '分片将一张大表拆分到多个节点上。' },
  { word: 'data warehouse', phonetic: '/ˈdeɪtə ˈwerhaʊs/', meaning: '数据仓库', category: '数据库', example: 'The data warehouse supports analytical queries.', exampleZh: '数据仓库支持分析型查询。' },
  { word: 'query', phonetic: '/ˈkwɪri/', meaning: '查询', category: '数据库', example: 'The query returns all active users.', exampleZh: '该查询返回所有活跃用户。' },

  // 网络与安全
  { word: 'protocol', phonetic: '/ˈproʊtəkɑːl/', meaning: '协议', category: '网络安全', example: 'HTTP is the protocol used by web browsers.', exampleZh: 'HTTP 是 Web 浏览器所使用的协议。' },
  { word: 'bandwidth', phonetic: '/ˈbændwɪdθ/', meaning: '带宽', category: '网络安全', example: 'Video streaming consumes a lot of bandwidth.', exampleZh: '视频流会消耗大量带宽。' },
  { word: 'router', phonetic: '/ˈruːtər/', meaning: '路由器', category: '网络安全', example: 'The router forwards packets between networks.', exampleZh: '路由器在网络之间转发数据包。' },
  { word: 'gateway', phonetic: '/ˈɡeɪtˌweɪ/', meaning: '网关', category: '网络安全', example: 'An API gateway routes requests to services.', exampleZh: 'API 网关将请求路由到各个服务。' },
  { word: 'firewall', phonetic: '/ˈfaɪərwɔːl/', meaning: '防火墙', category: '网络安全', example: 'The firewall blocks unauthorized traffic.', exampleZh: '防火墙拦截未经授权的流量。' },
  { word: 'encryption', phonetic: '/ɪnˈkrɪpʃən/', meaning: '加密', category: '网络安全', example: 'Encryption protects data in transit.', exampleZh: '加密可保护传输中的数据。' },
  { word: 'decryption', phonetic: '/dɪˈkrɪpʃən/', meaning: '解密', category: '网络安全', example: 'Decryption requires the correct key.', exampleZh: '解密需要正确的密钥。' },
  { word: 'authentication', phonetic: '/ɔːˌθentɪˈkeɪʃən/', meaning: '认证；鉴别', category: '网络安全', example: 'Authentication verifies who the user is.', exampleZh: '认证用于验证用户是谁。' },
  { word: 'authorization', phonetic: '/ɔːˌθɔːrəˈzeɪʃən/', meaning: '授权', category: '网络安全', example: 'Authorization decides what the user may access.', exampleZh: '授权决定用户可以访问什么。' },
  { word: 'integrity', phonetic: '/ɪnˈteɡrəti/', meaning: '完整性', category: '网络安全', example: 'A hash verifies the integrity of the file.', exampleZh: '哈希值用于验证文件的完整性。' },
  { word: 'confidentiality', phonetic: '/ˌkɑːnfɪˌdenʃiˈæləti/', meaning: '机密性', category: '网络安全', example: 'Encryption preserves the confidentiality of messages.', exampleZh: '加密可保持消息的机密性。' },
  { word: 'certificate', phonetic: '/sərˈtɪfɪkət/', meaning: '证书', category: '网络安全', example: 'The website presents a valid TLS certificate.', exampleZh: '该网站出示了有效的 TLS 证书。' },
  { word: 'digital signature', phonetic: '/ˈdɪdʒɪtəl ˈsɪɡnətʃər/', meaning: '数字签名', category: '网络安全', example: 'A digital signature proves the sender\'s identity.', exampleZh: '数字签名可证明发送者的身份。' },
  { word: 'vulnerability', phonetic: '/ˌvʌlnərəˈbɪləti/', meaning: '漏洞', category: '网络安全', example: 'The patch fixes a serious security vulnerability.', exampleZh: '该补丁修复了一个严重的安全漏洞。' },
  { word: 'load balancing', phonetic: '/loʊd ˈbælənsɪŋ/', meaning: '负载均衡', category: '网络安全', example: 'Load balancing distributes traffic across servers.', exampleZh: '负载均衡将流量分摊到多台服务器上。' },

  // 分布式与云
  { word: 'distributed system', phonetic: '/dɪˈstrɪbjətɪd ˈsɪstəm/', meaning: '分布式系统', category: '分布式与云', example: 'A distributed system runs on many networked computers.', exampleZh: '分布式系统运行在许多联网的计算机上。' },
  { word: 'cluster', phonetic: '/ˈklʌstər/', meaning: '集群', category: '分布式与云', example: 'The database runs on a three-node cluster.', exampleZh: '该数据库运行在一个三节点集群上。' },
  { word: 'virtualization', phonetic: '/ˌvɜːrtʃuələˈzeɪʃən/', meaning: '虚拟化', category: '分布式与云', example: 'Virtualization lets one server host many VMs.', exampleZh: '虚拟化让一台服务器承载多个虚拟机。' },
  { word: 'container', phonetic: '/kənˈteɪnər/', meaning: '容器', category: '分布式与云', example: 'A container packages an app with its dependencies.', exampleZh: '容器将应用及其依赖打包在一起。' },
  { word: 'orchestration', phonetic: '/ˌɔːrkɪˈstreɪʃən/', meaning: '编排', category: '分布式与云', example: 'Kubernetes handles container orchestration.', exampleZh: 'Kubernetes 负责容器编排。' },
  { word: 'cloud computing', phonetic: '/klaʊd kəmˈpjuːtɪŋ/', meaning: '云计算', category: '分布式与云', example: 'Cloud computing provides resources on demand.', exampleZh: '云计算按需提供资源。' },
  { word: 'elasticity', phonetic: '/ɪˌlæsˈtɪsəti/', meaning: '弹性伸缩', category: '分布式与云', example: 'Elasticity scales resources up or down automatically.', exampleZh: '弹性伸缩可自动扩容或缩容资源。' },
  { word: 'message queue', phonetic: '/ˈmesɪdʒ kjuː/', meaning: '消息队列', category: '分布式与云', example: 'A message queue buffers tasks between services.', exampleZh: '消息队列在服务之间缓冲任务。' },
  { word: 'partition tolerance', phonetic: '/pɑːrˈtɪʃən ˈtɑːlərəns/', meaning: '分区容错', category: '分布式与云', example: 'Partition tolerance keeps the system working despite network splits.', exampleZh: '分区容错让系统在网络分裂时仍能工作。' },
  { word: 'idempotent', phonetic: '/ˌaɪˈdempətənt/', meaning: '幂等的', category: '分布式与云', example: 'An idempotent request has the same effect if repeated.', exampleZh: '幂等请求即使重复执行效果也相同。' },
  { word: 'cache', phonetic: '/kæʃ/', meaning: '缓存', category: '分布式与云', example: 'The cache stores frequently accessed data.', exampleZh: '缓存存储被频繁访问的数据。' },
  { word: 'scalable', phonetic: '/ˈskeɪləbəl/', meaning: '可伸缩的', category: '分布式与云', example: 'The design is scalable to millions of users.', exampleZh: '该设计可伸缩至数百万用户。' },

  // 项目管理
  { word: 'stakeholder', phonetic: '/ˈsteɪkˌhoʊldər/', meaning: '干系人', category: '项目管理', example: 'Every stakeholder should approve the requirements.', exampleZh: '每一位干系人都应确认需求。' },
  { word: 'milestone', phonetic: '/ˈmaɪlstoʊn/', meaning: '里程碑', category: '项目管理', example: 'The first milestone is due next month.', exampleZh: '第一个里程碑将在下个月到期。' },
  { word: 'deliverable', phonetic: '/dɪˈlɪvərəbəl/', meaning: '交付物', category: '项目管理', example: 'The design document is a key deliverable.', exampleZh: '设计文档是一项关键交付物。' },
  { word: 'baseline', phonetic: '/ˈbeɪslaɪn/', meaning: '基线', category: '项目管理', example: 'The approved plan becomes the project baseline.', exampleZh: '被批准的计划成为项目基线。' },
  { word: 'critical path', phonetic: '/ˈkrɪtɪkəl pæθ/', meaning: '关键路径', category: '项目管理', example: 'A delay on the critical path delays the whole project.', exampleZh: '关键路径上的延误会拖延整个项目。' },
  { word: 'scope', phonetic: '/skoʊp/', meaning: '范围', category: '项目管理', example: 'Adding features expands the project scope.', exampleZh: '增加功能会扩大项目范围。' },
  { word: 'estimation', phonetic: '/ˌestɪˈmeɪʃən/', meaning: '估算', category: '项目管理', example: 'The estimation of effort was too optimistic.', exampleZh: '对工作量的估算过于乐观。' },
  { word: 'risk mitigation', phonetic: '/rɪsk ˌmɪtɪˈɡeɪʃən/', meaning: '风险减轻', category: '项目管理', example: 'A backup plan is a form of risk mitigation.', exampleZh: '备用计划是一种风险减轻手段。' },
  { word: 'constraint', phonetic: '/kənˈstreɪnt/', meaning: '约束', category: '项目管理', example: 'The budget is a hard constraint on the project.', exampleZh: '预算是项目的一项硬性约束。' },
  { word: 'trade-off', phonetic: '/ˈtreɪd ɔːf/', meaning: '权衡', category: '项目管理', example: 'There is a trade-off between speed and cost.', exampleZh: '速度与成本之间存在权衡。' },
]
