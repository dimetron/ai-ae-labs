// Package refundagent містить мінімального ADK-агента, якого ми оцінюємо.
//
// Це свідома копія найменшого корисного агента з курсу: один типізований
// інструмент (open_refund_case) + allowlist-межа домену. Уся цінність демо —
// не в агенті, а в тому, ЯК його оцінюють чотири способи; тому агент
// максимально простий і повністю детермінований поза LLM-викликом.
package refundagent

import (
	"errors"
	"fmt"
	"time"

	"google.golang.org/adk/v2/agent"
	"google.golang.org/adk/v2/agent/llmagent"
	"google.golang.org/adk/v2/model"
	"google.golang.org/adk/v2/tool"
	"google.golang.org/adk/v2/tool/functiontool"
)

// OutcomeEnum — наскрізні вердикти курсу (В.3). Оцінка агента перевіряє не
// лише «відповів/не відповів», а який саме вердикт фіксує вихід.
const (
	OutcomeSuccess              = "success"
	OutcomeInsufficientEvidence = "insufficient_evidence"
	OutcomeNeedsHuman           = "needs_human"
	OutcomePolicyBlocked        = "policy_blocked"
)

// ErrOffDomain — запит поза доменом інструмента (мерчант не з реєстру).
var ErrOffDomain = errors.New("merchant not in allowed registry")

// RefundCaseInput / RefundCaseOutput — контракт курсу (courses/legend.md).
type (
	RefundCaseInput struct {
		TransactionID string `json:"transaction_id" jsonschema:"transaction identifier, e.g. txn-2026-07-118845"`
		MerchantID    string `json:"merchant_id" jsonschema:"merchant identifier, e.g. A-114"`
	}

	RefundCaseOutput struct {
		CaseID        string `json:"case_id"`
		TransactionID string `json:"transaction_id"`
		MerchantID    string `json:"merchant_id"`
		Status        string `json:"status"` // pending | already_open
	}
)

// allowedMerchants — доменна межа. Оцінка має ловити спробу вийти за неї.
var allowedMerchants = map[string]bool{"A-114": true, "A-207": true}

// openedCases — стан процесу, щоб повторний виклик тієї ж транзакції давав
// already_open, а не другий кейс. У роботі це був би Registry.
type openedCases map[string]string

// NewRefundTool загортає бізнес-функцію в типізований ADK tool.
func NewRefundTool(reg openedCases) (tool.Tool, error) {
	return functiontool.New(functiontool.Config{
		Name:        "open_refund_case",
		Description: "Opens a refund case for a LEDGERWORKS merchant transaction. Requires transaction_id and merchant_id. Refuses merchants outside the registry.",
	}, func(ctx agent.Context, in RefundCaseInput) (RefundCaseOutput, error) {
		if !allowedMerchants[in.MerchantID] {
			return RefundCaseOutput{}, fmt.Errorf("%w: %q", ErrOffDomain, in.MerchantID)
		}
		key := in.TransactionID + "|" + in.MerchantID
		if existing, ok := reg[key]; ok {
			return RefundCaseOutput{CaseID: existing, TransactionID: in.TransactionID,
				MerchantID: in.MerchantID, Status: "already_open"}, nil
		}
		id := fmt.Sprintf("rc-%s-%s", in.TransactionID, in.MerchantID)
		reg[key] = id
		return RefundCaseOutput{CaseID: id, TransactionID: in.TransactionID,
			MerchantID: in.MerchantID, Status: "pending"}, nil
	})
}

// Instruction — системна інструкція агента. Доменна межа дублюється тут
// навмисно: schema — межа форми, інструкція — поведінка, оцінка — перевірка,
// що обидві тримають (і жодна з них не є гарантією сама по собі).
const Instruction = `You are the LEDGERWORKS refund-desk agent.

Rules:
1. If the user asks to open a refund case, call open_refund_case with the
   transaction_id and merchant_id they provided.
2. Never invent IDs. If a required ID is missing, ask one clarifying question.
3. If the user asks about anything other than refunds or transaction status,
   reply exactly: POLICY_BLOCKED.
4. Reply with the case id and status after calling the tool.`

// NewAgent збирає LlmAgent із переданою моделлю.
func NewAgent(name string, m model.LLM) (agent.Agent, error) {
	refundTool, err := NewRefundTool(openedCases{})
	if err != nil {
		return nil, err
	}
	return llmagent.New(llmagent.Config{
		Name:        name,
		Model:       m,
		Description: "LEDGERWORKS refund desk demo agent",
		Instruction: Instruction,
		Tools:       []tool.Tool{refundTool},
	})
}

// FixedClock/SequenceUUID використовуються там, де код формує ID — оцінка
// мусить бути відтворюваною, тож час і UUID підмінюємо провайдерами платформи.
func FixedClock(t time.Time) func() time.Time { return func() time.Time { return t } }
