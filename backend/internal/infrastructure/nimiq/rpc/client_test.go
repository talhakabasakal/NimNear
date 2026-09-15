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

func baseVerificationFixture() (*NimiqVerifier, *model.Purchase, *fakeRPC) {
	hash := strings.Repeat("a", 64)
	executionOK := true
	rpc := &fakeRPC{
		tx: Transaction{
			Hash:            hash,
			BlockNumber:     uint64Value(120),
			To:              "NQAB 0000 0000 0000 0000 0000 0000 0000 0000",
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
	purchase := &model.Purchase{ID: uuid.New(), AmountLunas: 250000, TransactionHash: &hash}
	return NewVerifier(rpc, "NQAB 0000 0000 0000 0000 0000 0000 0000 0000", "MainAlbatross"), purchase, rpc
}

func TestNimiqVerifierOutcomes(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*fakeRPC, *model.Purchase)
		want    verification.Outcome
		wantErr bool
	}{
		{name: "finalized valid basic transfer", want: verification.OutcomeConfirmed},
		{name: "not found remains submitted", mutate: func(rpc *fakeRPC, _ *model.Purchase) { rpc.txErr = ErrTransactionNotFound }, want: verification.OutcomeNotFound},
		{name: "non-final micro block is not confirmed", mutate: func(rpc *fakeRPC, _ *model.Purchase) { rpc.batch = 10 }, want: verification.OutcomeNotFinal},
		{name: "wrong recipient is invalid", mutate: func(rpc *fakeRPC, _ *model.Purchase) { rpc.tx.To = "NQZZ 0000 0000 0000 0000 0000 0000 0000 0000" }, want: verification.OutcomeInvalid},
		{name: "wrong amount is invalid", mutate: func(rpc *fakeRPC, _ *model.Purchase) { rpc.tx.Value = uint64Value(250001) }, want: verification.OutcomeInvalid},
		{name: "wrong network is invalid", mutate: func(rpc *fakeRPC, _ *model.Purchase) { rpc.block.Network = "TestAlbatross" }, want: verification.OutcomeInvalid},
		{name: "non-basic transfer is invalid", mutate: func(rpc *fakeRPC, _ *model.Purchase) { rpc.tx.ToType = uint64Value(1) }, want: verification.OutcomeInvalid},
		{name: "failed execution is invalid", mutate: func(rpc *fakeRPC, _ *model.Purchase) {
			ok := false
			rpc.tx.ExecutionResult = &ok
		}, want: verification.OutcomeInvalid},
		{name: "rpc failure is retryable", mutate: func(rpc *fakeRPC, _ *model.Purchase) { rpc.txErr = errors.New("temporary outage") }, want: verification.OutcomeNotFound, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			verifier, purchase, rpc := baseVerificationFixture()
			if tt.mutate != nil {
				tt.mutate(rpc, purchase)
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
