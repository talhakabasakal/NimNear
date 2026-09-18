package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

const officialWalletAddress = "NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604"

type capturedRPC struct {
	method string
	params []any
	auth   string
}

func rpcServer(t *testing.T, status int, body string, capture *capturedRPC) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		raw, err := io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("read body: %v", err)
			return
		}
		var payload struct {
			Method string `json:"method"`
			Params []any  `json:"params"`
		}
		if err := json.Unmarshal(raw, &payload); err != nil {
			t.Errorf("decode request: %v body=%s", err, raw)
		}
		if capture != nil {
			capture.method = payload.Method
			capture.params = payload.Params
			capture.auth = r.Header.Get("Authorization")
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write([]byte(body))
	}))
}

func TestGetAccountByAddressParsesValidAccount(t *testing.T) {
	capture := &capturedRPC{}
	server := rpcServer(t, http.StatusOK, `{"jsonrpc":"2.0","result":{"data":{"address":"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604","balance":123456789,"type":"basic"},"metadata":{"blockNumber":10,"blockHash":"aa"}},"id":1}`, capture)
	defer server.Close()

	account, err := NewClient(server.URL).GetAccountByAddress(context.Background(), officialWalletAddress)
	if err != nil {
		t.Fatal(err)
	}
	if capture.method != "getAccountByAddress" {
		t.Fatalf("method = %q", capture.method)
	}
	if len(capture.params) != 1 || capture.params[0] != officialWalletAddress {
		t.Fatalf("params = %#v", capture.params)
	}
	if account.Address != officialWalletAddress || uint64(account.Balance) != 123456789 || account.Type != "basic" {
		t.Fatalf("account = %+v", account)
	}
}

func TestGetBalancePreservesLargeLunaPrecision(t *testing.T) {
	const large = "9007199254740991"
	server := rpcServer(t, http.StatusOK, `{"jsonrpc":"2.0","result":{"data":{"address":"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604","balance":`+large+`,"type":"basic"}},"id":1}`, nil)
	defer server.Close()

	balance, err := NewClient(server.URL).GetBalance(context.Background(), officialWalletAddress)
	if err != nil {
		t.Fatal(err)
	}
	if balance != 9007199254740991 {
		t.Fatalf("balance = %d", balance)
	}
}

func TestGetAccountByAddressRejectsMalformedResponse(t *testing.T) {
	server := rpcServer(t, http.StatusOK, `{"jsonrpc":"2.0","result":{"data":{"address":"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604","balance":"not-a-number"}},"id":1}`, nil)
	defer server.Close()

	if _, err := NewClient(server.URL).GetAccountByAddress(context.Background(), officialWalletAddress); err == nil {
		t.Fatal("expected malformed balance to be rejected")
	}
}

func TestRPCJSONRPCErrorIsReturned(t *testing.T) {
	server := rpcServer(t, http.StatusOK, `{"jsonrpc":"2.0","error":{"code":-32602,"message":"Invalid params"},"id":1}`, nil)
	defer server.Close()

	_, err := NewClient(server.URL).GetAccountByAddress(context.Background(), officialWalletAddress)
	if err == nil || !strings.Contains(err.Error(), "Nimiq RPC error -32602") {
		t.Fatalf("error = %v", err)
	}
}

func TestRPCHTTPErrorDoesNotLeakCredentials(t *testing.T) {
	server := rpcServer(t, http.StatusInternalServerError, `node exploded`, nil)
	defer server.Close()

	client := NewClient("http://super:secret@" + strings.TrimPrefix(server.URL, "http://"))
	_, err := client.GetAccountByAddress(context.Background(), officialWalletAddress)
	if err == nil {
		t.Fatal("expected HTTP error")
	}
	if strings.Contains(err.Error(), "secret") || strings.Contains(err.Error(), "super") {
		t.Fatalf("credentials leaked: %v", err)
	}
}

func TestRPCAuthenticationFailure(t *testing.T) {
	capture := &capturedRPC{}
	server := rpcServer(t, http.StatusUnauthorized, `unauthorized`, capture)
	defer server.Close()

	client := NewClient("http://rpc-user:rpc-pass@" + strings.TrimPrefix(server.URL, "http://"))
	_, err := client.GetBalance(context.Background(), officialWalletAddress)
	if !errors.Is(err, ErrRPCAuthenticationFailed) {
		t.Fatalf("error = %v", err)
	}
	if !strings.HasPrefix(capture.auth, "Basic ") {
		t.Fatalf("Authorization = %q", capture.auth)
	}
	if strings.Contains(client.endpoint, "rpc-pass") || strings.Contains(client.endpoint, "rpc-user") {
		t.Fatalf("endpoint retained credentials: %s", client.endpoint)
	}
}

