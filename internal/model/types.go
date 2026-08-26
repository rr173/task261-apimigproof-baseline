// Package model 定义跨版本 API 语义弃用迁移证明服务的核心实体与状态常量。
//
// 领域模型围绕四个实体展开：契约版本（ContractVersion）、字段语义
// （FieldSemantics）、请求样本（RequestSample）与迁移证明（MigrationProof），
// 外加支撑实体：转换规则（TransformationRule）、兼容窗口（CompatWindow）
// 与比较任务（Comparison）。
package model

import (
	"time"
)

// 契约版本状态机：draft → pending_compare → compatible | ambiguous → sealed。
// sealed 是终态，封存后任何字段语义与版本元数据都不可变更。
const (
	ContractDraft         = "draft"          // 草稿：可编辑字段与规则
	ContractPendingCompare = "pending_compare" // 待比较：字段齐备，等待比较任务
	ContractCompatible    = "compatible"     // 兼容：与该版本对照的最近一次比较无语义改变
	ContractAmbiguous     = "ambiguous"      // 存在歧义：存在语义改变或拒绝路径
	ContractSealed        = "sealed"         // 封存：终态，不可再修改
)

// 字段语义状态机：valid → deprecated → transformed | conflict。
// valid 为生效字段；deprecated 表示已进入弃用窗口；transformed 表示已由转换
// 规则接管；conflict 表示存在相互冲突的语义声明（不允许进入比较）。
const (
	FieldValid       = "valid"       // 有效：新客户端直接读取
	FieldDeprecated  = "deprecated"  // 废弃：旧客户端仍可发送，新客户端忽略
	FieldTransformed = "transformed" // 转换：由转换规则映射到新字段
	FieldConflict    = "conflict"    // 冲突：语义声明互相矛盾，阻塞比较
)

// 字段值类型。类型决定 coerce 动作的合法性与比较口径。
const (
	TypeBool   = "bool"
	TypeInt    = "int"
	TypeString = "string"
	TypeFloat  = "float"
	TypeJSON   = "json"
)

// 转换动作：决定旧字段值在新版本中的去向。
const (
	ActionKeep    = "keep"    // 保留：字段名与语义不变（或经 rename 改名）
	ActionRename  = "rename"  // 改名：字段名变化，值原样传递
	ActionCoerce  = "coerce"  // 强制转换：类型/格式映射，需可逆或无损
	ActionDefault = "default" // 补默认值：旧字段缺失时写入新默认值
	ActionDrop    = "drop"    // 丢弃：值被移除，有丢失区分度风险
	ActionReject  = "reject"  // 拒绝：命中即拒绝整个请求
)

// 请求样本状态机：original → migratable | rejected | semantics_changed。
const (
	SampleOriginal         = "original"          // 原始：已导入，尚未参与比较
	SampleMigratable       = "migratable"        // 可迁移：转换后可见效果等价
	SampleRejected         = "rejected"          // 被拒绝：命中 reject 规则
	SampleSemanticsChanged = "semantics_changed" // 语义改变：丢失区分度或默认值冲突
)

// 兼容窗口拒绝策略。
const (
	PolicyPreserve  = "preserve"  // 保留：窗口期内旧字段继续有效
	PolicyTransform = "transform" // 转换：窗口期内旧字段走转换路径
	PolicyReject    = "reject"    // 拒绝：窗口期满后旧字段被拒绝
)

// 比较任务状态。
const (
	ComparisonRunning   = "running"
	ComparisonCompatible = "compatible"
	ComparisonAmbiguous = "ambiguous"
	ComparisonFailed    = "failed"
)

// 迁移证明状态机：draft → published → superseded。
const (
	ProofDraft      = "draft"
	ProofPublished  = "published"
	ProofSuperseded = "superseded"
)

// 契约版本：一次 API 契约的不可变快照（字段语义集合）。
type ContractVersion struct {
	ID         int64     `json:"id"`
	Name       string    `json:"name"`   // 契约名，如 orders.v1
	Version    int       `json:"version"` // 版本号，同一契约内单调递增且不可倒退
	Status     string    `json:"status"`
	CreatedAt  time.Time `json:"created_at"`
	SealedAt   *time.Time `json:"sealed_at,omitempty"`
	FieldCount int       `json:"field_count"` // 冗余计数，便于列表展示
}

