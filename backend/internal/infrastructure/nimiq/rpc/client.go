package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
)

var ErrTransactionNotFound = errors.New("nimiq transaction not found")

type RPCClient interface {
	GetTransactionByHash(ctx context.Context, hash string) (Transaction, error)
	GetBlockByNumber(ctx context.Context, number uint64) (Block, error)
	GetBatchNumber(ctx context.Context) (uint64, error)
}

type Transaction struct {
	Hash            string
	BlockNumber     uint64Value
	From            string
	To              string
	FromType        uint64Value
	ToType          uint64Value
	Value           uint64Value
	Flags           uint64Value
	SenderData      string
	RecipientData   string
	NetworkID       uint64Value
	ExecutionResult *bool
}

type Block struct {
	Batch   uint64Value
	Network string
	Type    string
}

type Client struct {
	endpoint   string
	httpClient *http.Client
}

func NewClient(endpoint string) *Client {
	return &Client{
		endpoint:   strings.TrimRight(endpoint, "/"),
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

type responseEnvelope struct {
	Result *struct {
		Data json.RawMessage
	}
	Error *struct {
		Code    int
		Message string
	}
}

func (c *Client) call(ctx context.Context, method string, params []any, target any) error {
	if c == nil || c.endpoint == "" {
		return errors.New("Nimiq RPC URL is not configured")
	}
	body, err := json.Marshal(map[string]any{
		"jsonrpc": "2.0",
		"id":      1,
		"method":  method,
		"params":  params,
	})
	if err != nil {
		return fmt.Errorf("marshal Nimiq RPC request: %w", err)
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create Nimiq RPC request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := c.httpClient.Do(request)
	if err != nil {
		return fmt.Errorf("call Nimiq RPC: %w", err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("read Nimiq RPC response: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("Nimiq RPC returned HTTP %d", response.StatusCode)
	}
	var envelope responseEnvelope
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fmt.Errorf("decode Nimiq RPC response: %w", err)
	}
	if envelope.Error != nil {
		if strings.Contains(strings.ToLower(envelope.Error.Message), "not found") {
			return ErrTransactionNotFound
		}
		return fmt.Errorf("Nimiq RPC error %d: %s", envelope.Error.Code, envelope.Error.Message)
	}
	if envelope.Result == nil || len(envelope.Result.Data) == 0 || string(envelope.Result.Data) == "null" {
		return errors.New("Nimiq RPC response did not contain result data")
	}
	if err := json.Unmarshal(envelope.Result.Data, target); err != nil {
		return fmt.Errorf("decode Nimiq RPC result: %w", err)
	}
	return nil
}

func (c *Client) GetTransactionByHash(ctx context.Context, hash string) (Transaction, error) {
	var tx Transaction
	err := c.call(ctx, "getTransactionByHash", []any{hash}, &tx)
	return tx, err
}

func (c *Client) GetBlockByNumber(ctx context.Context, number uint64) (Block, error) {
	var block Block
	err := c.call(ctx, "getBlockByNumber", []any{number, false}, &block)
	return block, err
}

func (c *Client) GetBatchNumber(ctx context.Context) (uint64, error) {
	var number uint64Value
	err := c.call(ctx, "getBatchNumber", []any{}, &number)
	return uint64(number), err
}

// NimiqVerifier checks recipient, exact Luna amount, basic transfer semantics, network, execution,
// inclusion, and macro-block finality. It intentionally does not use confirmation-count heuristics.
type NimiqVerifier struct {
	rpc      RPCClient
	merchant string
	network  string
}

func NewVerifier(client RPCClient, merchant, network string) *NimiqVerifier {
	return &NimiqVerifier{
		rpc:      client,
		merchant: normalizeAddress(merchant),
		network:  strings.TrimSpace(network),
	}
}

func (v *NimiqVerifier) Verify(ctx context.Context, purchase *model.Purchase) (verification.Outcome, error) {
	if v == nil || v.rpc == nil || purchase == nil || purchase.TransactionHash == nil {
		return verification.OutcomeNotFound, nil
	}
	tx, err := v.rpc.GetTransactionByHash(ctx, *purchase.TransactionHash)
	if errors.Is(err, ErrTransactionNotFound) {
		return verification.OutcomeNotFound, nil
	}
	if err != nil {
		return verification.OutcomeNotFound, err
	}

	if tx.Hash != "" && !strings.EqualFold(tx.Hash, *purchase.TransactionHash) {
		return verification.OutcomeInvalid, nil
	}
	if normalizeAddress(tx.To) != v.merchant ||
		purchase.AmountLunas <= 0 ||
		uint64(purchase.AmountLunas) != uint64(tx.Value) ||
		tx.FromType != 0 ||
		tx.ToType != 0 ||
		tx.Flags != 0 ||
		strings.TrimSpace(tx.SenderData) != "" ||
		strings.TrimSpace(tx.RecipientData) != "" ||
		tx.NetworkID == 0 ||
		tx.BlockNumber == 0 ||
		tx.ExecutionResult == nil ||
		!*tx.ExecutionResult {
		return verification.OutcomeInvalid, nil
	}

	block, err := v.rpc.GetBlockByNumber(ctx, uint64(tx.BlockNumber))
	if err != nil {
		return verification.OutcomeNotFound, err
	}
	if block.Network != v.network || !strings.EqualFold(block.Type, "micro") || block.Batch == 0 {
		return verification.OutcomeInvalid, nil
	}

	currentBatch, err := v.rpc.GetBatchNumber(ctx)
	if err != nil {
		return verification.OutcomeNotFound, err
	}
	if currentBatch <= uint64(block.Batch) {
		return verification.OutcomeNotFinal, nil
	}
	return verification.OutcomeConfirmed, nil
}

func normalizeAddress(address string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(address), " ", ""))
}

type uint64Value uint64

func (v *uint64Value) UnmarshalJSON(data []byte) error {
	value := strings.TrimSpace(string(data))
	value = strings.Trim(value, string(rune(34)))
	if value == "" || value == "null" {
		*v = 0
		return nil
	}
	parsed, err := strconv.ParseUint(value, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid uint64 RPC value %q: %w", value, err)
	}
	*v = uint64Value(parsed)
	return nil
}
