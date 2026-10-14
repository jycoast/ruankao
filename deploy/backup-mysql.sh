#!/usr/bin/env bash
# MySQL 逻辑备份：mysqldump + gzip，保留 N 天。
#
# mysql_data 是 docker named volume，`docker compose down -v` 一条命令即可全部抹掉，
# 而题库是几十个迁移加爬虫累积的成果，没有备份就等于没有退路。
#
# cron 示例（每天 03:17，错开整点避开其它任务）：
#   17 3 * * * /path/to/software-exam/deploy/backup-mysql.sh >> /var/log/software-exam-backup.log 2>&1
set -euo pipefail

CONTAINER="${MYSQL_CONTAINER:-software-exam-mysql}"
DATABASE="${MYSQL_DATABASE:-software_exam}"
BACKUP_DIR="${BACKUP_DIR:-/var/backups/software-exam}"
RETENTION_DAYS="${RETENTION_DAYS:-14}"

# 密码从环境变量读，不写进脚本 —— 脚本在 git 里，凭据不该跟着一起进版本控制。
# 建议放在 /etc/default/software-exam-backup 并由 cron 引入，或直接写进 crontab 行首。
if [[ -z "${MYSQL_ROOT_PASSWORD:-}" ]]; then
  echo "[backup] MYSQL_ROOT_PASSWORD 未设置，退出" >&2
  exit 1
fi

STAMP="$(date +%Y%m%d-%H%M%S)"
mkdir -p "$BACKUP_DIR"
TARGET="$BACKUP_DIR/${DATABASE}-${STAMP}.sql.gz"
TMP="$TARGET.partial"

echo "[backup] $(date -Iseconds) dumping $DATABASE -> $TARGET"

# --single-transaction 保证 InnoDB 一致性快照且不锁表；
# 密码经 stdin 传给容器内的 mysqldump，避免出现在宿主机的进程列表里。
if ! MYSQL_PWD="$MYSQL_ROOT_PASSWORD" docker exec -i \
  -e MYSQL_PWD \
  "$CONTAINER" \
  mysqldump -u root \
    --single-transaction \
    --quick \
    --routines \
    --triggers \
    --events \
    --default-character-set=utf8mb4 \
    "$DATABASE" | gzip -9 > "$TMP"; then
  rm -f "$TMP"
  echo "[backup] 失败：mysqldump 非零退出" >&2
  exit 1
fi

# gzip 完整性校验 + 非空检查：截断的备份比没有备份更危险，因为它看起来像有备份。
if ! gzip -t "$TMP" 2>/dev/null || [[ ! -s "$TMP" ]]; then
  rm -f "$TMP"
  echo "[backup] 失败：产物损坏或为空" >&2
  exit 1
fi

# 校验通过才落到最终文件名，保证目录里的每个 .sql.gz 都是可用备份
mv "$TMP" "$TARGET"
echo "[backup] 完成 $(du -h "$TARGET" | cut -f1)"

find "$BACKUP_DIR" -name "${DATABASE}-*.sql.gz" -type f -mtime "+$RETENTION_DAYS" -print -delete
find "$BACKUP_DIR" -name "${DATABASE}-*.sql.gz.partial" -type f -mtime +1 -delete
