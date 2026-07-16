package types

import (
	"encoding/json"
	"testing"
)

// ---- Agent types ----

func TestAgentJSON(t *testing.T) {
	a := Agent{
		ID:      "agent-001",
		Project: "myproject",
		NodeID:  "node-1",
		Status:  AgentRunning,
		Expose:  "8080",
		Owner:   "admin",
	}
	data, err := json.Marshal(a)
	if err != nil {
		t.Fatalf("marshal Agent: %v", err)
	}
	var decoded Agent
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("unmarshal Agent: %v", err)
	}
	if decoded.ID != a.ID || decoded.Project != a.Project || decoded.Status != a.Status {
		t.Fatal("Agent round-trip failed")
	}
	if decoded.Expose != "8080" {
		t.Fatal("Expose field not preserved")
	}
}

func TestAgentResourcesJSON(t *testing.T) {
	r := AgentResources{
		CPURequest:    "500m",
		CPULimit:      "2",
		MemoryRequest: "512Mi",
		MemoryLimit:   "2Gi",
		StorageSize:   "5Gi",
	}
	data, err := json.Marshal(r)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded AgentResources
	json.Unmarshal(data, &decoded)
	if decoded.CPURequest != "500m" || decoded.StorageSize != "5Gi" {
		t.Fatal("AgentResources round-trip failed")
	}
}

func TestCreateAgentRequestJSON(t *testing.T) {
	req := CreateAgentRequest{
		Project:     "proj",
		NodeID:      "node-1",
		Image:       "custom:v1",
		ClaudeFlags: []string{"--model", "opus"},
		Resources:   &AgentResources{CPURequest: "1"},
		Logging:     true,
	}
	data, err := json.Marshal(req)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var decoded CreateAgentRequest
	json.Unmarshal(data, &decoded)
	if decoded.Project != "proj" || len(decoded.ClaudeFlags) != 2 {
		t.Fatal("CreateAgentRequest round-trip failed")
	}
}

func TestAgentSecurityJSON(t *testing.T) {
	s := AgentSecurity{
		Capabilities: []string{"NET_ADMIN"},
		DockerAccess: true,
		AdditionalMounts: []AgentMount{
			{HostPath: "/data", MountPath: "/mnt", ReadOnly: true},
		},
	}
	data, _ := json.Marshal(s)
	var decoded AgentSecurity
	json.Unmarshal(data, &decoded)
	if len(decoded.Capabilities) != 1 || !decoded.DockerAccess {
		t.Fatal("AgentSecurity round-trip failed")
	}
}

func TestConfigureAgentRequestJSON(t *testing.T) {
	project := "new-proj"
	logging := true
	claudeMd := "# Updated"
	req := ConfigureAgentRequest{
		Project:  &project,
		Logging:  &logging,
		ClaudeMd: &claudeMd,
	}
	data, _ := json.Marshal(req)
	var decoded ConfigureAgentRequest
	json.Unmarshal(data, &decoded)
	if *decoded.Project != "new-proj" || !*decoded.Logging {
		t.Fatal("ConfigureAgentRequest round-trip failed")
	}
}

// ---- Auth types ----

func TestLoginRequestJSON(t *testing.T) {
	lr := LoginRequest{Username: "admin", Password: "secret", MFAToken: "123456"}
	data, _ := json.Marshal(lr)
	var decoded LoginRequest
	json.Unmarshal(data, &decoded)
	if decoded.Username != "admin" || decoded.MFAToken != "123456" {
		t.Fatal("LoginRequest round-trip failed")
	}
}

func TestSessionJSON(t *testing.T) {
	s := Session{
		AccessToken:  "tok",
		RefreshToken: "ref",
		ExpiresAt:    "2026-01-01T00:00:00Z",
		Principal:    Principal{ID: "1", Username: "admin", Roles: []string{"admin"}},
	}
	data, _ := json.Marshal(s)
	var decoded Session
	json.Unmarshal(data, &decoded)
	if decoded.Principal.Username != "admin" {
		t.Fatal("Session round-trip failed")
	}
}

func TestPrincipalJSON(t *testing.T) {
	p := Principal{
		ID:                "user-1",
		Username:          "testuser",
		Name:              "Test User",
		Email:             "test@example.com",
		Groups:            []string{"dev"},
		Roles:             []string{"user"},
		IDP:               IDPLocal,
		AuthorizedApplets: []string{"nmgr", "amgr"},
	}
	data, _ := json.Marshal(p)
	var decoded Principal
	json.Unmarshal(data, &decoded)
	if decoded.IDP != IDPLocal || len(decoded.AuthorizedApplets) != 2 {
		t.Fatal("Principal round-trip failed")
	}
}

