// migrate 按文件名顺序执行 migrations/*.sql，并把已执行的文件记到 schema_migrations 表里，
// 重复运行时自动跳过。此前 54 个迁移全靠手工执行、无版本记录，重跑会因重复建列/重复插入而报错。
//
//	go run ./cmd/migrate -status     查看已执行 / 待执行
//	go run ./cmd/migrate             执行所有待执行迁移
//	go run ./cmd/migrate -dry-run    只打印将要执行的文件
//	go run ./cmd/migrate -baseline -through 053_paper_source.sql
//	    把 053 及之前的迁移标记为「已执行」但不真正执行。现网库此前是手工执行的，
//	    首次接入必须先 baseline 到最后一个手工执行过的文件，否则会把老迁移重跑一遍。
//
// 忽略 *_rollback_*.sql 与非 .sql 文件；回滚需手工执行对应的 rollback 脚本。
package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/hex"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"software-exam/config"

	_ "github.com/go-sql-driver/mysql"
)

const createVersionTable = `
CREATE TABLE IF NOT EXISTS schema_migrations (
  version    VARCHAR(191) NOT NULL,
  checksum   CHAR(32)     NOT NULL,
  applied_at DATETIME     NOT NULL,
  PRIMARY KEY (version)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;`

func main() {
	var (
		dir        = flag.String("dir", "migrations", "迁移文件目录")
		cfgPath    = flag.String("config", "config/config.yaml", "配置文件路径")
		statusOnly = flag.Bool("status", false, "只显示状态")
		baseline   = flag.Bool("baseline", false, "把迁移标记为已执行，不实际执行")
		through    = flag.String("through", "", "配合 -baseline：只标记到该文件为止（含），留空表示全部")
		dryRun     = flag.Bool("dry-run", false, "只打印将要执行的迁移")
	)
	flag.Parse()

	if err := config.Load(*cfgPath); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}
	c := config.AppConfig.Database
	// multiStatements 让一个 .sql 文件可以整体提交，免去自己切分语句时踩到字符串里的分号
	// maxAllowedPacket 默认 4MB (go-sql-driver 客户端侧), 大迁移(094 网工 5.4MB)会
	// 报 "packet for query is too large"; 服务器侧 max_allowed_packet 64MB, 故放大客户端限制。
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&multiStatements=true&maxAllowedPacket=67108864",
		c.User, c.Password, c.Host, c.Port, c.DBName)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	defer db.Close()
	if err := db.Ping(); err != nil {
		log.Fatalf("连接数据库失败: %v", err)
	}
	if _, err := db.Exec(createVersionTable); err != nil {
		log.Fatalf("创建 schema_migrations 失败: %v", err)
	}

	files, err := listMigrations(*dir)
	if err != nil {
		log.Fatalf("读取迁移目录失败: %v", err)
	}
	applied, err := loadApplied(db)
	if err != nil {
		log.Fatalf("读取已执行记录失败: %v", err)
	}

	if *statusOnly {
		printStatus(*dir, files, applied)
		return
	}

	pending := make([]string, 0, len(files))
	for _, f := range files {
		if _, ok := applied[f]; !ok {
			pending = append(pending, f)
		}
	}
	if *baseline && *through != "" {
		cut := -1
		for i, f := range pending {
			if f == *through {
				cut = i
				break
			}
		}
		if cut < 0 {
			log.Fatalf("-through %s 不在待执行列表里（可能已标记过或文件名有误）", *through)
		}
		pending = pending[:cut+1]
	}
	if len(pending) == 0 {
		fmt.Println("没有待执行的迁移")
		return
	}

	if *dryRun {
		fmt.Printf("待执行 %d 个迁移：\n", len(pending))
		for _, f := range pending {
			fmt.Println("  " + f)
		}
		return
	}

	for _, f := range pending {
		body, err := os.ReadFile(filepath.Join(*dir, f))
		if err != nil {
			log.Fatalf("读取 %s 失败: %v", f, err)
		}
		sum := md5.Sum(body)
		checksum := hex.EncodeToString(sum[:])

		if *baseline {
			if err := markApplied(db, f, checksum); err != nil {
				log.Fatalf("标记 %s 失败: %v", f, err)
			}
			fmt.Printf("baseline  %s\n", f)
			continue
		}

		fmt.Printf("执行 %s ...", f)
		if _, err := db.Exec(string(body)); err != nil {
			// 不继续跑后面的迁移：顺序有依赖，带着半截状态往下走只会更难收拾
			fmt.Println(" 失败")
			log.Fatalf("%s 执行失败: %v", f, err)
		}
		if err := markApplied(db, f, checksum); err != nil {
			log.Fatalf("%s 已执行但记录失败（请手工插入 schema_migrations）: %v", f, err)
		}
		fmt.Println(" 完成")
	}
	fmt.Printf("共处理 %d 个迁移\n", len(pending))
}

// listMigrations 返回按文件名排序的迁移文件。数据库迁移靠文件名前缀的数字保证顺序，
// 因此新增迁移必须继续沿用 NNN_ 前缀。
func listMigrations(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasSuffix(name, ".sql") || strings.Contains(name, "_rollback_") {
			continue
		}
		if len(name) < 4 || name[0] < '0' || name[0] > '9' {
			continue // restore_*.sql 等一次性修数据脚本不纳入版本管理
		}
		out = append(out, name)
	}
	sort.Strings(out)
	return out, nil
}

func loadApplied(db *sql.DB) (map[string]string, error) {
	rows, err := db.Query("SELECT version, checksum FROM schema_migrations")
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := map[string]string{}
	for rows.Next() {
		var v, sum string
		if err := rows.Scan(&v, &sum); err != nil {
			return nil, err
		}
		m[v] = sum
	}
	return m, rows.Err()
}

func markApplied(db *sql.DB, version, checksum string) error {
	_, err := db.Exec(
		"INSERT INTO schema_migrations (version, checksum, applied_at) VALUES (?, ?, NOW()) "+
			"ON DUPLICATE KEY UPDATE checksum = VALUES(checksum)",
		version, checksum)
	return err
}

func printStatus(dir string, files []string, applied map[string]string) {
	pending := 0
	for _, f := range files {
		sum, ok := applied[f]
		if !ok {
			pending++
			fmt.Printf("[ ] %s\n", f)
			continue
		}
		// 校验和不一致说明已执行的迁移文件事后被改过，线上库不会自动补上这段改动
		mark := "x"
		if body, err := os.ReadFile(filepath.Join(dir, f)); err == nil {
			s := md5.Sum(body)
			if hex.EncodeToString(s[:]) != sum {
				mark = "!"
			}
		}
		fmt.Printf("[%s] %s\n", mark, f)
	}
	fmt.Printf("\n共 %d 个，已执行 %d，待执行 %d（! = 文件在执行后被修改过）\n",
		len(files), len(files)-pending, pending)
}
