// Package mcpserver 提供 flow 工作台的 MCP stdio / HTTP 服务（单个服务、16 个工具）。
// 它本身不直连数据库、不复制判权逻辑，只把 agent 的调用转成 flow HTTP 接口，
// 因此"谁能看到什么"始终由 IAM + flow 服务端裁决。
package mcpserver

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"flow-service/conf"
)

// Run 是 `flow-service mcp` 子命令入口：从 stdin 读 JSON-RPC，向 stdout 写结果。
func Run(ctx context.Context, cfg *conf.Config) error {
	token := resolveToken()
	if token == "" {
		return errors.New("缺少登录态：请在你自己的环境变量 FLOW_TOKEN 里放个人 token，或写入 ~/.fastarch/flow.token（可用 FLOW_TOKEN_FILE 改路径）；服务端配置不接受任何人的 token")
	}
	s := &server{
		base:  "http://" + cfg.Http.AddrWithPort(),
		token: token,
		http:  &http.Client{Timeout: 20 * time.Second},
	}
	return s.serve(ctx, os.Stdin, os.Stdout)
}

// resolveToken 只从调用者自己的环境取值：环境变量 FLOW_TOKEN → ~/.fastarch/flow.token。
// 刻意不读服务端配置——token 标识的是某个具体用户，不该落在一份共享部署配置里。
func resolveToken() string {
	if v := strings.TrimSpace(os.Getenv("FLOW_TOKEN")); v != "" {
		return v
	}
	path := strings.TrimSpace(os.Getenv("FLOW_TOKEN_FILE"))
	if path == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		path = filepath.Join(home, ".fastarch", "flow.token")
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(b))
}

type server struct {
	base  string
	token string
	http  *http.Client
	// mu 保护 stdout 写入：请求改为并发处理后，多个 goroutine 同时写会把 JSON 行拼坏，
	// 上层就会报 "invalid raw MCP result"。
	mu  sync.Mutex
	enc *json.Encoder
}

// ---- JSON-RPC 线协议 ----

type rpcRequest struct {
	Jsonrpc string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type rpcError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type rpcResponse struct {
	Jsonrpc string          `json:"jsonrpc"`
	Id      json.RawMessage `json:"id,omitempty"`
	Result  any             `json:"result,omitempty"`
	Error   *rpcError       `json:"error,omitempty"`
}

func (s *server) serve(ctx context.Context, in io.Reader, out io.Writer) error {
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	s.enc = json.NewEncoder(out)

	var wg sync.WaitGroup
	for sc.Scan() {
		// Scanner 的缓冲会被下一次 Scan 覆盖，这里必须先拷贝再交给 goroutine。
		line := append([]byte(nil), bytes.TrimSpace(sc.Bytes())...)
		if len(line) == 0 {
			continue
		}
		var req rpcRequest
		if err := json.Unmarshal(line, &req); err != nil {
			_ = s.write(rpcResponse{Jsonrpc: "2.0", Error: &rpcError{Code: -32700, Message: "parse error"}})
			continue
		}
		// 并发处理：一个慢调用不能把后面排队的调用拖超时（上层超时会报 invalid raw MCP result）。
		wg.Add(1)
		go func(req rpcRequest) {
			defer wg.Done()
			resp := s.handle(ctx, &req)
			if resp == nil {
				return // 通知类消息不回复
			}
			if err := s.write(*resp); err != nil {
				return
			}
		}(req)
	}
	wg.Wait()
	return sc.Err()
}

// write 串行化 stdout 写入，保证每条响应是完整的一行 JSON。
func (s *server) write(resp rpcResponse) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.enc.Encode(resp)
}