// FieldSemantics：契约内一个字段的语义声明。
type FieldSemantics struct {
	ID             int64     `json:"id"`
	ContractID     int64     `json:"contract_id"`
	FieldID        string    `json:"field_id"` // 如 auto_renew
	Status         string    `json:"status"`
	ValueType      string    `json:"value_type"`
	HasDefault     bool      `json:"has_default"` // 是否存在显式默认值
	DefaultValue   *string   `json:"default_value,omitempty"` // 默认值 JSON 文本
	DeprecatedIn   *int      `json:"deprecated_in,omitempty"` // 从哪个版本起弃用
	RemovedIn      *int      `json:"removed_in,omitempty"`    // 计划移除版本
	Description    string    `json:"description"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// TransformationRule：跨版本的字段转换规则。
type TransformationRule struct {
	ID          int64     `json:"id"`
	ContractID  int64     `json:"contract_id"` // 规则所属目标契约
	FromField   string    `json:"from_field"`
	ToField     string    `json:"to_field"` // 空串表示“无去向”（drop/reject）
	Action      string    `json:"action"`
	CoerceFrom  string    `json:"coerce_from"` // coerce 动作的源类型
	CoerceTo    string    `json:"coerce_to"`   // coerce 动作的目标类型
	DefaultJSON *string   `json:"default_json,omitempty"` // default 动作的默认值
	Precedence  int       `json:"precedence"`  // 同一 from 字段多条规则时的顺序
	CreatedAt   time.Time `json:"created_at"`
}

// RequestSample：从旧客户端捕获的真实请求样本。
type RequestSample struct {
	ID          int64     `json:"id"`
	Fingerprint string    `json:"fingerprint"` // 内容指纹，幂等去重
	PayloadJSON string    `json:"payload_json"` // 请求体 JSON
	Status      string    `json:"status"`
	ImportedAt  time.Time `json:"imported_at"`
}

// SampleResult：一个样本在某个比较任务中的判定明细。
type SampleResult struct {
	SampleID    int64  `json:"sample_id"`
	Fingerprint string `json:"fingerprint"`
	Verdict     string `json:"verdict"` // migratable / rejected / semantics_changed
	Issue       string `json:"issue"`   // IssueKind 或空
	Detail      string `json:"detail"`  // 人类可读原因
}

// Comparison：两个契约版本之间的一次语义保持性比较任务。
type Comparison struct {
	ID              int64          `json:"id"`
	FromContractID  int64          `json:"from_contract_id"`
	ToContractID    int64          `json:"to_contract_id"`
	Status          string         `json:"status"`
	TotalSamples    int            `json:"total_samples"`
	Migratable      int            `json:"migratable"`
	Rejected        int            `json:"rejected"`
	SemanticsChanged int           `json:"semantics_changed"`
	WindowID        *int64         `json:"window_id,omitempty"`
	CreatedAt       time.Time      `json:"created_at"`
	FinishedAt      *time.Time     `json:"finished_at,omitempty"`
	Results         []SampleResult `json:"results,omitempty"`
	Issues          []Issue        `json:"issues,omitempty"`
}

// IssueKind：语义保持性判定的问题类别。
const (
	IssueEquivalence       = "equivalence"        // 等价：无问题
	IssueLossOfDistinction = "loss_of_distinction" // 丢失区分度
	IssueDefaultConflict   = "default_conflict"    // 冲突默认值
	IssueRuleRejected      = "rule_rejected"       // 规则拒绝
	IssueUnknownField      = "unknown_field"       // 未知字段
	IssueCoerceFailure     = "coerce_failure"      // 转换失败
)

// Issue：比较中发现的一条语义问题。
type Issue struct {
	SampleID    int64  `json:"sample_id"`
	Kind        string `json:"kind"`
	Field       string `json:"field"`
	Message     string `json:"message"`
}

// CompatWindow：兼容窗口声明，绑定一对契约版本与拒绝策略。
type CompatWindow struct {
	ID            int64     `json:"id"`
	FromContractID int64    `json:"from_contract_id"`
	ToContractID  int64     `json:"to_contract_id"`
	Policy        string    `json:"policy"`
	ValidUntil    *string   `json:"valid_until,omitempty"` // RFC3339 或空（长期有效）
	Note          string    `json:"note"`
	CreatedAt     time.Time `json:"created_at"`
}

// MigrationProof：迁移证明，发布后绑定两个契约版本与证据摘要，不可修改。
type MigrationProof struct {
	ID             int64      `json:"id"`
	FromContractID int64      `json:"from_contract_id"`
	ToContractID   int64      `json:"to_contract_id"`
	ComparisonID   int64      `json:"comparison_id"`
	Status         string     `json:"status"`
	SummaryJSON    string     `json:"summary_json"`     // 统计摘要（total/migratable/...）
	EvidenceFingerprint string `json:"evidence_fingerprint"` // 样本证据指纹
	CreatedAt      time.Time  `json:"created_at"`
	PublishedAt    *time.Time `json:"published_at,omitempty"`
	SupersededBy   *int64     `json:"superseded_by,omitempty"`
}
