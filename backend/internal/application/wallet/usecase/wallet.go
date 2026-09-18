package usecase

import (
	"context"
	"errors"
	"regexp"
	"strconv"
	"strings"

	"github.com/google/uuid"
	eventUC "github.com/masterfabric-go/masterfabric/internal/application/event/usecase"
	iamUC "github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	"github.com/masterfabric-go/masterfabric/internal/application/wallet/dto"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const (
	DefaultTransactionLimit = 50
	MaxTransactionLimit     = 500
)

var transactionHashPattern = regexp.MustCompile(`^[0-9a-f]{64}$`)

type IdentityResolver interface {
	VerifiedAddresses(context.Context, uuid.UUID) ([]string, error)
}

// Account is the RPC account subset used for wallet reads.
type Account struct {
	Address string
	Balance uint64
	Type    string
}

// Transaction is the RPC transaction subset used for wallet history.
type Transaction struct {
	Hash            string
	BlockNumber     uint64
	Timestamp       uint64
	From            string
	To              string
	Value           uint64
	NetworkID       uint64
	Network         string
	ExecutionResult *bool
}

type AccountReader interface {
	GetAccountByAddress(ctx context.Context, address string) (Account, error)
	GetTransactionsByAddress(ctx context.Context, address string, max int, startAt string) ([]Transaction, error)
}

// WalletUseCase reads authenticated users' verified Nimiq wallet data from RPC.
type WalletUseCase struct {
	identities IdentityResolver
	rpc        AccountReader
	network    string
}

func NewWalletUseCase(identities IdentityResolver, client AccountReader, network string) *WalletUseCase {
	return &WalletUseCase{identities: identities, rpc: client, network: strings.TrimSpace(network)}
}

func (uc *WalletUseCase) GetBalance(ctx context.Context, query dto.WalletQuery) (*dto.WalletBalanceResponse, error) {
	address, err := uc.authorizedAddress(ctx, query)
	if err != nil {
		return nil, err
	}
	if uc.rpc == nil {
		return nil, domainErr.NewWithCode(domainErr.ErrInternal, "nimiq_rpc_unavailable", "Nimiq RPC is not configured", nil)
	}
	account, err := uc.rpc.GetAccountByAddress(ctx, address)
	if err != nil {
		return nil, err
	}
	return &dto.WalletBalanceResponse{Data: dto.WalletBalance{
		Address:      address,
		BalanceLunas: strconv.FormatUint(account.Balance, 10),
		BalanceNIM:   formatNIM(account.Balance),
		AccountType:  strings.TrimSpace(account.Type),
		Network:      uc.network,
	}}, nil
}

func (uc *WalletUseCase) GetTransactions(ctx context.Context, query dto.WalletQuery) (*dto.WalletTransactionsResponse, error) {
	address, err := uc.authorizedAddress(ctx, query)
	if err != nil {
		return nil, err
	}
	if uc.rpc == nil {
		return nil, domainErr.NewWithCode(domainErr.ErrInternal, "nimiq_rpc_unavailable", "Nimiq RPC is not configured", nil)
	}
	max := query.Max
	if max < 0 {
		return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "invalid_transaction_limit", "max must be a positive integer", nil)
	}
	startAt := strings.ToLower(strings.TrimSpace(query.StartAt))
	if startAt != "" && !transactionHashPattern.MatchString(startAt) {
		return nil, domainErr.NewWithCode(domainErr.ErrBadRequest, "invalid_start_at", "start_at must be a 64-character transaction hash", nil)
	}
	transactions, err := uc.rpc.GetTransactionsByAddress(ctx, address, max, startAt)
	if err != nil {
		return nil, err
	}
	items := make([]dto.WalletTransaction, 0, len(transactions))
	for _, tx := range transactions {
		normalized, err := normalizeWalletTransaction(tx)
		if err != nil {
			return nil, domainErr.NewWithCode(domainErr.ErrInternal, "malformed_nimiq_transaction", "Nimiq RPC returned a malformed transaction", err)
		}
		items = append(items, normalized)
	}
	response := &dto.WalletTransactionsResponse{Data: items}
	if max <= 0 {
		max = DefaultTransactionLimit
	}
	if max > MaxTransactionLimit {
		max = MaxTransactionLimit
	}
	if len(items) == max && items[len(items)-1].Hash != "" {
		response.NextStartAt = items[len(items)-1].Hash
	}
	return response, nil
}