func (s *server) handle(ctx context.Context, req *rpcRequest) *rpcResponse {
	// 通知（无 id）不回复
	hasID := len(req.Id) > 0
	switch req.Method {
	case "initialize":
		return &rpcResponse{Jsonrpc: "2.0", Id: req.Id, Result: map[string]any{
			"protocolVersion": "2024-11-05",
			"capabilities":    map[string]any{"tools": map[string]any{"listChanged": false}},
			"serverInfo":      map[string]any{"name": "flow-workbench", "version": "1.0.0"},
		}}
	case "notifications/initialized", "initialized", "notifications/cancelled":
		return nil
	case "ping":
		return &rpcResponse{Jsonrpc: "2.0", Id: req.Id, Result: map[string]any{}}
	case "tools/list":
		return &rpcResponse{Jsonrpc: "2.0", Id: req.Id, Result: map[string]any{"tools": tools()}}
	case "tools/call":
		var p struct {
			Name      string          `json:"name"`
			Arguments json.RawMessage `json:"arguments"`
		}
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return errResp(req.Id, -32602, "invalid params")
		}
		args := map[string]any{}
		if len(p.Arguments) > 0 {
			_ = json.Unmarshal(p.Arguments, &args)
		}
		text, err := s.callTool(ctx, p.Name, args)
		if err != nil {
			return &rpcResponse{Jsonrpc: "2.0", Id: req.Id, Result: map[string]any{
				"content": []map[string]any{{"type": "text", "text": err.Error()}},
				"isError": true,
			}}
		}
		return &rpcResponse{Jsonrpc: "2.0", Id: req.Id, Result: map[string]any{
			"content": []map[string]any{{"type": "text", "text": text}},
			"isError": false,
		}}
	default:
		if !hasID {
			return nil
		}
		return errResp(req.Id, -32601, "method not found: "+req.Method)
	}
}

func errResp(id json.RawMessage, code int, msg string) *rpcResponse {
	return &rpcResponse{Jsonrpc: "2.0", Id: id, Error: &rpcError{Code: code, Message: msg}}
}

// ---- 工具定义（7 个） ----