func TestRPCTimeoutIsHandled(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		time.Sleep(50 * time.Millisecond)
	}))
	defer server.Close()

	client := NewClient(server.URL)
	client.httpClient.Timeout = 10 * time.Millisecond
	_, err := client.GetAccountByAddress(context.Background(), officialWalletAddress)
	if !errors.Is(err, ErrRPCTimeout) && !errors.Is(err, ErrRPCUnavailable) {
		t.Fatalf("error = %v", err)
	}
	if err != nil && (strings.Contains(err.Error(), "http://") || strings.Contains(err.Error(), server.URL)) {
		t.Fatalf("timeout error leaked endpoint: %v", err)
	}
}

func TestGetTransactionsByAddressNormalizesOfficialFields(t *testing.T) {
	capture := &capturedRPC{}
	ok := true
	server := rpcServer(t, http.StatusOK, `{
		"jsonrpc":"2.0",
		"result":{"data":[{
			"hash":"`+strings.Repeat("ab", 32)+`",
			"blockNumber":120,
			"timestamp":1720000000000,
			"from":"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604",
			"to":"NQ07 0000 0000 0000 0000 0000 0000 0000 0000",
			"value":9007199254740991,
			"networkId":5,
			"executionResult":true
		}]},
		"id":1
	}`, capture)
	defer server.Close()

	txs, err := NewClient(server.URL).GetTransactionsByAddress(context.Background(), officialWalletAddress, TransactionQuery{Max: 25, StartAt: strings.Repeat("cd", 32)})
	if err != nil {
		t.Fatal(err)
	}
	if capture.method != "getTransactionsByAddress" {
		t.Fatalf("method = %q", capture.method)
	}
	if len(capture.params) != 3 || capture.params[0] != officialWalletAddress {
		t.Fatalf("params = %#v", capture.params)
	}
	if max, ok := capture.params[1].(float64); !ok || max != 25 {
		t.Fatalf("max param = %#v", capture.params[1])
	}
	if capture.params[2] != strings.Repeat("cd", 32) {
		t.Fatalf("startAt param = %#v", capture.params[2])
	}
	if len(txs) != 1 {
		t.Fatalf("len = %d", len(txs))
	}
	tx := txs[0]
	if tx.From != officialWalletAddress || tx.To != "NQ07 0000 0000 0000 0000 0000 0000 0000 0000" {
		t.Fatalf("addresses = %q -> %q", tx.From, tx.To)
	}
	if uint64(tx.Value) != 9007199254740991 || uint64(tx.BlockNumber) != 120 || uint64(tx.Timestamp) != 1720000000000 || uint64(tx.NetworkID) != 5 {
		t.Fatalf("tx = %+v", tx)
	}
	if tx.ExecutionResult == nil || *tx.ExecutionResult != ok {
		t.Fatalf("executionResult = %v", tx.ExecutionResult)
	}
}

func TestGetTransactionsByAddressEmptyHistory(t *testing.T) {
	capture := &capturedRPC{}
	server := rpcServer(t, http.StatusOK, `{"jsonrpc":"2.0","result":{"data":[]},"id":1}`, capture)
	defer server.Close()

	txs, err := NewClient(server.URL).GetTransactionsByAddress(context.Background(), officialWalletAddress, TransactionQuery{})
	if err != nil {
		t.Fatal(err)
	}
	if txs == nil || len(txs) != 0 {
		t.Fatalf("txs = %#v", txs)
	}
	if len(capture.params) != 3 || capture.params[2] != nil {
		t.Fatalf("startAt should be JSON null, params=%#v", capture.params)
	}
}

func TestGetTransactionsByAddressRejectsMalformedValue(t *testing.T) {
	server := rpcServer(t, http.StatusOK, `{"jsonrpc":"2.0","result":{"data":[{"hash":"aa","from":"NQ46 KLJE 5TMF 4Y1A 1255 CJHJ YG1S H0NU T604","to":"NQ07 0000 0000 0000 0000 0000 0000 0000 0000","value":1.5}]},"id":1}`, nil)
	defer server.Close()

	if _, err := NewClient(server.URL).GetTransactionsByAddress(context.Background(), officialWalletAddress, TransactionQuery{}); err == nil {
		t.Fatal("expected malformed transaction value to be rejected")
	}
}

func TestWalletRPCMethodsRejectEmptyAddress(t *testing.T) {
	client := NewClient("http://127.0.0.1:1")
	if _, err := client.GetAccountByAddress(context.Background(), "  "); err == nil {
		t.Fatal("expected empty address rejection")
	}
	if _, err := client.GetTransactionsByAddress(context.Background(), "", TransactionQuery{}); err == nil {
		t.Fatal("expected empty address rejection")
	}
}

func TestNetworkNameForIDUsesKnownAlbatrossIdentifiers(t *testing.T) {
	if got := NetworkNameForID(24); got != "MainAlbatross" {
		t.Fatalf("got %q", got)
	}
	if got := NetworkNameForID(5); got != "TestAlbatross" {
		t.Fatalf("got %q", got)
	}
	if got := NetworkNameForID(99); got != "" {
		t.Fatalf("unknown id should not invent a name, got %q", got)
	}
}
