// API types mirroring openapi.yaml and Go types

// Enums
export type NodeStatus = "online" | "draining" | "offline";
export type AgentStatus = "running" | "connected" | "stopped" | "migrating" | "stopping";
export type CardStatus = "todo" | "active" | "done" | "fail" | "note";
export type EdgeType = "DependsOn" | "RelatesTo";
export type UserStatus = "active" | "disabled";
export type Visibility = "private" | "internal" | "public";
export type PrStatus = "open" | "merged" | "closed";
export type CheckStatus = "passing" | "running" | "failed";
export type TicketStatus = "open" | "in-progress" | "resolved" | "closed";
export type TicketPriority = "low" | "medium" | "high";
export type IDP = "local" | "github";

// Common
export interface Page<T> {
  offset: number;
  limit: number;
  total: number;
  items: T[];
}

export interface ApiError {
  code: string;
  message: string;
  details?: FieldDetail[];
}

export interface FieldDetail {
  field: string;
  issue: string;
}

// Auth
export interface LoginRequest {
  username: string;
  password: string;
  mfaToken: string;
}

export interface Session {
  accessToken: string;
  refreshToken: string;
  expiresAt: string;
  principal: Principal;
}

export interface Principal {
  id: string;
  username: string;
  name: string;
  email?: string;
  groups?: string[];
  roles: string[];
  idp: IDP;
  authorizedApplets: string[];
}

// Node Manager
export interface LoadAvg {
  one: number;
  five: number;
  fifteen: number;
}

export interface Node {
  id: string;
  location: string;
  ip: string;
  status: NodeStatus;
  agents: number;
  loadAvg: LoadAvg;
  memUsedGB: number;
  memTotalGB: number;
  diskUsedGB: number;
  diskTotalGB: number;
  tags: string[];
}

export interface NodeDetail extends Node {
  agentList: Agent[];
  notes: Note[];
}

export interface Note {
  author: string;
  createdAt: string;
  text: string;
}

export interface ProvisionNodeRequest {
  host: string;
  user: string;
  password?: string;
  location: string;
  tags: string[];
}

// Agent Manager
export interface Agent {
  id: string;
  project: string;
  nodeId: string;
  status: AgentStatus;
  expose?: string;
  owner: string;
}

export interface CreateAgentRequest {
  project: string;
  nodeId: string;
  image?: string;
  command?: string;
}

export interface ConfigureAgentRequest {
  project?: string;
  nodeId?: string;
  expose?: string;
}

// Project Board
export interface Geometry {
  x: number;
  y: number;
  w: number;
  h: number;
}

export interface Position {
  x: number;
  y: number;
}

export interface Size {
  w: number;
  h: number;
}

export interface BoardSummary {
  id: string;
  name: string;
  repoId?: string;
  updatedAt: string;
}

export interface Board extends BoardSummary {
  containers: Container[];
  cards: Card[];
  edges: Edge[];
}

export interface Container {
  id: string;
  title: string;
  agentId: string | null;
  minimized: boolean;
  geometry?: Geometry;
}

export interface Card {
  id: string;
  title: string;
  status: CardStatus;
  content: string;
  containerId: string | null;
  position?: Position;
  size?: Size;
  sourceRefs?: string[];
  note: string | null;
  links: Edge[];
}

export interface Edge {
  id: string;
  type: EdgeType;
  from: string;
  to: string;
}

export interface ExecutionRun {
  id: string;
  boardId: string;
  dispatchedCards: string[];
  pullRequestId: string | null;
  startedAt: string;
}

// Code IDE
export interface Project {
  id: string;
  name: string;
  repoId: string;
  specificationFileId: string;
  boardId?: string;
}

export interface FileContent {
  path: string;
  content: string;
  language?: string;
  updatedAt: string;
}

// Repo Manager
export interface Repo {
  id: string;
  name: string;
  description: string;
  defaultBranch: string;
  visibility: Visibility;
  updatedAt: string;
}

export interface RepoFile {
  name: string;
  type: "file" | "dir";
  size: number;
  path: string;
}

export interface PullRequest {
  id: string;
  repoId: string;
  title: string;
  branch: string;
  targetBranch: string;
  status: PrStatus;
  author: string;
  files: string[];
  checks: Check[];
}

export interface Check {
  name: string;
  status: CheckStatus;
}

// Access Control
export interface User {
  id: string;
  email: string;
  name: string;
  status: UserStatus;
  groups: string[];
}

export interface Group {
  id: string;
  name: string;
  builtin: boolean;
  userCount: number;
  roles: string[];
}

export interface Role {
  id: string;
  description: string;
  applet: string;
}

export interface GlobalSettings {
  requireMfa: boolean;
  sessionTimeoutMinutes: number;
  passwordMinLength: number;
  passwordRotationDays: number;
}

// Support Tool
export interface Ticket {
  id: string;
  subject: string;
  status: TicketStatus;
  priority: TicketPriority;
  body: string;
  updatedAt: string;
}

export interface DocArticle {
  id: string;
  title: string;
  slug: string;
}

// Events
export interface WSEvent {
  type: string;
  timestamp: string;
  payload: unknown;
}

// Platform Status
export interface ServiceStatus {
  name: string;
  status: "healthy" | "degraded" | "unavailable";
  latencyMs: number;
  message?: string;
}

export interface NodeHealthSummary {
  nodeId: string;
  status: NodeStatus;
  reachable: boolean;
  agents: number;
}

export interface PlatformStatus {
  status: "healthy" | "degraded" | "unavailable";
  version: string;
  uptime: string;
  services: ServiceStatus[];
  nodes: NodeHealthSummary[];
  timestamp: string;
}
