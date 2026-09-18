package nimiq

import "testing"

func TestParseNetworkCanonicalEnvironments(t *testing.T) {
	cases := []struct {
		input         string
		authName      string
		consensusName string
		environment   string
		id            uint64
	}{
		{input: "test-albatross", authName: AuthNetworkTest, consensusName: ConsensusTest, environment: EnvironmentTestnet, id: NetworkIDTest},
		{input: "TestAlbatross", authName: AuthNetworkTest, consensusName: ConsensusTest, environment: EnvironmentTestnet, id: NetworkIDTest},
		{input: "testnet", authName: AuthNetworkTest, consensusName: ConsensusTest, environment: EnvironmentTestnet, id: NetworkIDTest},
		{input: "main-albatross", authName: AuthNetworkMain, consensusName: ConsensusMain, environment: EnvironmentMainnet, id: NetworkIDMain},
		{input: "MainAlbatross", authName: AuthNetworkMain, consensusName: ConsensusMain, environment: EnvironmentMainnet, id: NetworkIDMain},
		{input: "mainnet", authName: AuthNetworkMain, consensusName: ConsensusMain, environment: EnvironmentMainnet, id: NetworkIDMain},
	}
	for _, tt := range cases {
		got, err := ParseNetwork(tt.input)
		if err != nil {
			t.Fatalf("ParseNetwork(%q): %v", tt.input, err)
		}
		if got.AuthName != tt.authName || got.ConsensusName != tt.consensusName || got.Environment != tt.environment || got.ID != tt.id {
			t.Fatalf("ParseNetwork(%q)=%#v", tt.input, got)
		}
	}
}

func TestParseNetworkRejectsUnknown(t *testing.T) {
	for _, input := range []string{"", "ethereum", "devnet", "albatross"} {
		if _, err := ParseNetwork(input); err == nil {
			t.Fatalf("ParseNetwork(%q) accepted unknown network", input)
		}
	}
}

func TestNetworkIDs(t *testing.T) {
	if NetworkIDTest != 5 {
		t.Fatalf("TestAlbatross id=%d", NetworkIDTest)
	}
	if NetworkIDMain != 24 {
		t.Fatalf("MainAlbatross id=%d", NetworkIDMain)
	}
}

func TestSameEnvironment(t *testing.T) {
	if !SameEnvironment("test-albatross", "TestAlbatross") {
		t.Fatal("auth test and payment TestAlbatross must match")
	}
	if SameEnvironment("test-albatross", "MainAlbatross") {
		t.Fatal("auth test and payment MainAlbatross must not match")
	}
	if SameEnvironment("unknown", "TestAlbatross") {
		t.Fatal("unknown networks must not match")
	}
}