// ---- Board types ----

func TestBoardJSON(t *testing.T) {
	b := Board{
		BoardSummary: BoardSummary{ID: "board-1", Name: "Test Board", UpdatedAt: "2026-01-01T00:00:00Z"},
		Cards: []Card{
			{ID: "card-1", Title: "Task 1", Status: CardTodo, Links: []Edge{}},
		},
		Edges: []Edge{
			{ID: "edge-1", Type: EdgeDependsOn, From: "card-1", To: "card-2"},
		},
	}
	data, _ := json.Marshal(b)
	var decoded Board
	json.Unmarshal(data, &decoded)
	if decoded.ID != "board-1" || len(decoded.Cards) != 1 {
		t.Fatal("Board round-trip failed")
	}
}

func TestCardJSON(t *testing.T) {
	note := "test note"
	c := Card{
		ID:       "card-1",
		Title:    "Task",
		Status:   CardActive,
		Content:  "Do the thing",
		Position: &Position{X: 10, Y: 20},
		Size:     &Size{W: 200, H: 100},
		Note:     &note,
		Links:    []Edge{},
	}
	data, _ := json.Marshal(c)
	var decoded Card
	json.Unmarshal(data, &decoded)
	if decoded.Position.X != 10 || decoded.Size.W != 200 {
		t.Fatal("Card round-trip failed")
	}
	if *decoded.Note != "test note" {
		t.Fatal("Note not preserved")
	}
}

func TestCardInputJSON(t *testing.T) {
	ci := CardInput{Title: "New Card", Content: "content", Status: CardTodo}
	data, _ := json.Marshal(ci)
	var decoded CardInput
	json.Unmarshal(data, &decoded)
	if decoded.Title != "New Card" {
		t.Fatal("CardInput round-trip failed")
	}
}

func TestEdgeInputJSON(t *testing.T) {
	ei := EdgeInput{From: "a", To: "b", Type: EdgeRelatesTo}
	data, _ := json.Marshal(ei)
	var decoded EdgeInput
	json.Unmarshal(data, &decoded)
	if decoded.Type != EdgeRelatesTo {
		t.Fatal("EdgeInput round-trip failed")
	}
}