func tools() []map[string]any {
	return []map[string]any{
		{
			"name":        "whoami",
			"description": "确认我是谁：账号、当前工作角色、可担任的角色、当前角色的可开工/被阻塞任务数与待我回答的问题数。问题卡住时先调它。",
			"inputSchema": map[string]any{"type": "object", "properties": map[string]any{}},
		},
		{
			"name":        "switch_role",
			"description": "切换当前工作角色（一人多角色）。切完才看得到该角色的任务；不能切到未持有的角色。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"role_code": map[string]any{"type": "string", "description": "角色 code 或中文名，如 dev / 开发 / arch"},
				},
				"required": []string{"role_code"},
			},
		},
		{
			"name":        "check_ready",
			"description": "判断任务能不能开工。传 task_id 查单条（返回 state=ready/blocked 与阻塞问题清单）；不传则列出当前角色所有 ready 任务。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":   map[string]any{"type": "string", "description": "任务 id，可空"},
					"page_size": map[string]any{"type": "integer", "description": "不传 task_id 时返回条数，默认 20"},
				},
			},
		},
		{
			"name":        "list_tasks",
			"description": "列任务：scope=mine（我当前角色名下的任务）/upstream（上游角色任务，只读，用来建子任务）/created（我发起的，跨角色）/all（全部，仅 boss）；state 可筛 ready/blocked/done/discarded。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"scope":     map[string]any{"type": "string", "enum": []string{"mine", "upstream", "created", "all"}, "description": "默认 mine"},
					"state":     map[string]any{"type": "string", "description": "ready / blocked / done / discarded，可空"},
					"page_size": map[string]any{"type": "integer", "description": "默认 20"},
				},
			},
		},
		{
			"name":        "my_questions",
			"description": "待我当前角色回答的 open 问题（自带 AI 预填选项）。scope=raised 时改为看我自己提出的问题。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"scope":  map[string]any{"type": "string", "enum": []string{"mine", "raised"}, "description": "默认 mine"},
					"status": map[string]any{"type": "string", "enum": []string{"open", "answered", "all"}, "description": "默认 open；all = 未闭环+已闭环"},
				},
			},
		},
		{
			"name":        "answer_question",
			"description": "闭环一个问题：选一个预填选项或写结论。answer_kind=evidence 的问题必须带 evidence（commit/工单号/部署单/文档链接），否则返回业务码 3001001；证据形态不合法返回 3001023。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question_id": map[string]any{"type": "string"},
					"option_id":   map[string]any{"type": "string", "description": "预填选项 id（A/B/C）"},
					"text":        map[string]any{"type": "string", "description": "补充说明；没有选项时必填"},
					"evidence":    map[string]any{"type": "string", "description": "证据：commit / 工单号 / 部署单 / 文档链接"},
				},
				"required": []string{"question_id"},
			},
		},
		{
			"name":        "reopen_question",
			"description": "把已闭环问题打回：仅原提问人或老板角色可重开。重开后任务自动重新 blocked，reopen_count+1，并写 question.reopen 留痕。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question_id": map[string]any{"type": "string", "description": "要重开的问题 id"},
					"reason":      map[string]any{"type": "string", "description": "为什么打回：哪条答案不可靠、期望补什么，最长 512"},
				},
				"required": []string{"question_id"},
			},
		},
		{
			"name":        "raise_question",
			"description": "抛出一个阻塞问题给上游角色，任务立刻变 blocked（无需改任务表）。options 由 agent 预填候选答案，人只需选。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":      map[string]any{"type": "string"},
					"to_role_code": map[string]any{"type": "string", "description": "问谁：analyst / pm / arch / dev / qa / ops ..."},
					"body":         map[string]any{"type": "string", "description": "问题描述，越具体越好"},
					"answer_kind":  map[string]any{"type": "string", "enum": []string{"decision", "evidence"}, "description": "默认 decision"},
					"options": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id":    map[string]any{"type": "string"},
								"label": map[string]any{"type": "string"},
								"rec":   map[string]any{"type": "boolean"},
							},
							"required": []string{"id", "label"},
						},
					},
				},
				"required": []string{"task_id", "to_role_code", "body"},
			},
		},
		{
			"name":        "escalate_question",
			"description": "把一个别人抛给你的问题继续上报给更高角色（架构定不了就报老板）。会生成子问题并挂父子链，最多 3 层。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"question_id":  map[string]any{"type": "string", "description": "要上报的问题 id（必须是发给当前角色的 open 问题）"},
					"to_role_code": map[string]any{"type": "string", "description": "上报给谁：boss / qa / ops ..."},
					"body":         map[string]any{"type": "string", "description": "为什么要上报、卡在哪，越具体越好"},
					"options": map[string]any{
						"type": "array",
						"items": map[string]any{
							"type": "object",
							"properties": map[string]any{
								"id":    map[string]any{"type": "string"},
								"label": map[string]any{"type": "string"},
								"rec":   map[string]any{"type": "boolean"},
							},
							"required": []string{"id", "label"},
						},
						"description": "不传则沿用父问题的候选答案",
					},
				},
				"required": []string{"question_id", "to_role_code", "body"},
			},
		},
		{
			"name":        "create_task",
			"description": "在当前人 + 当前角色名下建一条自己的任务。下游角色基于上游父任务建子任务：先 list_tasks(scope=upstream) 找父任务，再用 parent_task_id 建自己的任务。父任务有未闭环问题时返回 3001033；不允许把任务改派给别的角色。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"title":                   map[string]any{"type": "string", "description": "任务标题"},
					"brief":                   map[string]any{"type": "string", "description": "一句话说明做什么、交付什么"},
					"parent_task_id":          map[string]any{"type": "string", "description": "父任务 id；为空表示顶层任务（老板目标）。父任务必须属于上游角色。"},
					"required_artifact_types": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "开工所需物料类型，可空；dev/qa/ops 有默认，规划类为空"},
				},
				"required": []string{"title"},
			},
		},
		{
			"name":        "close_task",
			"description": "老板关闭目标/兜底完成；规划类必须先 submit_task，实施类必须先 start_task 再 finish_task。",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"task_id": map[string]any{"type": "string"}},
				"required":   []string{"task_id"},
			},
		},
		{
			"name":        "discard_task",
			"description": "作废自己名下的任务（不再需要做了）。会同时丢掉该任务下未闭环的问题。",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"task_id": map[string]any{"type": "string"}},
				"required":   []string{"task_id"},
			},
		},
		{
			"name":        "start_task",
			"description": "实施类开工：校验并读取上游规划物料（Markdown），返回工作包并把任务改为进行中；缺物料返回 3001034。",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"task_id": map[string]any{"type": "string"}},
				"required":   []string{"task_id"},
			},
		},
		{
			"name":        "submit_task",
			"description": "规划类提交 Markdown 物料（需求规格/功能设计/技术方案等），任务进入已提交，等待下游或老板确认。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":       map[string]any{"type": "string"},
					"artifact_type": map[string]any{"type": "string", "description": "例如 requirement_spec / feature_design / tech_plan / acceptance_criteria"},
					"title":         map[string]any{"type": "string", "description": "物料标题，可空，默认任务标题"},
					"content_md":    map[string]any{"type": "string", "description": "Markdown 正文"},
				},
				"required": []string{"task_id", "artifact_type", "content_md"},
			},
		},
		{
			"name":        "finish_task",
			"description": "实施类完成任务：提交实施产物（code_change/test_report/deployment_record），任务直接完成；未开工返回 3001036。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"task_id":       map[string]any{"type": "string"},
					"artifact_type": map[string]any{"type": "string", "description": "可空，按角色默认"},
					"title":         map[string]any{"type": "string", "description": "产物标题，可空"},
					"content_md":    map[string]any{"type": "string", "description": "Markdown 正文"},
				},
				"required": []string{"task_id", "content_md"},
			},
		},
		{
			"name":        "accept_task",
			"description": "确认规划类物料：下游角色或老板调用，任务完成、物料归档；不能自己确认自己的提交。",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"task_id": map[string]any{"type": "string"}},
				"required":   []string{"task_id"},
			},
		},
		{
			"name":        "work_package",
			"description": "只读查看某任务的开工工作包：任务 + 上游 Markdown 物料 + 缺失物料类型，不改变状态。",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"task_id": map[string]any{"type": "string"}},
				"required":   []string{"task_id"},
			},
		},
		{
			"name":        "list_artifacts",
			"description": "列某任务的全部 Markdown 物料（含版本、状态、hash）。",
			"inputSchema": map[string]any{
				"type":       "object",
				"properties": map[string]any{"task_id": map[string]any{"type": "string"}},
				"required":   []string{"task_id"},
			},
		},
		{
			"name":        "list_members",
			"description": "看全部成员各自能担任哪些角色（只有当前角色是 boss 才看得到）。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"keyword":   map[string]any{"type": "string", "description": "按姓名过滤，可空"},
					"page_size": map[string]any{"type": "integer", "description": "默认 20"},
				},
			},
		},
		{
			"name":        "assign_roles",
			"description": "老板给某个成员设定能担任哪些角色（整体替换，传全集）。只有当前角色是 boss 才能调用；给自己加角色也走这个。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"member_id":  map[string]any{"type": "string", "description": "成员 id（list_members 里查）"},
					"role_codes": map[string]any{"type": "array", "items": map[string]any{"type": "string"}, "description": "角色 code 全集，如 [\"boss\",\"pm\",\"dev\"]"},
				},
				"required": []string{"member_id", "role_codes"},
			},
		},
		{
			"name":        "history",
			"description": "留痕：谁在什么时候把哪个任务/问题改成了什么。可按 entity（task/question/member）与 entity_id 过滤。",
			"inputSchema": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"entity":    map[string]any{"type": "string", "enum": []string{"task", "question", "member"}},
					"entity_id": map[string]any{"type": "string"},
					"limit":     map[string]any{"type": "integer", "description": "默认 20，上限 100"},
				},
			},
		},
	}
}

