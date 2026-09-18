package rpc

import (
	"context"
	"errors"

	walletUC "github.com/masterfabric-go/masterfabric/internal/application/wallet/usecase"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

// WalletReader adapts the shared Nimiq RPC client to the wallet use case.
type WalletReader struct {
	client *Client
}

func NewWalletReader(client *Client) *WalletReader {
	if client == nil {
		return nil
	}
	return &WalletReader{client: client}
}

func (r *WalletReader) GetAccountByAddress(ctx context.Context, address string) (walletUC.Account, error) {
	if r == nil || r.client == nil {
		return walletUC.Account{}, domainErr.NewWithCode(domainErr.ErrInternal, "nimiq_rpc_unavailable", "Nimiq RPC is not configured", nil)
	}
	account, err := r.client.GetAccountByAddress(ctx, address)
	if err != nil {
		return walletUC.Account{}, mapWalletRPCError(err)
	}
	return walletUC.Account{
		Address: account.Address,
		Balance: uint64(account.Balance),
		Type:    account.Type,
	}, nil
}

func (r *WalletReader) GetTransactionsByAddress(ctx context.Context, address string, max int, startAt string) ([]walletUC.Transaction, error) {
	if r == nil || r.client == nil {
		return nil, domainErr.NewWithCode(domainErr.ErrInternal, "nimiq_rpc_unavailable", "Nimiq RPC is not configured", nil)
	}
	transactions, err := r.client.GetTransactionsByAddress(ctx, address, TransactionQuery{Max: max, StartAt: startAt})
	if err != nil {
		return nil, mapWalletRPCError(err)
	}
	out := make([]walletUC.Transaction, 0, len(transactions))
	for _, tx := range transactions {
		item := walletUC.Transaction{
			Hash:            tx.Hash,
			BlockNumber:     uint64(tx.BlockNumber),
			Timestamp:       uint64(tx.Timestamp),
			From:            tx.From,
			To:              tx.To,
			Value:           uint64(tx.Value),
			NetworkID:       uint64(tx.NetworkID),
			Network:         NetworkNameForID(uint64(tx.NetworkID)),
			ExecutionResult: tx.ExecutionResult,
		}
		out = append(out, item)
	}
	return out, nil
}

func mapWalletRPCError(err error) error {
	switch {
	case err == nil:
		return nil
	case errors.Is(err, ErrRPCTimeout):
		return domainErr.NewWithCode(domainErr.ErrInternal, "nimiq_rpc_timeout", "Nimiq RPC request timed out", err)
	case errors.Is(err, ErrRPCAuthenticationFailed):
		return domainErr.NewWithCode(domainErr.ErrInternal, "nimiq_rpc_authentication_failed", "Nimiq RPC authentication failed", err)
	case errors.Is(err, ErrRPCUnavailable):
		return domainErr.NewWithCode(domainErr.ErrInternal, "nimiq_rpc_unavailable", "Nimiq RPC is unavailable", err)
	default:
		return domainErr.NewWithCode(domainErr.ErrInternal, "nimiq_rpc_error", "Nimiq wallet data is temporarily unavailable", err)
	}
}
