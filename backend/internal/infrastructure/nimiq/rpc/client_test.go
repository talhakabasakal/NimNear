package rpc

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/masterfabric-go/masterfabric/internal/application/eventpurchase/verification"
	"github.com/masterfabric-go/masterfabric/internal/domain/eventpurchase/model"
)

const (
	verifiedSender  = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"
	secondSender    = "NQ07 0000 0000 0000 0000 0000 0000 0000 0000"
	unrelatedSender = "NQ88 0000 0000 0000 0000 0000 0000 0000 0000"
	merchantAddress = "NQAB 0000 0000 0000 0000 0000 0000 0000 0000"
)

type fakeRPC struct {
	tx       Transaction
	txErr    error
	block    Block
	blockErr error
	batch    uint64
	batchErr error
}

func (f *fakeRPC) GetTransactionByHash(context.Context, string) (Transaction, error) {
	return f.tx, f.txErr
}

func (f *fakeRPC) GetBlockByNumber(context.Context, uint64) (Block, error) {
	return f.block, f.blockErr
}

func (f *fakeRPC) GetBatchNumber(context.Context) (uint64, error) {
	return f.batch, f.batchErr
}

type staticSenders struct {
	addresses []string
	err       error
}

func (s staticSenders) VerifiedAddresses(context.Context, uuid.UUID) ([]string, error) {
	return s.addresses, s.err
}

func baseVerificationFixture() (*NimiqVerifier, *model.Purchase, *fakeRPC) {
	hash := strings.Repeat("a", 64)
	executionOK := true
	rpc := &fakeRPC{
		tx: Transaction{
			Hash:            hash,
			BlockNumber:     uint64Value(120),
			From:            verifiedSender,
			To:              merchantAddress,
			FromType:        uint64Value(0),
			ToType:          uint64Value(0),
			Value:           uint64Value(250000),
			Flags:           uint64Value(0),
			NetworkID:       uint64Value(24),
			ExecutionResult: &executionOK,
		},
		block: Block{Batch: uint64Value(10), Network: "MainAlbatross", Type: "micro"},
		batch: 11,
	}
	purchase := &model.Purchase{ID: uuid.New(), UserID: uuid.New(), AmountLunas: 250000, TransactionHash: &hash}
	return NewVerifier(rpc, merchantAddress, "MainAlbatross", staticSenders{addresses: []string{verifiedSender}}), purchase, rpc
}

func TestNimiqVerifierOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*NimiqVerifier, *fakeRPC, *model.Purchase)
		want    verification.Outcome
		wantErr bool
	}{
		{name: "finalized valid basic transfer", want: verification.OutcomeConfirmed},
		{name: "correct sender is accepted", want: verification.OutcomeConfirmed},
		{name: "wrong sender is invalid", mutate: func(_ *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) { rpc.tx.From = unrelatedSender }, want: verification.OutcomeInvalid},
		{name: "missing sender identity is invalid", mutate: func(v *NimiqVerifier, _ *fakeRPC, _ *model.Purchase) {
			v.senders = staticSenders{}
		}, want: verification.OutcomeInvalid},
		{name: "sender lookup failure is retryable", mutate: func(v *NimiqVerifier, _ *fakeRPC, _ *model.Purchase) {
			v.senders = staticSenders{err: errors.New("identity store unavailable")}
		}, want: verification.OutcomeNotFound, wantErr: true},
		{name: "correct recipient is accepted", want: verification.OutcomeConfirmed},
		{name: "wrong recipient is invalid", mutate: func(_ *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) {
			rpc.tx.To = "NQZZ 0000 0000 0000 0000 0000 0000 0000 0000"
		}, want: verification.OutcomeInvalid},
		{name: "correct amount is accepted", want: verification.OutcomeConfirmed},
		{name: "wrong amount is invalid", mutate: func(_ *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) { rpc.tx.Value = uint64Value(250001) }, want: verification.OutcomeInvalid},
		{name: "wrong network is invalid", mutate: func(_ *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) { rpc.block.Network = "TestAlbatross" }, want: verification.OutcomeInvalid},
		{name: "auth-style MainAlbatross spelling accepts consensus block network", mutate: func(v *NimiqVerifier, _ *fakeRPC, _ *model.Purchase) {
			v.network = "main-albatross"
		}, want: verification.OutcomeConfirmed},
		{name: "wrong network id is invalid", mutate: func(_ *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) { rpc.tx.NetworkID = uint64Value(5) }, want: verification.OutcomeInvalid},
		{name: "not found remains submitted", mutate: func(_ *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) { rpc.txErr = ErrTransactionNotFound }, want: verification.OutcomeNotFound},
		{name: "non-final micro block is not confirmed", mutate: func(_ *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) { rpc.batch = 10 }, want: verification.OutcomeNotFinal},
		{name: "non-basic transfer is invalid", mutate: func(_ *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) { rpc.tx.ToType = uint64Value(1) }, want: verification.OutcomeInvalid},
		{name: "failed execution is invalid", mutate: func(_ *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) {
			ok := false
			rpc.tx.ExecutionResult = &ok
		}, want: verification.OutcomeInvalid},
		{name: "rpc failure is retryable", mutate: func(_ *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) { rpc.txErr = errors.New("temporary outage") }, want: verification.OutcomeNotFound, wantErr: true},
		{name: "any verified identity is accepted", mutate: func(v *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) {
			v.senders = staticSenders{addresses: []string{verifiedSender, secondSender}}
			rpc.tx.From = secondSender
		}, want: verification.OutcomeConfirmed},
		{name: "unrelated sender is rejected when multiple identities exist", mutate: func(v *NimiqVerifier, rpc *fakeRPC, _ *model.Purchase) {
			v.senders = staticSenders{addresses: []string{verifiedSender, secondSender}}
			rpc.tx.From = unrelatedSender
		}, want: verification.OutcomeInvalid},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier, purchase, rpc := baseVerificationFixture()
			if tt.mutate != nil {
				tt.mutate(verifier, rpc, purchase)
			}
			got, err := verifier.Verify(context.Background(), purchase)
			if (err != nil) != tt.wantErr {
				t.Fatalf("error = %v, wantErr %v", err, tt.wantErr)
			}
			if got != tt.want {
				t.Fatalf("outcome = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestVerifyTransferUsesPaymentRequestRecipientAndPayer(t *testing.T) {
	verifier, _, rpc := baseVerificationFixture()
	recipient := "NQ88 0000 0000 0000 0000 0000 0000 0000 0000"
	rpc.tx.To = recipient
	payerID := uuid.New()
	hash := strings.Repeat("a", 64)
	got, err := verifier.VerifyTransfer(context.Background(), verification.Transfer{
		TransactionHash: hash,
		Recipient:       recipient,
		AmountLunas:     250000,
		PayerUserID:     payerID,
	})
	if err != nil {
		t.Fatalf("VerifyTransfer: %v", err)
	}
	if got != verification.OutcomeConfirmed {
		t.Fatalf("outcome = %q, want confirmed", got)
	}

	got, err = verifier.VerifyTransfer(context.Background(), verification.Transfer{
		TransactionHash: hash,
		Recipient:       merchantAddress,
		AmountLunas:     250000,
		PayerUserID:     payerID,
	})
	if err != nil {
		t.Fatalf("VerifyTransfer merchant mismatch: %v", err)
	}
	if got != verification.OutcomeInvalid {
		t.Fatalf("outcome = %q, want invalid when recipient is the merchant instead of the request recipient", got)
	}
}

func TestNewClientStripsUserinfoFromStoredEndpoint(t *testing.T) {
	client := NewClient("https://rpcuser:super-secret@rpc.example.com/v1")
	if strings.Contains(client.endpoint, "super-secret") || strings.Contains(client.endpoint, "rpcuser") {
		t.Fatalf("stored endpoint leaked userinfo: %q", client.endpoint)
	}
	if client.username != "rpcuser" || client.password != "super-secret" {
		t.Fatalf("basic auth not preserved: %q", client.username)
	}
}

func TestGetBatchNumberRecordsReadOnlyHealthProbe(t *testing.T) {
	capture := &capturedRPC{}
	server := rpcServer(t, 200, `{"jsonrpc":"2.0","result":{"data":11},"id":1}`, capture)
	defer server.Close()
	got, err := NewClient(server.URL).GetBatchNumber(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got != 11 || capture.method != "getBatchNumber" {
		t.Fatalf("batch=%d method=%q", got, capture.method)
	}
}
