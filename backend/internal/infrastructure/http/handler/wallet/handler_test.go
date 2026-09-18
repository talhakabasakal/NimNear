package wallet

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/google/uuid"
	iamUC "github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	"github.com/masterfabric-go/masterfabric/internal/application/wallet/usecase"
	"github.com/masterfabric-go/masterfabric/internal/shared/middleware"
)

type handlerIdentities struct {
	addresses []string
}

func (s handlerIdentities) VerifiedAddresses(context.Context, uuid.UUID) ([]string, error) {
	return s.addresses, nil
}

type handlerRPC struct {
	account      usecase.Account
	transactions []usecase.Transaction
	seenAddress  string
}

func (s *handlerRPC) GetAccountByAddress(_ context.Context, address string) (usecase.Account, error) {
	s.seenAddress = address
	return s.account, nil
}

func (s *handlerRPC) GetTransactionsByAddress(_ context.Context, address string, _ int, _ string) ([]usecase.Transaction, error) {
	s.seenAddress = address
	if s.transactions == nil {
		return []usecase.Transaction{}, nil
	}
	return s.transactions, nil
}

const handlerPrimary = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"

func TestGetBalanceRequiresAuthentication(t *testing.T) {
	handler := NewHandler(usecase.NewWalletUseCase(handlerIdentities{addresses: []string{handlerPrimary}}, &handlerRPC{}, ""))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wallet/balance", nil)
	rec := httptest.NewRecorder()
	handler.GetBalance(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestGetBalanceReturnsAuthenticatedWallet(t *testing.T) {
	rpc := &handlerRPC{account: usecase.Account{Address: handlerPrimary, Balance: 250000, Type: "basic"}}
	handler := NewHandler(usecase.NewWalletUseCase(handlerIdentities{addresses: []string{handlerPrimary}}, rpc, "test-albatross"))
	userID := uuid.New()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wallet/balance", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, userID))
	rec := httptest.NewRecorder()
	handler.GetBalance(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	data := payload["data"].(map[string]any)
	if data["address"] != handlerPrimary || data["balance_lunas"] != "250000" || data["balance_nim"] != "2.5" {
		t.Fatalf("data = %#v", data)
	}
	if rpc.seenAddress != handlerPrimary {
		t.Fatalf("seen = %q", rpc.seenAddress)
	}
}

func TestGetBalanceRejectsUnrelatedWalletAddress(t *testing.T) {
	unrelated := iamUC.AddressFromPublicKey(bytes.Repeat([]byte{9}, 32))
	rpc := &handlerRPC{account: usecase.Account{Balance: 1}}
	handler := NewHandler(usecase.NewWalletUseCase(handlerIdentities{addresses: []string{handlerPrimary}}, rpc, ""))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wallet/balance?address="+url.QueryEscape(unrelated), nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, uuid.New()))
	rec := httptest.NewRecorder()
	handler.GetBalance(rec, req)
	if rec.Code != http.StatusForbidden {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), `"balance_lunas"`) {
		t.Fatal("unrelated wallet data was returned")
	}
	if rpc.seenAddress != "" {
		t.Fatalf("unrelated address reached RPC: %q", rpc.seenAddress)
	}
}

func TestGetTransactionsRequiresAuthentication(t *testing.T) {
	handler := NewHandler(usecase.NewWalletUseCase(handlerIdentities{addresses: []string{handlerPrimary}}, &handlerRPC{}, ""))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wallet/transactions", nil)
	rec := httptest.NewRecorder()
	handler.GetTransactions(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestGetBalanceReportsUnconfiguredRPCWithoutLeakingInternals(t *testing.T) {
	handler := NewHandler(usecase.NewWalletUseCase(handlerIdentities{addresses: []string{handlerPrimary}}, nil, "test-albatross"))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wallet/balance", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, uuid.New()))
	rec := httptest.NewRecorder()
	handler.GetBalance(rec, req)
	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	body := rec.Body.String()
	if strings.Contains(body, "http://") || strings.Contains(body, "password") || strings.Contains(body, "rpc-user") {
		t.Fatalf("internal details leaked: %s", body)
	}
	if !strings.Contains(body, "an internal error occurred") {
		t.Fatalf("body = %s", body)
	}
}

func TestGetTransactionsEmptyHistory(t *testing.T) {
	handler := NewHandler(usecase.NewWalletUseCase(handlerIdentities{addresses: []string{handlerPrimary}}, &handlerRPC{transactions: []usecase.Transaction{}}, ""))
	req := httptest.NewRequest(http.MethodGet, "/api/v1/wallet/transactions", nil)
	req = req.WithContext(context.WithValue(req.Context(), middleware.ContextKeyUserID, uuid.New()))
	rec := httptest.NewRecorder()
	handler.GetTransactions(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body=%s", rec.Code, rec.Body.String())
	}
	var payload struct {
		Data []map[string]any `json:"data"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	if payload.Data == nil || len(payload.Data) != 0 {
		t.Fatalf("data = %#v", payload.Data)
	}
}
