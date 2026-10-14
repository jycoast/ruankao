-- 000_baseline_schema.sql —— 建表合并基线
--
-- 用途：在空库上一次建出全部业务表。所有语句均为 CREATE TABLE IF NOT EXISTS，
--       对已有库执行是安全的（表已存在时整条语句被跳过，索引与外键自然不会重复添加）。
--
-- 内容来源：生产库 software_exam 的 SHOW CREATE TABLE 实际输出，共 42 张表；
--           另有 1 张表生产库不存在，见下方第 1 条。
--
-- 与生产库的已知偏离：
--   1) daily_sessions —— 生产库中不存在该表（已知的生产缺口）。此处按仓库
--      migrations/084_daily_sessions.sql 的原始 CREATE TABLE 定义补入，
--      表尾的 ENGINE/CHARSET 按本文件统一规则改写。
--   2) users.linux_do_id —— 该列（含为其建的索引 idx_linux_do_id）不收录。
--      它没有任何迁移创建过（全仓搜索无 SQL 引用），是生产库里的孤儿列；
--      代码实际使用的是 linuxdo_id 与 idx_linuxdo_id
--      （见 backend/internal/domain/repository/user.go:71）。
--
-- 统一化处理（相对生产库 SHOW CREATE TABLE 输出）：
--   - 表尾统一为 ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci
--     （生产库中部分表继承服务器默认的 utf8mb4_0900_ai_ci，此处改为与
--      docker-compose 的 --collation-server=utf8mb4_unicode_ci 一致）
--   - 去掉表尾的 AUTO_INCREMENT=<n>（那是当前自增值，不属于表结构）；
--     列上的 AUTO_INCREMENT 属性保留
--   - 标识符不加反引号，与 001_init.sql 等现有迁移风格一致
--   - 表按外键依赖顺序排列（被引用的表在前），保证空库能一次性建起来
--
-- 不包含：CREATE DATABASE / USE，以及任何 ALTER TABLE / CREATE INDEX / DROP 语句。
-- 所有索引与外键都内联在各自的 CREATE TABLE 里。