func (uc *WalletUseCase) authorizedAddress(ctx context.Context, query dto.WalletQuery) (string, error) {
	if query.UserID == uuid.Nil {
		return "", domainErr.New(domainErr.ErrUnauthorized, "user not authenticated", nil)
	}
	if uc.identities == nil {
		return "", domainErr.NewWithCode(domainErr.ErrNotFound, "no_verified_nimiq_identity", "no verified Nimiq identity is linked to this account", nil)
	}
	addresses, err := uc.identities.VerifiedAddresses(ctx, query.UserID)
	if err != nil {
		return "", err
	}
	if len(addresses) == 0 {
		return "", domainErr.NewWithCode(domainErr.ErrNotFound, "no_verified_nimiq_identity", "no verified Nimiq identity is linked to this account", nil)
	}

	requested := strings.TrimSpace(query.Address)
	if requested == "" {
		return addresses[0], nil
	}
	normalized, err := iamUC.NormalizeNimiqAddress(requested)
	if err != nil {
		return "", domainErr.NewWithCode(domainErr.ErrBadRequest, "invalid_wallet_address", "invalid Nimiq wallet address", nil)
	}
	compactRequested := compactAddress(normalized)
	for _, address := range addresses {
		if compactAddress(address) == compactRequested {
			return address, nil
		}
	}
	return "", domainErr.NewWithCode(domainErr.ErrForbidden, "wallet_identity_forbidden", "wallet address is not a verified identity of this user", nil)
}

func normalizeWalletTransaction(tx Transaction) (dto.WalletTransaction, error) {
	if strings.TrimSpace(tx.Hash) == "" || strings.TrimSpace(tx.From) == "" || strings.TrimSpace(tx.To) == "" {
		return dto.WalletTransaction{}, errors.New("transaction is missing hash, sender, or recipient")
	}
	item := dto.WalletTransaction{
		Hash:       strings.TrimSpace(tx.Hash),
		Sender:     strings.TrimSpace(tx.From),
		Recipient:  strings.TrimSpace(tx.To),
		ValueLunas: strconv.FormatUint(tx.Value, 10),
		ValueNIM:   formatNIM(tx.Value),
		Network:    strings.TrimSpace(tx.Network),
	}
	if tx.BlockNumber != 0 {
		blockNumber := tx.BlockNumber
		item.BlockNumber = &blockNumber
	}
	if tx.Timestamp != 0 {
		timestamp := tx.Timestamp
		item.Timestamp = &timestamp
	}
	if tx.NetworkID != 0 {
		networkID := tx.NetworkID
		item.NetworkID = &networkID
	}
	if tx.ExecutionResult != nil {
		if *tx.ExecutionResult {
			item.Status = "successful"
		} else {
			item.Status = "failed"
		}
	}
	return item, nil
}

func compactAddress(address string) string {
	return strings.ToUpper(strings.ReplaceAll(strings.TrimSpace(address), " ", ""))
}

func formatNIM(lunas uint64) string {
	if lunas == 0 {
		return "0"
	}
	whole := lunas / uint64(eventUC.LunasPerNIM)
	fraction := lunas % uint64(eventUC.LunasPerNIM)
	if fraction == 0 {
		return strconv.FormatUint(whole, 10)
	}
	text := strconv.FormatUint(fraction+uint64(eventUC.LunasPerNIM), 10)[1:]
	for len(text) > 0 && text[len(text)-1] == '0' {
		text = text[:len(text)-1]
	}
	return strconv.FormatUint(whole, 10) + "." + text
}
