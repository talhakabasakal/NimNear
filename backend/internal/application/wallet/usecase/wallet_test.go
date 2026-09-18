package usecase

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	iamUC "github.com/masterfabric-go/masterfabric/internal/application/iam/usecase"
	"github.com/masterfabric-go/masterfabric/internal/application/wallet/dto"
	domainErr "github.com/masterfabric-go/masterfabric/internal/shared/errors"
)

const primaryAddress = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"

func derivedAddress(seed byte) string {
	return iamUC.AddressFromPublicKey(bytes.Repeat([]byte{seed}, 32))
}

type identityStub struct {
	addresses []string
	err       error
}

func (s identityStub) VerifiedAddresses(context.Context, uuid.UUID) ([]string, error) {
	return s.addresses, s.err
}

type accountStub struct {
	account      Account
	accountErr   error
	transactions []Transaction
	txErr        error
	seenAddress  string
	seenMax      int
	seenStartAt  string
}

func (s *accountStub) GetAccountByAddress(_ context.Context, address string) (Account, error) {
	s.seenAddress = address
	return s.account, s.accountErr
}

func (s *accountStub) GetTransactionsByAddress(_ context.Context, address string, max int, startAt string) ([]Transaction, error) {
	s.seenAddress = address
	s.seenMax = max
	s.seenStartAt = startAt
	if s.transactions == nil {
		return []Transaction{}, s.txErr
	}
	return s.transactions, s.txErr
}