-- users
CREATE TABLE IF NOT EXISTS users (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    username VARCHAR(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    email VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    password_hash VARCHAR(255) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    nickname VARCHAR(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
    avatar VARCHAR(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
    phone VARCHAR(20) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
    role TINYINT NOT NULL DEFAULT '0' COMMENT '0:普通用户 1:管理员',
    status TINYINT NOT NULL DEFAULT '1' COMMENT '0:禁用 1:正常',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    linuxdo_id BIGINT UNSIGNED NOT NULL DEFAULT '0' COMMENT 'LinuxDo 用户ID，0=未绑定',
    subject_id BIGINT UNSIGNED NOT NULL DEFAULT '1' COMMENT '当前考试科目(跨设备同步)',
    PRIMARY KEY (id),
    UNIQUE KEY username (username),
    UNIQUE KEY email (email),
    KEY idx_users_email (email),
    KEY idx_users_username (username),
    KEY idx_linuxdo_id (linuxdo_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- subjects
CREATE TABLE IF NOT EXISTS subjects (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    code VARCHAR(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
    icon VARCHAR(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
    sort_order INT DEFAULT '0',
    status TINYINT NOT NULL DEFAULT '1',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    has_essay TINYINT NOT NULL DEFAULT '0' COMMENT '是否含下午论文(0否 1是)',
    has_sprint TINYINT NOT NULL DEFAULT '0' COMMENT '是否提供冲刺资料(0否 1是)',
    level TINYINT NOT NULL DEFAULT '2' COMMENT '资格等级(1高级 2中级 3初级)',
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- categories
CREATE TABLE IF NOT EXISTS categories (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    parent_id BIGINT UNSIGNED DEFAULT '0',
    sort_order INT DEFAULT '0',
    description TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_categories_parent (parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- questions
CREATE TABLE IF NOT EXISTS questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    subject_id BIGINT UNSIGNED DEFAULT '1',
    title TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    content TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    question_type TINYINT NOT NULL COMMENT '1:单选 2:多选 3:案例分析 4:论文',
    difficulty TINYINT DEFAULT '1' COMMENT '1:简单 2:中等 3:困难',
    category_id BIGINT UNSIGNED DEFAULT '0',
    knowledge_point VARCHAR(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
    year SMALLINT DEFAULT '0',
    sort_order INT NOT NULL DEFAULT '0' COMMENT '排序权重：0=未手动排序(排最后)，数值越小越靠前',
    batch TINYINT NOT NULL DEFAULT '0' COMMENT '考试批次 0:单次 1:上半年 2:下半年',
    paper_section VARCHAR(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '' COMMENT '试卷部分：上午/下午I/下午II',
    explanation TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
    analysis TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
    score DECIMAL(5,1) DEFAULT '0.0',
    status TINYINT NOT NULL DEFAULT '1',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_questions_type (question_type),
    KEY idx_questions_year (year),
    KEY idx_questions_category (category_id),
    KEY idx_questions_difficulty (difficulty),
    KEY idx_questions_subject (subject_id),
    KEY idx_questions_year_batch (year, batch),
    KEY idx_questions_type_status_subject (question_type, status, subject_id),
    KEY idx_questions_status_subject (status, subject_id),
    FULLTEXT KEY idx_questions_fulltext (title, content)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- options
CREATE TABLE IF NOT EXISTS options (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    question_id BIGINT UNSIGNED NOT NULL,
    option_label CHAR(1) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    content TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    is_correct TINYINT NOT NULL DEFAULT '0',
    sort_order INT DEFAULT '0',
    PRIMARY KEY (id),
    KEY idx_options_question (question_id),
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- papers
CREATE TABLE IF NOT EXISTS papers (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    subject_id BIGINT UNSIGNED DEFAULT '1',
    title VARCHAR(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    year SMALLINT NOT NULL,
    batch TINYINT NOT NULL DEFAULT '0' COMMENT '考试批次 0:单次 1:上半年 2:下半年',
    paper_type TINYINT NOT NULL COMMENT '1:上午题 2:下午题I 3:下午题II 4:综合',
    total_score DECIMAL(5,1) DEFAULT '0.0',
    duration INT DEFAULT '150' COMMENT '考试时长(分钟)',
    description TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
    status TINYINT NOT NULL DEFAULT '1',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    source TINYINT NOT NULL DEFAULT '0',
    created_by BIGINT UNSIGNED NOT NULL DEFAULT '0',
    PRIMARY KEY (id),
    KEY idx_papers_year (year),
    KEY idx_papers_type (paper_type),
    KEY idx_papers_subject (subject_id),
    KEY idx_papers_year_batch (year, batch),
    KEY idx_paper_source (source),
    KEY idx_paper_created_by (created_by)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- paper_questions
CREATE TABLE IF NOT EXISTS paper_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    paper_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    sort_order INT DEFAULT '0',
    PRIMARY KEY (id),
    UNIQUE KEY idx_paper_question (paper_id, question_id),
    KEY question_id (question_id),
    FOREIGN KEY (paper_id) REFERENCES papers(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- knowledge_points
CREATE TABLE IF NOT EXISTS knowledge_points (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    name VARCHAR(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    category_id BIGINT UNSIGNED DEFAULT '0',
    parent_id BIGINT UNSIGNED DEFAULT '0',
    content TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
    key_points TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci COMMENT '考点',
    easy_mistakes TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci COMMENT '易错点',
    match_key VARCHAR(300) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    sort_order INT DEFAULT '0',
    status TINYINT NOT NULL DEFAULT '1',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    subject_id BIGINT UNSIGNED NOT NULL DEFAULT '1',
    PRIMARY KEY (id),
    KEY idx_kp_category (category_id),
    KEY idx_kp_parent (parent_id),
    KEY idx_kp_subject_id (subject_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- knowledge_point_questions
CREATE TABLE IF NOT EXISTS knowledge_point_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    knowledge_point_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY idx_kp_question (knowledge_point_id, question_id),
    KEY question_id (question_id),
    FOREIGN KEY (knowledge_point_id) REFERENCES knowledge_points(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- articles
CREATE TABLE IF NOT EXISTS articles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    title VARCHAR(200) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    content TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci NOT NULL,
    article_type TINYINT NOT NULL COMMENT '1:论文范文 2:写作模板 3:评分标准 4:AI点评',
    category VARCHAR(100) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
    year SMALLINT DEFAULT '0',
    score DECIMAL(5,1) DEFAULT '0.0' COMMENT '评分',
    author VARCHAR(50) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
    tags VARCHAR(500) CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci DEFAULT '',
    status TINYINT NOT NULL DEFAULT '1',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    subject_id BIGINT UNSIGNED NOT NULL DEFAULT '1',
    PRIMARY KEY (id),
    KEY idx_articles_type (article_type),
    KEY idx_articles_year (year),
    KEY idx_art_subject_id (subject_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- wrong_questions
CREATE TABLE IF NOT EXISTS wrong_questions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    wrong_count INT DEFAULT '1',
    correct_count INT DEFAULT '0',
    last_wrong_answer TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
    next_review_time DATETIME DEFAULT NULL COMMENT '下次复习时间(Anki)',
    interval_days INT DEFAULT '1' COMMENT '复习间隔天数',
    ease_factor DECIMAL(3,2) DEFAULT '2.50' COMMENT '易度因子',
    status TINYINT NOT NULL DEFAULT '1' COMMENT '1:待复习 2:已掌握',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY idx_wq_user_question (user_id, question_id),
    KEY idx_wq_user_status (user_id, status),
    KEY idx_wq_review_time (user_id, next_review_time),
    KEY question_id (question_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- favorites
CREATE TABLE IF NOT EXISTS favorites (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    target_type TINYINT NOT NULL COMMENT '1:题目 2:论文 3:知识点',
    target_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY idx_fav_target (user_id, target_type, target_id),
    KEY idx_fav_user (user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- practice_records
CREATE TABLE IF NOT EXISTS practice_records (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    practice_type TINYINT NOT NULL COMMENT '1:顺序刷题 2:随机刷题 3:专项练习 4:错题重练 5:收藏题练习 6:模拟考试',
    user_answer TEXT CHARACTER SET utf8mb4 COLLATE utf8mb4_unicode_ci,
    is_correct TINYINT DEFAULT '0',
    time_spent INT DEFAULT '0' COMMENT '用时(秒)',
    paper_id BIGINT UNSIGNED DEFAULT '0',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_pr_user (user_id),
    KEY idx_pr_question (question_id),
    KEY idx_pr_type (practice_type),
    KEY idx_pr_created (created_at),
    KEY idx_pr_user_question (user_id, question_id),
    KEY idx_pr_user_type (user_id, practice_type),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (question_id) REFERENCES questions(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- exam_records
CREATE TABLE IF NOT EXISTS exam_records (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    paper_id BIGINT UNSIGNED NOT NULL,
    score DECIMAL(5,1) DEFAULT '0.0',
    total_questions INT DEFAULT '0',
    correct_count INT DEFAULT '0',
    time_spent INT DEFAULT '0' COMMENT '用时(秒)',
    started_at DATETIME NOT NULL,
    finished_at DATETIME DEFAULT NULL,
    status TINYINT NOT NULL DEFAULT '0' COMMENT '0:进行中 1:已完成',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_er_user (user_id),
    KEY idx_er_paper (paper_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    FOREIGN KEY (paper_id) REFERENCES papers(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- user_profiles
CREATE TABLE IF NOT EXISTS user_profiles (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    total_practice INT DEFAULT '0',
    total_correct INT DEFAULT '0',
    total_time INT DEFAULT '0' COMMENT '总学习时长(秒)',
    streak_days INT DEFAULT '0' COMMENT '连续学习天数',
    last_practice_date DATE DEFAULT NULL,
    target_date DATE DEFAULT NULL COMMENT '目标考试日期',
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY user_id (user_id),
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- user_subject_stats
CREATE TABLE IF NOT EXISTS user_subject_stats (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    subject_id BIGINT UNSIGNED NOT NULL,
    total_practice INT NOT NULL DEFAULT '0',
    total_correct INT NOT NULL DEFAULT '0',
    total_time INT NOT NULL DEFAULT '0',
    streak_days INT NOT NULL DEFAULT '0',
    last_practice_at DATETIME DEFAULT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_subject (user_id, subject_id),
    KEY idx_user (user_id),
    KEY idx_subject (subject_id),
    CONSTRAINT fk_usstats_user FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE,
    CONSTRAINT fk_usstats_subject FOREIGN KEY (subject_id) REFERENCES subjects(id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- exam_schedules
CREATE TABLE IF NOT EXISTS exam_schedules (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    label VARCHAR(50) NOT NULL COMMENT '排期名称，如 2026年秋季',
    exam_date DATE NOT NULL COMMENT '考试日期',
    created_at DATETIME DEFAULT NULL,
    updated_at DATETIME DEFAULT NULL,
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- paper_practice_sessions
CREATE TABLE IF NOT EXISTS paper_practice_sessions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    paper_id BIGINT UNSIGNED NOT NULL,
    mode VARCHAR(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'practice',
    data TEXT COLLATE utf8mb4_unicode_ci NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY idx_pps_scope (user_id, paper_id, mode)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- practice_sessions
CREATE TABLE IF NOT EXISTS practice_sessions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    subject_id BIGINT UNSIGNED NOT NULL DEFAULT '0',
    question_type TINYINT NOT NULL,
    knowledge_point VARCHAR(200) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    question_ids TEXT COLLATE utf8mb4_unicode_ci NOT NULL,
    answers TEXT COLLATE utf8mb4_unicode_ci NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY idx_session_scope (user_id, subject_id, question_type, knowledge_point)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- daily_sessions
CREATE TABLE IF NOT EXISTS daily_sessions (
    id BIGINT UNSIGNED PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    date VARCHAR(10) NOT NULL,
    subject_id BIGINT UNSIGNED NOT NULL,
    data TEXT NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE INDEX idx_daily_session_scope (user_id, date, subject_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- daily_checkins
CREATE TABLE IF NOT EXISTS daily_checkins (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    checkin_date VARCHAR(10) COLLATE utf8mb4_unicode_ci NOT NULL,
    question_count INT NOT NULL DEFAULT '0',
    correct_count INT NOT NULL DEFAULT '0',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_date (user_id, checkin_date)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- question_notes
CREATE TABLE IF NOT EXISTS question_notes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    question_id BIGINT UNSIGNED NOT NULL,
    content LONGTEXT COLLATE utf8mb4_unicode_ci NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_question (user_id, question_id),
    KEY idx_user_id (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- question_comments
CREATE TABLE IF NOT EXISTS question_comments (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    question_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    parent_id BIGINT UNSIGNED NOT NULL DEFAULT '0',
    reply_to_user_id BIGINT UNSIGNED NOT NULL DEFAULT '0',
    content TEXT NOT NULL,
    status TINYINT NOT NULL DEFAULT '0' COMMENT '0=正常 1:用户自删 2:管理员隐藏',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_q (question_id),
    KEY idx_user (user_id),
    KEY idx_parent (parent_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- comment_reports
CREATE TABLE IF NOT EXISTS comment_reports (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    comment_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    reason VARCHAR(200) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT '0' COMMENT '0=待处理 1=已处理(已隐藏评论) 2=驳回',
    admin_note VARCHAR(500) DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    handled_at DATETIME DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_comment_user (comment_id, user_id),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- question_reports
CREATE TABLE IF NOT EXISTS question_reports (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    question_id BIGINT UNSIGNED NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    report_type TINYINT NOT NULL DEFAULT '1',
    description TEXT NOT NULL,
    status TINYINT NOT NULL DEFAULT '0',
    admin_note VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_question_id (question_id),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- essay_submissions
CREATE TABLE IF NOT EXISTS essay_submissions (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,
    category VARCHAR(100) NOT NULL DEFAULT '',
    year SMALLINT NOT NULL DEFAULT '0',
    score FLOAT NOT NULL DEFAULT '0',
    review_status TINYINT NOT NULL DEFAULT '0',
    reviewer_note VARCHAR(500) NOT NULL DEFAULT '',
    article_id BIGINT UNSIGNED NOT NULL DEFAULT '0',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    KEY idx_user_id (user_id),
    KEY idx_review_status (review_status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- feedbacks
CREATE TABLE IF NOT EXISTS feedbacks (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    feedback_type TINYINT NOT NULL DEFAULT '1',
    content TEXT NOT NULL,
    contact VARCHAR(100) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT '0',
    admin_note VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    question_id BIGINT UNSIGNED NOT NULL DEFAULT '0' COMMENT '关联题目ID，0=未关联',
    PRIMARY KEY (id),
    KEY idx_user_id (user_id),
    KEY idx_status (status),
    KEY idx_question_id (question_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- announcements
CREATE TABLE IF NOT EXISTS announcements (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    title VARCHAR(200) NOT NULL,
    content TEXT NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL DEFAULT '0',
    status TINYINT NOT NULL DEFAULT '1',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- announcement_reads
CREATE TABLE IF NOT EXISTS announcement_reads (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    announcement_id BIGINT UNSIGNED NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user_ann (user_id, announcement_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- notifications
CREATE TABLE IF NOT EXISTS notifications (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    type TINYINT NOT NULL DEFAULT '1',
    title VARCHAR(120) NOT NULL DEFAULT '',
    content TEXT,
    biz_type VARCHAR(20) NOT NULL DEFAULT '',
    biz_id BIGINT UNSIGNED NOT NULL DEFAULT '0',
    link VARCHAR(200) NOT NULL DEFAULT '',
    read_at DATETIME DEFAULT NULL,
    created_at DATETIME DEFAULT NULL,
    PRIMARY KEY (id),
    KEY idx_notif_user_read (user_id, read_at, id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- membership_plans
CREATE TABLE IF NOT EXISTS membership_plans (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    plan_key VARCHAR(20) COLLATE utf8mb4_unicode_ci NOT NULL,
    name VARCHAR(50) COLLATE utf8mb4_unicode_ci NOT NULL,
    days INT NOT NULL,
    amount INT NOT NULL,
    ldc_amount INT NOT NULL,
    sort_order INT NOT NULL DEFAULT '0',
    enabled TINYINT NOT NULL DEFAULT '1',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY plan_key (plan_key),
    KEY idx_sort (sort_order)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- memberships
CREATE TABLE IF NOT EXISTS memberships (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    plan VARCHAR(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    expire_at DATETIME NOT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY user_id (user_id),
    KEY idx_expire (expire_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- orders
CREATE TABLE IF NOT EXISTS orders (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    out_trade_no VARCHAR(64) COLLATE utf8mb4_unicode_ci NOT NULL,
    user_id BIGINT UNSIGNED NOT NULL,
    plan VARCHAR(20) COLLATE utf8mb4_unicode_ci NOT NULL,
    channel VARCHAR(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT 'yipay',
    amount INT NOT NULL,
    status TINYINT NOT NULL DEFAULT '0',
    pay_channel VARCHAR(20) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    trade_no VARCHAR(64) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    paid_at DATETIME DEFAULT NULL,
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY out_trade_no (out_trade_no),
    KEY idx_user (user_id),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- card_keys
CREATE TABLE IF NOT EXISTS card_keys (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code VARCHAR(32) COLLATE utf8mb4_unicode_ci NOT NULL,
    plan VARCHAR(20) COLLATE utf8mb4_unicode_ci NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL DEFAULT '0',
    used_by BIGINT UNSIGNED NOT NULL DEFAULT '0',
    used_at DATETIME DEFAULT NULL,
    note VARCHAR(200) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT '0',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY code (code),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- invite_codes
CREATE TABLE IF NOT EXISTS invite_codes (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    code VARCHAR(20) COLLATE utf8mb4_unicode_ci NOT NULL,
    created_by BIGINT UNSIGNED NOT NULL DEFAULT '0',
    used_by BIGINT UNSIGNED NOT NULL DEFAULT '0',
    used_at DATETIME DEFAULT NULL,
    note VARCHAR(200) COLLATE utf8mb4_unicode_ci NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT '0',
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    PRIMARY KEY (id),
    UNIQUE KEY code (code),
    KEY idx_status (status)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- password_resets
CREATE TABLE IF NOT EXISTS password_resets (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    account VARCHAR(100) NOT NULL DEFAULT '',
    contact VARCHAR(100) NOT NULL DEFAULT '',
    reason VARCHAR(500) NOT NULL DEFAULT '',
    client_ip VARCHAR(64) NOT NULL DEFAULT '',
    status TINYINT NOT NULL DEFAULT '0',
    admin_note VARCHAR(500) NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT NULL,
    updated_at DATETIME DEFAULT NULL,
    PRIMARY KEY (id),
    KEY idx_pwreset_user (user_id),
    KEY idx_pwreset_status (status, created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- ai_configs
CREATE TABLE IF NOT EXISTS ai_configs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    base_url VARCHAR(255) NOT NULL DEFAULT '',
    api_key VARCHAR(255) NOT NULL DEFAULT '',
    model VARCHAR(100) NOT NULL DEFAULT '',
    temperature DECIMAL(3,2) NOT NULL DEFAULT '0.70',
    max_tokens INT NOT NULL DEFAULT '2048',
    enabled TINYINT NOT NULL DEFAULT '0',
    created_at DATETIME DEFAULT NULL,
    updated_at DATETIME DEFAULT NULL,
    free_trial_limit INT NOT NULL DEFAULT '5',
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- ai_call_logs
CREATE TABLE IF NOT EXISTS ai_call_logs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    scene VARCHAR(32) NOT NULL DEFAULT '',
    model VARCHAR(100) NOT NULL DEFAULT '',
    prompt_tokens INT NOT NULL DEFAULT '0',
    completion_tokens INT NOT NULL DEFAULT '0',
    total_tokens INT NOT NULL DEFAULT '0',
    duration_ms INT NOT NULL DEFAULT '0',
    success TINYINT NOT NULL DEFAULT '1',
    err_msg VARCHAR(255) NOT NULL DEFAULT '',
    own_key TINYINT NOT NULL DEFAULT '0',
    estimated TINYINT NOT NULL DEFAULT '0',
    created_at DATETIME DEFAULT NULL,
    PRIMARY KEY (id),
    KEY idx_ai_log_user (user_id),
    KEY idx_ai_log_scene (scene),
    KEY idx_ai_log_created (created_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- ai_conversations
CREATE TABLE IF NOT EXISTS ai_conversations (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    title VARCHAR(100) NOT NULL DEFAULT '',
    created_at DATETIME DEFAULT NULL,
    updated_at DATETIME DEFAULT NULL,
    PRIMARY KEY (id),
    KEY idx_ai_conv_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- ai_messages
CREATE TABLE IF NOT EXISTS ai_messages (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    conversation_id BIGINT UNSIGNED NOT NULL,
    role VARCHAR(16) NOT NULL,
    content LONGTEXT NOT NULL,
    created_at DATETIME DEFAULT NULL,
    PRIMARY KEY (id),
    KEY idx_ai_msg_conv (conversation_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- ai_trial_usages
CREATE TABLE IF NOT EXISTS ai_trial_usages (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    used_count INT NOT NULL DEFAULT '0',
    created_at DATETIME DEFAULT NULL,
    updated_at DATETIME DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_ai_trial_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- user_ai_configs
CREATE TABLE IF NOT EXISTS user_ai_configs (
    id BIGINT UNSIGNED NOT NULL AUTO_INCREMENT,
    user_id BIGINT UNSIGNED NOT NULL,
    base_url VARCHAR(255) NOT NULL DEFAULT '',
    api_key VARCHAR(255) NOT NULL DEFAULT '',
    model VARCHAR(100) NOT NULL DEFAULT '',
    temperature DECIMAL(3,2) NOT NULL DEFAULT '0.70',
    max_tokens INT NOT NULL DEFAULT '2048',
    enabled TINYINT NOT NULL DEFAULT '0',
    created_at DATETIME DEFAULT NULL,
    updated_at DATETIME DEFAULT NULL,
    PRIMARY KEY (id),
    UNIQUE KEY uk_user (user_id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;


-- site_settings
CREATE TABLE IF NOT EXISTS site_settings (
    id TINYINT NOT NULL,
    maintenance_enabled TINYINT NOT NULL DEFAULT '0',
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    PRIMARY KEY (id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