// ---- 工具实现（全部转 HTTP，判权仍在服务端） ----

func (s *server) callTool(ctx context.Context, name string, args map[string]any) (string, error) {
	switch name {
	case "whoami":
		body, err := s.get(ctx, "/flow/me")
		if err != nil {
			return "", err
		}
		return whoamiText(body), nil

	case "switch_role":
		code := argStr(args, "role_code")
		if code == "" {
			return "", errors.New("role_code 必填")
		}
		body, err := s.post(ctx, "/flow/me/role", map[string]any{"roleCode": code})
		if err != nil {
			return "", err
		}
		return whoamiText(body), nil

	case "check_ready":
		if id := argInt64(args, "task_id"); id > 0 {
			body, err := s.get(ctx, fmt.Sprintf("/flow/tasks/%d", id))
			if err != nil {
				return "", err
			}
			return pretty(body), nil
		}
		size := argInt64(args, "page_size")
		if size <= 0 {
			size = 20
		}
		body, err := s.get(ctx, fmt.Sprintf("/flow/tasks?state=ready&page_size=%d", size))
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "list_tasks":
		size := argInt64(args, "page_size")
		if size <= 0 {
			size = 20
		}
		path := fmt.Sprintf("/flow/tasks?page_size=%d", size)
		if scope := argStr(args, "scope"); scope != "" {
			path += "&scope=" + scope
		}
		if state := argStr(args, "state"); state != "" {
			path += "&state=" + state
		}
		body, err := s.get(ctx, path)
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "my_questions":
		scope := argStr(args, "scope")
		if scope == "" {
			scope = "mine"
		}
		status := argStr(args, "status")
		if status == "" {
			status = "open"
		}
		body, err := s.get(ctx, fmt.Sprintf("/flow/questions?scope=%s&status=%s&page_size=30", scope, status))
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "answer_question":
		id := argInt64(args, "question_id")
		if id <= 0 {
			return "", errors.New("question_id 必填")
		}
		payload := map[string]any{
			"optionId": argStr(args, "option_id"),
			"text":     argStr(args, "text"),
			"evidence": argStr(args, "evidence"),
		}
		body, err := s.post(ctx, fmt.Sprintf("/flow/questions/%d/answer", id), payload)
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "reopen_question":
		id := argInt64(args, "question_id")
		if id <= 0 {
			return "", errors.New("question_id 必填")
		}
		body, err := s.post(ctx, fmt.Sprintf("/flow/questions/%d/reopen", id),
			map[string]any{"reason": argStr(args, "reason")})
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "raise_question":
		taskID := argInt64(args, "task_id")
		if taskID <= 0 {
			return "", errors.New("task_id 必填")
		}
		payload := map[string]any{
			"taskId":     taskID,
			"toRoleCode": argStr(args, "to_role_code"),
			"body":       argStr(args, "body"),
			"answerKind": argStr(args, "answer_kind"),
		}
		if raw, ok := args["options"]; ok {
			payload["options"] = normalizeOptions(raw)
		}
		body, err := s.post(ctx, "/flow/questions", payload)
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "escalate_question":
		id := argInt64(args, "question_id")
		if id <= 0 {
			return "", errors.New("question_id 必填")
		}
		payload := map[string]any{
			"toRoleCode": argStr(args, "to_role_code"),
			"body":       argStr(args, "body"),
		}
		if raw, ok := args["options"]; ok {
			payload["options"] = normalizeOptions(raw)
		}
		body, err := s.post(ctx, fmt.Sprintf("/flow/questions/%d/escalate", id), payload)
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "create_task":
		title := argStr(args, "title")
		if title == "" {
			return "", errors.New("title 必填")
		}
		payload := map[string]any{
			"title": title,
			"brief": argStr(args, "brief"),
		}
		if parent := argStr(args, "parent_task_id"); parent != "" {
			payload["parentTaskId"] = parent
		}
		if types := argStrList(args, "required_artifact_types"); len(types) > 0 {
			payload["requiredArtifactTypes"] = types
		}
		body, err := s.post(ctx, "/flow/tasks", payload)
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "start_task":
		id := argInt64(args, "task_id")
		if id <= 0 {
			return "", errors.New("task_id 必填")
		}
		body, err := s.post(ctx, fmt.Sprintf("/flow/tasks/%d/start", id), map[string]any{})
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "submit_task":
		id := argInt64(args, "task_id")
		content := argStr(args, "content_md")
		artType := argStr(args, "artifact_type")
		if id <= 0 || content == "" || artType == "" {
			return "", errors.New("task_id、artifact_type、content_md 必填")
		}
		body, err := s.post(ctx, fmt.Sprintf("/flow/tasks/%d/submit", id), map[string]any{
			"artifactType": artType, "title": argStr(args, "title"), "contentMd": content,
		})
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "finish_task":
		id := argInt64(args, "task_id")
		content := argStr(args, "content_md")
		if id <= 0 || content == "" {
			return "", errors.New("task_id、content_md 必填")
		}
		body, err := s.post(ctx, fmt.Sprintf("/flow/tasks/%d/finish", id), map[string]any{
			"artifactType": argStr(args, "artifact_type"), "title": argStr(args, "title"), "contentMd": content,
		})
		if err != nil {
			return "", err
		}
		return closedText("已完成", body), nil

	case "accept_task":
		id := argInt64(args, "task_id")
		if id <= 0 {
			return "", errors.New("task_id 必填")
		}
		body, err := s.post(ctx, fmt.Sprintf("/flow/tasks/%d/accept", id), map[string]any{})
		if err != nil {
			return "", err
		}
		return closedText("已确认", body), nil

	case "work_package":
		id := argInt64(args, "task_id")
		if id <= 0 {
			return "", errors.New("task_id 必填")
		}
		body, err := s.get(ctx, fmt.Sprintf("/flow/tasks/%d/work-package", id))
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "list_artifacts":
		id := argInt64(args, "task_id")
		if id <= 0 {
			return "", errors.New("task_id 必填")
		}
		body, err := s.get(ctx, fmt.Sprintf("/flow/tasks/%d/artifacts", id))
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "close_task":
		id := argInt64(args, "task_id")
		if id <= 0 {
			return "", errors.New("task_id 必填")
		}
		body, err := s.post(ctx, fmt.Sprintf("/flow/tasks/%d/close", id), map[string]any{})
		if err != nil {
			return "", err
		}
		return closedText("已完成", body), nil

	case "discard_task":
		id := argInt64(args, "task_id")
		if id <= 0 {
			return "", errors.New("task_id 必填")
		}
		body, err := s.post(ctx, fmt.Sprintf("/flow/tasks/%d/discard", id), map[string]any{})
		if err != nil {
			return "", err
		}
		return closedText("已作废", body), nil

	case "list_members":
		size := argInt64(args, "page_size")
		if size <= 0 {
			size = 20
		}
		path := fmt.Sprintf("/flow/members?page_size=%d", size)
		if kw := argStr(args, "keyword"); kw != "" {
			path += "&keyword=" + url.QueryEscape(kw)
		}
		body, err := s.get(ctx, path)
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "assign_roles":
		id := argInt64(args, "member_id")
		if id <= 0 {
			return "", errors.New("member_id 必填")
		}
		codes := argStrList(args, "role_codes")
		if len(codes) == 0 {
			return "", errors.New("role_codes 必填（至少一个）")
		}
		body, err := s.post(ctx, fmt.Sprintf("/flow/members/%d/roles", id),
			map[string]any{"id": id, "roleCodes": codes})
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	case "history":
		limit := argInt64(args, "limit")
		if limit <= 0 {
			limit = 20
		}
		path := fmt.Sprintf("/flow/events?limit=%d", limit)
		if e := argStr(args, "entity"); e != "" {
			path += "&entity=" + e
		}
		if id := argInt64(args, "entity_id"); id > 0 {
			path += fmt.Sprintf("&entity_id=%d", id)
		}
		body, err := s.get(ctx, path)
		if err != nil {
			return "", err
		}
		return pretty(body), nil

	default:
		return "", fmt.Errorf("未知工具：%s", name)
	}
}

