package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/polesnet/bw-guardian/internal/config"
)

func cmdWhitelist(args []string) {
	if os.Getuid() != 0 {
		fmt.Fprintln(os.Stderr, "请使用 root 权限运行")
		os.Exit(1)
	}

	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "用法: bw-guardian whitelist <list|add|remove> [uuid]")
		os.Exit(1)
	}

	cfg := config.Load()
	file := cfg.WhitelistFile

	switch args[0] {
	case "list":
		whitelistList(file)
	case "add":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法: bw-guardian whitelist add <uuid>")
			os.Exit(1)
		}
		whitelistAdd(file, args[1])
	case "remove":
		if len(args) < 2 {
			fmt.Fprintln(os.Stderr, "用法: bw-guardian whitelist remove <uuid>")
			os.Exit(1)
		}
		whitelistRemove(file, args[1])
	default:
		fmt.Fprintf(os.Stderr, "未知子命令: %s\n", args[0])
		os.Exit(1)
	}
}

func whitelistList(file string) {
	f, err := os.Open(file)
	if err != nil {
		fmt.Println("白名单为空")
		return
	}
	defer f.Close()
	count := 0
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			fmt.Println(line)
			count++
		}
	}
	if count == 0 {
		fmt.Println("白名单为空")
	}
}

func whitelistAdd(file, uuid string) {
	// Check if already exists
	lines := readWhitelist(file)
	for _, l := range lines {
		if l == uuid {
			fmt.Printf("%s 已在白名单中\n", uuid)
			return
		}
	}
	f, err := os.OpenFile(file, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "写入失败: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()
	fmt.Fprintln(f, uuid)
	fmt.Printf("已添加: %s\n", uuid)
}

func whitelistRemove(file, uuid string) {
	lines := readWhitelist(file)
	filtered := lines[:0]
	found := false
	for _, l := range lines {
		if l == uuid {
			found = true
		} else {
			filtered = append(filtered, l)
		}
	}
	if !found {
		fmt.Printf("%s 不在白名单中\n", uuid)
		return
	}
	content := strings.Join(filtered, "\n")
	if len(filtered) > 0 {
		content += "\n"
	}
	if err := os.WriteFile(file, []byte(content), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "写入失败: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("已移除: %s\n", uuid)
}

func readWhitelist(file string) []string {
	f, err := os.Open(file)
	if err != nil {
		return nil
	}
	defer f.Close()
	var lines []string
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}
