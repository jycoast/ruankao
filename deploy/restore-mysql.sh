#!/usr/bin/env bash
# 从 backup-mysql.sh 的产物恢复数据库。
#
# 用法：MYSQL_ROOT_PASSWORD=xxx ./deploy/restore-mysql.sh /var/backups/software-exam/software_exam-20260731-031700.sql.gz
#
# 未演练过的备份不算备份。上线前至少跑一次本脚本恢复到临时库，确认能起来。
set -euo pipefail

ARCHIVE="${1:-}"
CONTAINER="${MYSQL_CONTAINER:-software-exam-mysql}"
DATABASE="${MYSQL_DATABASE:-software_exam}"

if [[ -z "$ARCHIVE" ]]; then
  echo "用法: $0 <备份文件.sql.gz>" >&2
  exit 1
fi
if [[ ! -f "$ARCHIVE" ]]; then
  echo "[restore] 找不到备份文件：$ARCHIVE" >&2
  exit 1
fi
if [[ -z "${MYSQL_ROOT_PASSWORD:-}" ]]; then
  echo "[restore] MYSQL_ROOT_PASSWORD 未设置，退出" >&2
  exit 1
fi
if ! gzip -t "$ARCHIVE" 2>/dev/null; then
  echo "[restore] 备份文件损坏：$ARCHIVE" >&2
  exit 1
fi

# 恢复会覆盖目标库的现有数据，非交互场景用 FORCE=1 跳过确认
if [[ "${FORCE:-0}" != "1" ]]; then
  read -rp "将把 $ARCHIVE 恢复进 $CONTAINER 的 $DATABASE 库，现有数据会被覆盖。确认？[y/N] " ans
  [[ "$ans" == "y" || "$ans" == "Y" ]] || { echo "已取消"; exit 1; }
fi

echo "[restore] $(date -Iseconds) restoring $ARCHIVE -> $DATABASE"
gunzip -c "$ARCHIVE" | MYSQL_PWD="$MYSQL_ROOT_PASSWORD" docker exec -i -e MYSQL_PWD \
  "$CONTAINER" mysql -u root --default-character-set=utf8mb4 "$DATABASE"
echo "[restore] 完成"