func TestGetBalanceUsesPrimaryVerifiedIdentity(t *testing.T) {
	rpc := &accountStub{account: Account{Address: primaryAddress, Balance: 123456789, Type: "basic"}}
	uc := NewWalletUseCase(identityStub{addresses: []string{primaryAddress, derivedAddress(1)}}, rpc, "test-albatross")
	result, err := uc.GetBalance(context.Background(), dto.WalletQuery{UserID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if result.Data.Address != primaryAddress || result.Data.BalanceLunas != "123456789" || result.Data.BalanceNIM != "1234.56789" {
		t.Fatalf("balance = %+v", result.Data)
	}
	if result.Data.Network != "test-albatross" || rpc.seenAddress != primaryAddress {
		t.Fatalf("network=%q seen=%q", result.Data.Network, rpc.seenAddress)
	}
}

func TestGetBalancePreservesLargeLunaPrecision(t *testing.T) {
	rpc := &accountStub{account: Account{Balance: 9007199254740991}}
	uc := NewWalletUseCase(identityStub{addresses: []string{primaryAddress}}, rpc, "")
	result, err := uc.GetBalance(context.Background(), dto.WalletQuery{UserID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if result.Data.BalanceLunas != "9007199254740991" || result.Data.BalanceNIM != "90071992547.40991" {
		t.Fatalf("precision lost: %+v", result.Data)
	}
}

func TestWalletAuthorization(t *testing.T) {
	secondaryAddress := derivedAddress(1)
	unrelatedAddress := derivedAddress(2)
	rpc := &accountStub{account: Account{Balance: 1}}
	uc := NewWalletUseCase(identityStub{addresses: []string{primaryAddress, secondaryAddress}}, rpc, "")
	userID := uuid.New()

	t.Run("unauthenticated", func(t *testing.T) {
		_, err := uc.GetBalance(context.Background(), dto.WalletQuery{})
		if !errors.Is(err, domainErr.ErrUnauthorized) {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("no verified identity", func(t *testing.T) {
		empty := NewWalletUseCase(identityStub{}, rpc, "")
		_, err := empty.GetBalance(context.Background(), dto.WalletQuery{UserID: userID})
		if domainErr.ErrorCode(err) != "no_verified_nimiq_identity" {
			t.Fatalf("error = %v", err)
		}
	})
	t.Run("unrelated address is forbidden", func(t *testing.T) {
		_, err := uc.GetBalance(context.Background(), dto.WalletQuery{UserID: userID, Address: unrelatedAddress})
		if !errors.Is(err, domainErr.ErrForbidden) {
			t.Fatalf("error = %v", err)
		}
		if rpc.seenAddress != "" && rpc.seenAddress == unrelatedAddress {
			t.Fatal("unrelated address was sent to RPC")
		}
	})
	t.Run("invalid address is rejected before RPC", func(t *testing.T) {
		rpc.seenAddress = ""
		_, err := uc.GetBalance(context.Background(), dto.WalletQuery{UserID: userID, Address: "NQ..."})
		if domainErr.ErrorCode(err) != "invalid_wallet_address" {
			t.Fatalf("error = %v", err)
		}
		if rpc.seenAddress != "" {
			t.Fatal("invalid address was sent to RPC")
		}
	})
	t.Run("blank address query uses primary identity", func(t *testing.T) {
		rpc.seenAddress = ""
		_, err := uc.GetBalance(context.Background(), dto.WalletQuery{UserID: userID, Address: "   "})
		if err != nil {
			t.Fatalf("blank address should fall back to primary, got %v", err)
		}
		if rpc.seenAddress != primaryAddress {
			t.Fatalf("seen = %q", rpc.seenAddress)
		}
	})
	t.Run("second verified identity is allowed", func(t *testing.T) {
		rpc.seenAddress = ""
		result, err := uc.GetBalance(context.Background(), dto.WalletQuery{UserID: userID, Address: strings.ReplaceAll(secondaryAddress, " ", "")})
		if err != nil {
			t.Fatal(err)
		}
		if result.Data.Address != secondaryAddress || rpc.seenAddress != secondaryAddress {
			t.Fatalf("address = %q seen = %q", result.Data.Address, rpc.seenAddress)
		}
	})
}

func TestGetTransactionsNormalizesOfficialFields(t *testing.T) {
	ok := true
	hash := strings.Repeat("ab", 32)
	secondaryAddress := derivedAddress(1)
	rpc := &accountStub{transactions: []Transaction{{
		Hash:            hash,
		From:            primaryAddress,
		To:              secondaryAddress,
		Value:           250000,
		BlockNumber:     120,
		Timestamp:       1720000000000,
		NetworkID:       5,
		Network:         "TestAlbatross",
		ExecutionResult: &ok,
	}}}
	uc := NewWalletUseCase(identityStub{addresses: []string{primaryAddress}}, rpc, "test-albatross")
	result, err := uc.GetTransactions(context.Background(), dto.WalletQuery{UserID: uuid.New(), Max: 10})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Data) != 1 {
		t.Fatalf("len = %d", len(result.Data))
	}
	tx := result.Data[0]
	if tx.Hash != hash || tx.Sender != primaryAddress || tx.Recipient != secondaryAddress {
		t.Fatalf("identity fields = %+v", tx)
	}
	if tx.ValueLunas != "250000" || tx.ValueNIM != "2.5" {
		t.Fatalf("value = %s / %s", tx.ValueLunas, tx.ValueNIM)
	}
	if tx.BlockNumber == nil || *tx.BlockNumber != 120 || tx.Timestamp == nil || *tx.Timestamp != 1720000000000 {
		t.Fatalf("inclusion fields = %+v", tx)
	}
	if tx.Network != "TestAlbatross" || tx.NetworkID == nil || *tx.NetworkID != 5 || tx.Status != "successful" {
		t.Fatalf("network/status = %+v", tx)
	}
	if result.NextStartAt != "" {
		t.Fatalf("partial page should not invent a cursor: %q", result.NextStartAt)
	}
}

func TestGetTransactionsEmptyHistory(t *testing.T) {
	rpc := &accountStub{transactions: []Transaction{}}
	uc := NewWalletUseCase(identityStub{addresses: []string{primaryAddress}}, rpc, "")
	result, err := uc.GetTransactions(context.Background(), dto.WalletQuery{UserID: uuid.New()})
	if err != nil {
		t.Fatal(err)
	}
	if result.Data == nil || len(result.Data) != 0 || result.NextStartAt != "" {
		t.Fatalf("result = %+v", result)
	}
}

func TestGetTransactionsRejectsMalformedTransaction(t *testing.T) {
	rpc := &accountStub{transactions: []Transaction{{Value: 1}}}
	uc := NewWalletUseCase(identityStub{addresses: []string{primaryAddress}}, rpc, "")
	_, err := uc.GetTransactions(context.Background(), dto.WalletQuery{UserID: uuid.New()})
	if domainErr.ErrorCode(err) != "malformed_nimiq_transaction" {
		t.Fatalf("error = %v", err)
	}
}

func TestGetTransactionsUsesNativeStartAtPagination(t *testing.T) {
	hash := strings.Repeat("ab", 32)
	rpc := &accountStub{transactions: []Transaction{{
		Hash: hash, From: primaryAddress, To: derivedAddress(1), Value: 1,
	}}}
	uc := NewWalletUseCase(identityStub{addresses: []string{primaryAddress}}, rpc, "")
	result, err := uc.GetTransactions(context.Background(), dto.WalletQuery{UserID: uuid.New(), Max: 1, StartAt: strings.Repeat("cd", 32)})
	if err != nil {
		t.Fatal(err)
	}
	if rpc.seenMax != 1 || rpc.seenStartAt != strings.Repeat("cd", 32) {
		t.Fatalf("query = max %d start %q", rpc.seenMax, rpc.seenStartAt)
	}
	if result.NextStartAt != hash {
		t.Fatalf("next_start_at = %q", result.NextStartAt)
	}
}

func TestFormatNIMUsesCanonicalFiveDecimals(t *testing.T) {
	if got := formatNIM(1); got != "0.00001" {
		t.Fatalf("got %q", got)
	}
	if got := formatNIM(100000); got != "1" {
		t.Fatalf("got %q", got)
	}
}

func TestGetBalanceReportsUnconfiguredRPC(t *testing.T) {
	uc := NewWalletUseCase(identityStub{addresses: []string{primaryAddress}}, nil, "test-albatross")
	_, err := uc.GetBalance(context.Background(), dto.WalletQuery{UserID: uuid.New()})
	if domainErr.ErrorCode(err) != "nimiq_rpc_unavailable" {
		t.Fatalf("error = %v", err)
	}
}