func normalizeOptions(raw any) []map[string]any {
	list, ok := raw.([]any)
	if !ok {
		return nil
	}
	out := make([]map[string]any, 0, len(list))
	for _, it := range list {
		m, ok := it.(map[string]any)
		if !ok {
			continue
		}
		id := argStr(m, "id")
		label := argStr(m, "label")
		if id == "" || label == "" {
			continue
		}
		out = append(out, map[string]any{"id": id, "label": label, "rec": argBool(m, "rec")})
	}
	return out
}

// ---- HTTP ----

func (s *server) get(ctx context.Context, path string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, s.base+path, nil)
	if err != nil {
		return nil, err
	}
	return s.do(req)
}

func (s *server) post(ctx context.Context, path string, payload any) ([]byte, error) {
	b, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.base+path, bytes.NewReader(b))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	return s.do(req)
}

func (s *server) do(req *http.Request) ([]byte, error) {
	req.Header.Set("Authorization", "Bearer "+s.token)
	req.Header.Set("X-Fa-App-Code", "flow")
	resp, err := s.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("调用 flow 网关失败（%s）：%w", s.base, err)
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return body, nil
	}
	return nil, fmt.Errorf("%s", explainError(resp.StatusCode, body))
}

// explainError 把网关错误体压成"业务码 + 人话"，便于 agent 直接转述给人。
func explainError(status int, body []byte) string {
	var e struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Details []struct {
			Reason string `json:"reason"`
		} `json:"details"`
	}
	if err := json.Unmarshal(body, &e); err == nil && (e.Message != "" || len(e.Details) > 0) {
		reason := ""
		if len(e.Details) > 0 && e.Details[0].Reason != "" {
			reason = e.Details[0].Reason
		}
		if reason != "" {
			return fmt.Sprintf("业务码 %s：%s", reason, e.Message)
		}
		return e.Message
	}
	if len(body) > 300 {
		body = body[:300]
	}
	return fmt.Sprintf("HTTP %d：%s", status, string(body))
}