func TestExecutionRunJSON(t *testing.T) {
	prID := "pr-123"
	er := ExecutionRun{
		ID:              "run-1",
		BoardID:         "board-1",
		DispatchedCards: []string{"card-1", "card-2"},
		PullRequestID:   &prID,
		StartedAt:       "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(er)
	var decoded ExecutionRun
	json.Unmarshal(data, &decoded)
	if *decoded.PullRequestID != "pr-123" {
		t.Fatal("ExecutionRun round-trip failed")
	}
}

// ---- Common types ----

func TestPageJSON(t *testing.T) {
	p := Page[string]{Offset: 0, Limit: 10, Total: 2, Items: []string{"a", "b"}}
	data, _ := json.Marshal(p)
	var decoded Page[string]
	json.Unmarshal(data, &decoded)
	if decoded.Total != 2 || len(decoded.Items) != 2 {
		t.Fatal("Page round-trip failed")
	}
}

func TestErrorJSON(t *testing.T) {
	e := Error{
		Code:    "NOT_FOUND",
		Message: "resource not found",
		Details: []FieldDetail{{Field: "id", Issue: "invalid"}},
	}
	data, _ := json.Marshal(e)
	var decoded Error
	json.Unmarshal(data, &decoded)
	if decoded.Code != "NOT_FOUND" || len(decoded.Details) != 1 {
		t.Fatal("Error round-trip failed")
	}
}

func TestNodeStatusConstants(t *testing.T) {
	if NodeReady != "Ready" {
		t.Fatal("NodeReady")
	}
	if NodeNotReady != "NotReady" {
		t.Fatal("NodeNotReady")
	}
	if NodeSchedulingDisabled != "SchedulingDisabled" {
		t.Fatal("NodeSchedulingDisabled")
	}
}

func TestAgentStatusConstants(t *testing.T) {
	statuses := []AgentStatus{AgentRunning, AgentConnected, AgentStopped, AgentMigrating, AgentStopping}
	for _, s := range statuses {
		if s == "" {
			t.Fatal("empty agent status constant")
		}
	}
}

func TestCardStatusConstants(t *testing.T) {
	statuses := []CardStatus{CardTodo, CardActive, CardDone, CardFail, CardNote}
	for _, s := range statuses {
		if s == "" {
			t.Fatal("empty card status constant")
		}
	}
}

func TestEdgeTypeConstants(t *testing.T) {
	if EdgeDependsOn != "DependsOn" {
		t.Fatal("EdgeDependsOn")
	}
	if EdgeRelatesTo != "RelatesTo" {
		t.Fatal("EdgeRelatesTo")
	}
}

func TestVisibilityConstants(t *testing.T) {
	if VisibilityPrivate != "private" || VisibilityInternal != "internal" || VisibilityPublic != "public" {
		t.Fatal("visibility constants")
	}
}

func TestPrStatusConstants(t *testing.T) {
	if PrOpen != "open" || PrMerged != "merged" || PrClosed != "closed" {
		t.Fatal("pr status constants")
	}
}

func TestCheckStatusConstants(t *testing.T) {
	if CheckPassing != "passing" || CheckRunning != "running" || CheckFailed != "failed" {
		t.Fatal("check status constants")
	}
}

func TestTicketStatusConstants(t *testing.T) {
	statuses := []TicketStatus{TicketOpen, TicketInProgress, TicketResolved, TicketClosed}
	for _, s := range statuses {
		if s == "" {
			t.Fatal("empty ticket status")
		}
	}
}

func TestTicketPriorityConstants(t *testing.T) {
	if PriorityLow != "low" || PriorityMedium != "medium" || PriorityHigh != "high" {
		t.Fatal("ticket priority constants")
	}
}

func TestIDPConstants(t *testing.T) {
	if IDPLocal != "local" || IDPGitHub != "github" {
		t.Fatal("IDP constants")
	}
}

func TestUserStatusConstants(t *testing.T) {
	if UserActive != "active" || UserDisabled != "disabled" {
		t.Fatal("user status constants")
	}
}

// ---- Event types ----

func TestEventJSON(t *testing.T) {
	e := Event{
		Type:      "node.updated",
		Timestamp: "2026-01-01T00:00:00Z",
		Payload:   map[string]string{"id": "node-1"},
	}
	data, _ := json.Marshal(e)
	var decoded Event
	json.Unmarshal(data, &decoded)
	if decoded.Type != "node.updated" {
		t.Fatal("Event round-trip failed")
	}
}

func TestServiceStatusJSON(t *testing.T) {
	ss := ServiceStatus{Name: "postgres", Status: "healthy", LatencyMs: 1.5}
	data, _ := json.Marshal(ss)
	var decoded ServiceStatus
	json.Unmarshal(data, &decoded)
	if decoded.LatencyMs != 1.5 {
		t.Fatal("ServiceStatus round-trip failed")
	}
}

func TestPlatformStatusJSON(t *testing.T) {
	ps := PlatformStatus{
		Status:  "healthy",
		Version: "2.0.0",
		Uptime:  "24h",
		Services: []ServiceStatus{
			{Name: "pg", Status: "healthy"},
		},
		Nodes: []NodeHealthSummary{
			{NodeID: "n1", Status: NodeReady, Reachable: true, Agents: 3},
		},
		Timestamp: "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(ps)
	var decoded PlatformStatus
	json.Unmarshal(data, &decoded)
	if len(decoded.Services) != 1 || len(decoded.Nodes) != 1 {
		t.Fatal("PlatformStatus round-trip failed")
	}
}

// ---- Node types ----

func TestNodeJSON(t *testing.T) {
	n := Node{
		ID:             "node-1",
		Location:       "us-east-1",
		IP:             "10.0.0.1",
		Status:         NodeReady,
		Agents:         5,
		LoadAvg:        LoadAvg{One: 1.0, Five: 0.8, Fifteen: 0.6},
		MemUsedGB:      8.5,
		MemTotalGB:     16.0,
		DiskUsedGB:     50.0,
		DiskTotalGB:    100.0,
		KubeletVersion: "v1.31.0",
		Tags:           []string{"env=prod"},
	}
	data, _ := json.Marshal(n)
	var decoded Node
	json.Unmarshal(data, &decoded)
	if decoded.LoadAvg.One != 1.0 || decoded.KubeletVersion != "v1.31.0" {
		t.Fatal("Node round-trip failed")
	}
}

func TestNodeDetailJSON(t *testing.T) {
	nd := NodeDetail{
		Node: Node{ID: "node-1", Status: NodeReady},
		Conditions: []NodeCondition{
			{Type: "Ready", Status: "True", Reason: "KubeletReady"},
		},
		Labels: map[string]string{"env": "prod"},
		Taints: []NodeTaint{
			{Key: "dedicated", Value: "agents", Effect: "NoSchedule"},
		},
		Capacity:    NodeResources{CPUCores: 4, MemoryGB: 16, EphemeralGB: 100, Pods: 110},
		Allocatable: NodeResources{CPUCores: 3.8, MemoryGB: 15, EphemeralGB: 90, Pods: 110},
	}
	data, _ := json.Marshal(nd)
	var decoded NodeDetail
	json.Unmarshal(data, &decoded)
	if len(decoded.Conditions) != 1 || decoded.Capacity.CPUCores != 4 {
		t.Fatal("NodeDetail round-trip failed")
	}
}

func TestNodeMetricsReportJSON(t *testing.T) {
	nmr := NodeMetricsReport{
		NodeName:       "node-1",
		LoadAvg:        LoadAvg{One: 2.0},
		MemUsedBytes:   8 * 1024 * 1024 * 1024,
		MemTotalBytes:  16 * 1024 * 1024 * 1024,
		DiskUsedBytes:  50 * 1024 * 1024 * 1024,
		DiskTotalBytes: 100 * 1024 * 1024 * 1024,
		UptimeSeconds:  86400,
		KubeletVersion: "v1.31.0",
		CPUCount:       8,
		Timestamp:      "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(nmr)
	var decoded NodeMetricsReport
	json.Unmarshal(data, &decoded)
	if decoded.CPUCount != 8 || decoded.UptimeSeconds != 86400 {
		t.Fatal("NodeMetricsReport round-trip failed")
	}
}

func TestProvisionNodeRequestJSON(t *testing.T) {
	pnr := ProvisionNodeRequest{
		Host:     "192.168.1.100",
		User:     "convocate",
		Password: "secret",
		Location: "us-west-2",
		Tags:     []string{"gpu", "high-mem"},
	}
	data, _ := json.Marshal(pnr)
	var decoded ProvisionNodeRequest
	json.Unmarshal(data, &decoded)
	if decoded.Host != "192.168.1.100" || len(decoded.Tags) != 2 {
		t.Fatal("ProvisionNodeRequest round-trip failed")
	}
}

// ---- Repo types ----

func TestRepoJSON(t *testing.T) {
	r := Repo{
		ID:            "repo-1",
		Name:          "convocate",
		Description:   "Agent platform",
		DefaultBranch: "main",
		Visibility:    VisibilityPrivate,
		UpdatedAt:     "2026-01-01T00:00:00Z",
	}
	data, _ := json.Marshal(r)
	var decoded Repo
	json.Unmarshal(data, &decoded)
	if decoded.Visibility != VisibilityPrivate {
		t.Fatal("Repo round-trip failed")
	}
}

func TestPullRequestJSON(t *testing.T) {
	pr := PullRequest{
		ID:           "pr-1",
		RepoID:       "repo-1",
		Title:        "Add feature",
		Branch:       "feature/x",
		TargetBranch: "main",
		Status:       PrOpen,
		Author:       "dev",
		Files:        []string{"main.go"},
		Checks:       []Check{{Name: "lint", Status: CheckPassing}},
	}
	data, _ := json.Marshal(pr)
	var decoded PullRequest
	json.Unmarshal(data, &decoded)
	if decoded.Status != PrOpen || len(decoded.Checks) != 1 {
		t.Fatal("PullRequest round-trip failed")
	}
}

func TestProjectJSON(t *testing.T) {
	p := Project{
		ID:                  "proj-1",
		Name:                "My Project",
		RepoID:              "repo-1",
		SpecificationFileID: "spec.md",
		BoardID:             "board-1",
	}
	data, _ := json.Marshal(p)
	var decoded Project
	json.Unmarshal(data, &decoded)
	if decoded.BoardID != "board-1" {
		t.Fatal("Project round-trip failed")
	}
}

func TestFileContentJSON(t *testing.T) {
	fc := FileContent{
		Path:     "/src/main.go",
		Content:  "package main",
		Language: "go",
	}
	data, _ := json.Marshal(fc)
	var decoded FileContent
	json.Unmarshal(data, &decoded)
	if decoded.Language != "go" {
		t.Fatal("FileContent round-trip failed")
	}
}

// ---- Ticket types ----

func TestTicketJSON(t *testing.T) {
	tk := Ticket{
		ID:       "ticket-1",
		Subject:  "Bug report",
		Status:   TicketOpen,
		Priority: PriorityHigh,
		Body:     "Something broke",
	}
	data, _ := json.Marshal(tk)
	var decoded Ticket
	json.Unmarshal(data, &decoded)
	if decoded.Priority != PriorityHigh {
		t.Fatal("Ticket round-trip failed")
	}
}

func TestTicketInputJSON(t *testing.T) {
	ti := TicketInput{Subject: "Help", Priority: PriorityMedium, Body: "Need help"}
	data, _ := json.Marshal(ti)
	var decoded TicketInput
	json.Unmarshal(data, &decoded)
	if decoded.Subject != "Help" {
		t.Fatal("TicketInput round-trip failed")
	}
}

func TestDocArticleJSON(t *testing.T) {
	da := DocArticle{ID: "doc-1", Title: "Getting Started", Slug: "getting-started"}
	data, _ := json.Marshal(da)
	var decoded DocArticle
	json.Unmarshal(data, &decoded)
	if decoded.Slug != "getting-started" {
		t.Fatal("DocArticle round-trip failed")
	}
}

// ---- User types ----

func TestUserJSON(t *testing.T) {
	u := User{
		ID:     "user-1",
		Email:  "test@example.com",
		Name:   "Test User",
		Status: UserActive,
		Groups: []string{"developers"},
	}
	data, _ := json.Marshal(u)
	var decoded User
	json.Unmarshal(data, &decoded)
	if decoded.Email != "test@example.com" || decoded.Status != UserActive {
		t.Fatal("User round-trip failed")
	}
}

func TestUserInputJSON(t *testing.T) {
	ui := UserInput{
		Email:    "new@example.com",
		Name:     "New User",
		Password: "secret123",
		Status:   UserActive,
		Groups:   []string{"dev"},
	}
	data, _ := json.Marshal(ui)
	var decoded UserInput
	json.Unmarshal(data, &decoded)
	if decoded.Password != "secret123" {
		t.Fatal("UserInput round-trip failed")
	}
}

func TestGroupJSON(t *testing.T) {
	g := Group{
		ID:        "grp-1",
		Name:      "admins",
		Builtin:   true,
		UserCount: 5,
		Roles:     []string{"admin"},
	}
	data, _ := json.Marshal(g)
	var decoded Group
	json.Unmarshal(data, &decoded)
	if !decoded.Builtin || decoded.UserCount != 5 {
		t.Fatal("Group round-trip failed")
	}
}

func TestRoleJSON(t *testing.T) {
	r := Role{ID: "role-1", Description: "Admin role", Applet: "nmgr"}
	data, _ := json.Marshal(r)
	var decoded Role
	json.Unmarshal(data, &decoded)
	if decoded.Applet != "nmgr" {
		t.Fatal("Role round-trip failed")
	}
}

func TestGlobalSettingsJSON(t *testing.T) {
	gs := GlobalSettings{
		RequireMFA:           true,
		SessionTimeoutMin:    30,
		PasswordMinLength:    12,
		PasswordRotationDays: 90,
	}
	data, _ := json.Marshal(gs)
	var decoded GlobalSettings
	json.Unmarshal(data, &decoded)
	if !decoded.RequireMFA || decoded.SessionTimeoutMin != 30 {
		t.Fatal("GlobalSettings round-trip failed")
	}
}

// ---- Omitempty behavior ----

func TestAgentJSON_OmitEmptyExpose(t *testing.T) {
	a := Agent{ID: "a1", Status: AgentRunning}
	data, _ := json.Marshal(a)
	// Expose is omitempty, should not appear
	str := string(data)
	if contains(str, "expose") {
		t.Fatal("expected expose to be omitted when empty")
	}
}

func TestRepoFileJSON(t *testing.T) {
	rf := RepoFile{Name: "main.go", Type: "file", Size: 1024, Path: "/src/main.go"}
	data, _ := json.Marshal(rf)
	var decoded RepoFile
	json.Unmarshal(data, &decoded)
	if decoded.Size != 1024 {
		t.Fatal("RepoFile round-trip failed")
	}
}

func contains(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
