// dbctl 是本地联调小工具：建库 / 跑只读查询。
// 用法：
//   go run ./tools/dbctl -create flow_dev
//   go run ./tools/dbctl -db fastarch_dev -query "SELECT id,code FROM t_application"
package main

import (
	"database/sql"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/go-sql-driver/mysql"
	_ "github.com/go-sql-driver/mysql"
)

func main() {
	dsn := flag.String("dsn", "root:mysql12345678@tcp(127.0.0.1:3306)/fastarch_dev?charset=utf8mb4&parseTime=true", "带库名的 DSN")
	dbName := flag.String("db", "", "覆盖库名（建库用）")
	create := flag.String("create", "", "要创建的库名")
	query := flag.String("query", "", "要执行的查询 SQL")
	flag.Parse()

	cfg, err := mysql.ParseDSN(*dsn)
	if err != nil {
		die("解析 DSN 失败: %v", err)
	}

	if *create != "" {
		cfg.DBName = ""
		conn, err := sql.Open("mysql", cfg.FormatDSN())
		if err != nil {
			die("连接 MySQL 失败: %v", err)
		}
		defer conn.Close()
		stmt := fmt.Sprintf("CREATE DATABASE IF NOT EXISTS `%s` DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_0900_ai_ci", *create)
		if _, err := conn.Exec(stmt); err != nil {
			die("建库失败: %v", err)
		}
		fmt.Printf("已确保库存在: %s\n", *create)
		return
	}

	if *dbName != "" {
		cfg.DBName = *dbName
	}
	if *query == "" {
		die("需要 -create 或 -query")
	}
	conn, err := sql.Open("mysql", cfg.FormatDSN())
	if err != nil {
		die("连接失败: %v", err)
	}
	defer conn.Close()

	rows, err := conn.Query(*query)
	if err != nil {
		die("查询失败: %v", err)
	}
	defer rows.Close()
	cols, _ := rows.Columns()
	fmt.Println(strings.Join(cols, " | "))
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			die("扫描失败: %v", err)
		}
		parts := make([]string, len(cols))
		for i, v := range vals {
			switch t := v.(type) {
			case []byte:
				parts[i] = string(t)
			case nil:
				parts[i] = "NULL"
			default:
				parts[i] = fmt.Sprint(t)
			}
		}
		fmt.Println(strings.Join(parts, " | "))
	}
}

func die(format string, a ...any) {
	fmt.Fprintf(os.Stderr, format+"\n", a...)
	os.Exit(1)
}