// ---- 展示 ----

// pretty 输出缩进 JSON；agent 读 JSON 比读一长串更稳。
func pretty(body []byte) string {
	var v any
	if err := json.Unmarshal(body, &v); err != nil {
		return string(body)
	}
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return string(body)
	}
	return string(b)
}

// whoamiText 把身份响应压成一句话＋角色清单，方便人一眼看懂。
func whoamiText(body []byte) string {
	var m map[string]any
	if err := json.Unmarshal(body, &m); err != nil {
		return string(body)
	}
	name := pickStr(m, "name")
	account := pickStr(m, "accountId", "account_id")
	cur := pickMap(m, "currentRole", "current_role")
	roleName := pickStr(cur, "name")
	roleCode := pickStr(cur, "code")
	roles := pickList(m, "roles")
	ready := pickNum(m, "readyTasks", "ready_tasks")
	blocked := pickNum(m, "blockedTasks", "blocked_tasks")
	openQ := pickNum(m, "openQuestions", "open_questions")

	var sb strings.Builder
	fmt.Fprintf(&sb, "我是 %s（账号 %s），当前角色：%s（%s）。\n", name, account, roleName, roleCode)
	codes := make([]string, 0, len(roles))
	for _, r := range roles {
		if rm, ok := r.(map[string]any); ok {
			if c := pickStr(rm, "code"); c != "" {
				codes = append(codes, c)
			}
		}
	}
	fmt.Fprintf(&sb, "可担任 %d 个角色：%s。\n", len(roles), strings.Join(codes, ", "))
	fmt.Fprintf(&sb, "当前角色任务：可开工 %d，被阻塞 %d；待我回答的问题 %d 条。\n", ready, blocked, openQ)
	if blocked > 0 {
		sb.WriteString("有被阻塞任务 → 用 my_questions 看问题、answer_question 闭环，再 check_ready 复查。\n")
	}
	return sb.String()
}

