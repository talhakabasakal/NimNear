package rpc

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/google/uuid"
	epverification "github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/verification"
	"github.com/masterfabric-go/masterfabric/internal/application/nimiqtx/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
	nimiqnet "github.com/masterfabric-go/masterfabric/internal/shared/nimiq"
)

var (
	ErrTransactionNotFound     = errors.New("nimiq transaction not found")
	ErrRPCAuthenticationFailed = errors.New("Nimiq RPC authentication failed")
	ErrRPCTimeout              = errors.New("Nimiq RPC request timed out")
	ErrRPCUnavailable          = errors.New("Nimiq RPC is unavailable")
)

const (
	DefaultTransactionLimit = 50
	MaxTransactionLimit     = 500
)

type RPCClient interface {
	GetTransactionByHash(ctx context.Context, hash string) (Transaction, error)
	GetBlockByNumber(ctx context.Context, number uint64) (Block, error)
	GetBatchNumber(ctx context.Context) (uint64, error)
}

type Transaction struct {
	Hash            string
	BlockNumber     uint64Value
	Timestamp       uint64Value
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

type Account struct {
	Address string      `json:"address"`
	Balance uint64Value `json:"balance"`
	Type    string      `json:"type"`
}

type TransactionQuery struct {
	Max     int
	StartAt string
}

type Block struct {
	Batch   uint64Value
	Network string
	Type    string
}

type Client struct {
	endpoint   string
	username   string
	password   string
	httpClient *http.Client
}

func NewClient(endpoint string) *Client {
	trimmed := strings.TrimRight(strings.TrimSpace(endpoint), "/")
	username, password := "", ""
	if parsed, err := url.Parse(trimmed); err == nil && parsed.User != nil {
		username = parsed.User.Username()
		password, _ = parsed.User.Password()
		parsed.User = nil
		trimmed = strings.TrimRight(parsed.String(), "/")
	}
	return &Client{
		endpoint:   trimmed,
		username:   username,
		password:   password,
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
	started := time.Now()
	err := c.doCall(ctx, method, params, target)
	observeRPC(method, rpcOutcome(err), time.Since(started).Seconds())
	return err
}

func rpcOutcome(err error) string {
	switch {
	case err == nil:
		return "success"
	case errors.Is(err, ErrRPCTimeout):
		return "timeout"
	case errors.Is(err, ErrRPCAuthenticationFailed):
		return "auth_failed"
	case errors.Is(err, ErrTransactionNotFound):
		return "not_found"
	case errors.Is(err, ErrRPCUnavailable):
		return "unavailable"
	default:
		return "error"
	}
}

func (c *Client) doCall(ctx context.Context, method string, params []any, target any) error {
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
	if c.username != "" || c.password != "" {
		request.SetBasicAuth(c.username, c.password)
	}
	response, err := c.httpClient.Do(request)
	if err != nil {
		return classifyRPCTransportError(err)
	}
	defer response.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(response.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("read Nimiq RPC response: %w", err)
	}
	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return ErrRPCAuthenticationFailed
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

func (c *Client) GetAccountByAddress(ctx context.Context, address string) (Account, error) {
	if strings.TrimSpace(address) == "" {
		return Account{}, errors.New("Nimiq address is required")
	}
	var account Account
	err := c.call(ctx, "getAccountByAddress", []any{address}, &account)
	return account, err
}

func (c *Client) GetBalance(ctx context.Context, address string) (uint64, error) {
	account, err := c.GetAccountByAddress(ctx, address)
	if err != nil {
		return 0, err
	}
	return uint64(account.Balance), nil
}

func (c *Client) GetTransactionsByAddress(ctx context.Context, address string, query TransactionQuery) ([]Transaction, error) {
	if strings.TrimSpace(address) == "" {
		return nil, errors.New("Nimiq address is required")
	}
	max := query.Max
	if max <= 0 {
		max = DefaultTransactionLimit
	}
	if max > MaxTransactionLimit {
		max = MaxTransactionLimit
	}
	var startAt any
	if hash := strings.TrimSpace(query.StartAt); hash != "" {
		startAt = hash
	}
	var transactions []Transaction
	err := c.call(ctx, "getTransactionsByAddress", []any{address, max, startAt}, &transactions)
	if err != nil {
		return nil, err
	}
	if transactions == nil {
		return []Transaction{}, nil
	}
	return transactions, nil
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

// NimiqVerifier checks sender, recipient, exact Luna amount, basic transfer semantics, network, execution,
// inclusion, and macro-block finality. It intentionally does not use confirmation-count heuristics.
type NimiqVerifier struct {
	rpc      RPCClient
	merchant string
	network  string
	senders  verification.SenderResolver
}

func NewVerifier(client RPCClient, merchant, network string, senders verification.SenderResolver) *NimiqVerifier {
	return &NimiqVerifier{
		rpc:      client,
		merchant: normalizeAddress(merchant),
		network:  strings.TrimSpace(network),
		senders:  senders,
	}
}

func (v *NimiqVerifier) Verify(ctx context.Context, purchase *model.Purchase) (epverification.Outcome, error) {
	if v == nil || purchase == nil || purchase.TransactionHash == nil {
		return epverification.OutcomeNotFound, nil
	}
	return v.VerifyTransfer(ctx, verification.Transfer{
		TransactionHash: *purchase.TransactionHash,
		Recipient:       v.merchant,
		AmountLunas:     purchase.AmountLunas,
		PayerUserID:     purchase.UserID,
	})
}

func (v *NimiqVerifier) VerifyTransfer(ctx context.Context, transfer verification.Transfer) (verification.Outcome, error) {
	if v == nil || v.rpc == nil || strings.TrimSpace(transfer.TransactionHash) == "" {
		return verification.OutcomeNotFound, nil
	}
	tx, err := v.rpc.GetTransactionByHash(ctx, transfer.TransactionHash)
	if errors.Is(err, ErrTransactionNotFound) {
		return verification.OutcomeNotFound, nil
	}
	if err != nil {
		return verification.OutcomeNotFound, err
	}

	if tx.Hash != "" && !strings.EqualFold(tx.Hash, transfer.TransactionHash) {
		return verification.OutcomeInvalid, nil
	}
	matched, err := senderMatchesVerifiedIdentity(ctx, v.senders, transfer.PayerUserID, tx.From)
	if err != nil {
		return verification.OutcomeNotFound, err
	}
	if !matched ||
		normalizeAddress(tx.To) != normalizeAddress(transfer.Recipient) ||
		transfer.AmountLunas <= 0 ||
		uint64(transfer.AmountLunas) != uint64(tx.Value) ||
		tx.FromType != 0 ||
		tx.ToType != 0 ||
		tx.Flags != 0 ||
		strings.TrimSpace(tx.SenderData) != "" ||
		strings.TrimSpace(tx.RecipientData) != "" ||
		!networkIDMatches(v.network, uint64(tx.NetworkID)) ||
		tx.BlockNumber == 0 ||
		tx.ExecutionResult == nil ||
		!*tx.ExecutionResult {
		return verification.OutcomeInvalid, nil
	}

	block, err := v.rpc.GetBlockByNumber(ctx, uint64(tx.BlockNumber))
	if err != nil {
		return verification.OutcomeNotFound, err
	}
	if !nimiqnet.SameEnvironment(block.Network, v.network) || !strings.EqualFold(block.Type, "micro") || block.Batch == 0 {
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

func senderMatchesVerifiedIdentity(ctx context.Context, resolver verification.SenderResolver, userID uuid.UUID, from string) (bool, error) {
	if resolver == nil || userID == uuid.Nil {
		return false, nil
	}
	addresses, err := resolver.VerifiedAddresses(ctx, userID)
	if err != nil {
		return false, err
	}
	if len(addresses) == 0 {
		return false, nil
	}
	sender := normalizeAddress(from)
	if sender == "" {
		return false, nil
	}
	for _, address := range addresses {
		if normalizeAddress(address) == sender {
			return true, nil
		}
	}
	return false, nil
}

// networkIDMatches enforces the configured consensus network against the RPC
// transaction network id when that id is a known Albatross identifier.
func networkIDMatches(network string, id uint64) bool {
	if id == 0 {
		return false
	}
	expected, known := expectedNetworkID(network)
	if !known {
		return true
	}
	return id == expected
}

func expectedNetworkID(network string) (uint64, bool) {
	parsed, err := nimiqnet.ParseNetwork(network)
	if err != nil {
		return 0, false
	}
	return parsed.ID, true
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

func NetworkNameForID(id uint64) string {
	switch id {
	case nimiqnet.NetworkIDMain:
		return nimiqnet.ConsensusMain
	case nimiqnet.NetworkIDTest:
		return nimiqnet.ConsensusTest
	default:
		return ""
	}
}

func classifyRPCTransportError(err error) error {
	if err == nil {
		return nil
	}
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return ErrRPCTimeout
	}
	var netErr net.Error
	if errors.As(err, &netErr) && netErr.Timeout() {
		return ErrRPCTimeout
	}
	var timeoutErr interface{ Timeout() bool }
	if errors.As(err, &timeoutErr) && timeoutErr.Timeout() {
		return ErrRPCTimeout
	}
	return ErrRPCUnavailable
}
