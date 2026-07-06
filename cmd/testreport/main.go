package main

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const baseURL = "http://localhost:${PORT}"

func main() {
	port := os.Getenv("SERVER_PORT")
	if port == "" {
		port = "8080"
	}
	base := strings.Replace(baseURL, "${PORT}", port, 1)

	phone := os.Getenv("GOV_PHONE")
	pass := os.Getenv("GOV_PASS")
	if phone == "" {
		phone = "13900000003"
	}
	if pass == "" {
		pass = "test123"
	}

	fmt.Println("=== 1. 登录 ===")
	token := login(base, phone, pass)
	fmt.Printf("Token: %s...\n", token[:min(30, len(token))])

	fmt.Println("\n=== 2. 创建会话 ===")
	sessionID := createSession(base, token, "数据分析报告测试")
	fmt.Printf("Session ID: %d\n", sessionID)

	prompt := "生成一份合肥地区企业行业分布和规模分布的数据分析报告，要求包含图表。"
	fmt.Printf("\n=== 3. 发送请求: %q ===\n", prompt)

	markdown, charts := sseStream(base, token, sessionID, prompt)
	markdown = cleanBlank(markdown)

	fname := fmt.Sprintf("report_%s.md", time.Now().Format("20060102_150405"))
	os.WriteFile(fname, []byte(markdown), 0644)
	fmt.Printf("\n=== 报告已保存到 %s (%d chars) ===\n", fname, len(markdown))
	fmt.Printf("=== 图表数量: %d ===\n", len(charts))
}

func login(base, phone, pass string) string {
	body, _ := json.Marshal(map[string]string{
		"password": pass,
		"role":     "government",
		"phone":    phone,
	})
	resp, err := http.Post(base+"/api/v1/auth/login", "application/json", bytes.NewReader(body))
	die(err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var r struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Token string `json:"token"`
		} `json:"data"`
	}
	json.Unmarshal(raw, &r)
	if r.Data.Token == "" {
		fmt.Println("登录失败，尝试注册...")
		regBody, _ := json.Marshal(map[string]string{
			"password":      pass,
			"role":          "government",
			"phone":         phone,
			"gov_name":      "报告测试政务",
			"gov_department": "数据分析科",
		})
		resp2, err := http.Post(base+"/api/v1/auth/register", "application/json", bytes.NewReader(regBody))
		if err != nil {
			fmt.Println("Register failed")
			os.Exit(1)
		}
		defer resp2.Body.Close()
		json.NewDecoder(resp2.Body).Decode(&r)
	}
	return r.Data.Token
}

func createSession(base, token, title string) uint {
	body, _ := json.Marshal(map[string]string{"title": title})
	req, _ := http.NewRequest("POST", base+"/api/v1/chat/sessions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	die(err)
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)

	var r struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			ID uint `json:"id"`
		} `json:"data"`
	}
	json.Unmarshal(raw, &r)
	if r.Code != 0 {
		fmt.Printf("CreateSession error: code=%d msg=%s\n", r.Code, r.Message)
		os.Exit(1)
	}
	return r.Data.ID
}

func sseStream(base, token string, sessionID uint, prompt string) (string, []string) {
	body, _ := json.Marshal(map[string]any{
		"content": prompt,
		"state":   map[string]any{},
	})
	url := fmt.Sprintf("%s/api/v1/chat/sessions/%d/messages", base, sessionID)
	req, _ := http.NewRequest("POST", url, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := http.DefaultClient.Do(req)
	die(err)
	defer resp.Body.Close()

	var markdown strings.Builder
	var charts []string
	reader := bufio.NewReader(resp.Body)

	for {
		line, err := reader.ReadString('\n')
		if err == io.EOF {
			break
		}
		if err != nil {
			fmt.Printf("read error: %v\n", err)
			break
		}
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := line[6:]

		var evt struct {
			Type string          `json:"type"`
			Data json.RawMessage `json:"data"`
		}
		if err := json.Unmarshal([]byte(data), &evt); err != nil {
			continue
		}

		switch evt.Type {
		case "reply":
			var reply string
			json.Unmarshal(evt.Data, &reply)
			markdown.WriteString(reply)
			fmt.Print(".")
		case "done":
			fmt.Println()
		case "error":
			fmt.Printf("\n[ERROR] %s\n", string(evt.Data))
		case "tool_result":
			var tr struct {
				Result string `json:"result"`
				Tool   string `json:"tool"`
			}
			json.Unmarshal(evt.Data, &tr)
			if tr.Tool == "generate_report" {
				var mr struct {
					Markdown string `json:"markdown"`
				}
				if json.Unmarshal([]byte(tr.Result), &mr) == nil && mr.Markdown != "" {
					markdown.Reset()
					markdown.WriteString(mr.Markdown)
					imgCount := strings.Count(mr.Markdown, "![")
					fmt.Printf("R(%d)", imgCount)
				}
			}
		case "tool_call":
			var calls []struct {
				Function struct{ Name string } `json:"function"`
			}
			json.Unmarshal(evt.Data, &calls)
			for _, c := range calls {
				fmt.Printf("\n[TOOL] %s ", c.Function.Name)
			}
		case "report_start":
			fmt.Println("\n[REPORT] start")
		case "report_progress":
			var progress map[string]any
			json.Unmarshal(evt.Data, &progress)
			phase, _ := progress["phase"].(string)
			if phase == "summarizer" {
				fmt.Println("[REPORT] summarize")
			} else {
				fmt.Printf("[REPORT] %v/%v\n", progress["current"], progress["total"])
			}
		case "report_done":
			fmt.Println("[REPORT] done")
		}
	}

	if markdown.Len() == 0 {
		return "", nil
	}
	for _, line := range strings.Split(markdown.String(), "\n") {
		if strings.HasPrefix(line, "```mermaid") {
			charts = append(charts, line)
		}
	}
	return markdown.String(), charts
}

func cleanBlank(md string) string {
	md = strings.TrimSpace(md)
	if idx := strings.Index(md, "# "); idx >= 0 {
		md = md[idx:]
	}
	// 去掉末尾 --- 后的闲聊
	lines := strings.Split(md, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) == "---" {
			hasStructure := false
			for _, l := range lines[i+1:] {
				t := strings.TrimSpace(l)
				if strings.HasPrefix(t, "#") || strings.HasPrefix(t, "```") || strings.HasPrefix(t, "|") {
					hasStructure = true
					break
				}
			}
			if !hasStructure {
				lines = lines[:i]
			}
			break
		}
	}
	return strings.TrimSpace(strings.Join(lines, "\n"))
}

func die(err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "FATAL: %v\n", err)
		os.Exit(1)
	}
}