func pickStr(m map[string]any, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return s
			}
			if f, ok := v.(float64); ok {
				return fmt.Sprintf("%.0f", f)
			}
		}
	}
	return ""
}

func pickMap(m map[string]any, keys ...string) map[string]any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if mm, ok := v.(map[string]any); ok {
				return mm
			}
		}
	}
	return map[string]any{}
}

func pickList(m map[string]any, keys ...string) []any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if l, ok := v.([]any); ok {
				return l
			}
		}
	}
	return nil
}

func pickNum(m map[string]any, keys ...string) int64 {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if f, ok := v.(float64); ok {
				return int64(f)
			}
		}
	}
	return 0
}

func argStr(m map[string]any, key string) string {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case string:
			return strings.TrimSpace(t)
		case float64:
			return fmt.Sprintf("%.0f", t)
		}
	}
	return ""
}

func argInt64(m map[string]any, key string) int64 {
	if v, ok := m[key]; ok {
		switch t := v.(type) {
		case float64:
			return int64(t)
		case string:
			var n int64
			_, _ = fmt.Sscanf(strings.TrimSpace(t), "%d", &n)
			return n
		}
	}
	return 0
}

func argBool(m map[string]any, key string) bool {
	if v, ok := m[key]; ok {
		if b, ok := v.(bool); ok {
			return b
		}
	}
	return false
}

// argStrList 读字符串数组，兼容 agent 传逗号分隔的单个字符串。
func argStrList(m map[string]any, key string) []string {
	v, ok := m[key]
	if !ok {
		return nil
	}
	if s, ok := v.(string); ok {
		out := make([]string, 0, 8)
		for _, p := range strings.Split(s, ",") {
			if p = strings.TrimSpace(p); p != "" {
				out = append(out, p)
			}
		}
		return out
	}
	raw, ok := v.([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, it := range raw {
		if s, ok := it.(string); ok {
			if s = strings.TrimSpace(s); s != "" {
				out = append(out, s)
			}
		}
	}
	return out
}

// closedText 把"完成/作废"这类空响应压成人话，避免 agent 回一段 {}。
func closedText(action string, body []byte) string {
	var v map[string]any
	if err := json.Unmarshal(body, &v); err == nil && len(v) > 0 {
		return action + "：" + pretty(body)
	}
	return action + "成功。"
}
