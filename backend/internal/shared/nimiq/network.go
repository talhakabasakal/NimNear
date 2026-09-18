package nimiq

import (
	"fmt"
	"strings"
)

const (
	// AuthNetworkTest is the canonical Nimiq auth network name for TestAlbatross.
	AuthNetworkTest = "test-albatross"
	// AuthNetworkMain is the canonical Nimiq auth network name for MainAlbatross.
	AuthNetworkMain = "main-albatross"

	// ConsensusTest is the Albatross consensus name used by payment/RPC verification.
	ConsensusTest = "TestAlbatross"
	// ConsensusMain is the Albatross consensus name used by payment/RPC verification.
	ConsensusMain = "MainAlbatross"

	EnvironmentTestnet = "testnet"
	EnvironmentMainnet = "mainnet"

	// NetworkIDTest is the TestAlbatross consensus network identifier.
	NetworkIDTest uint64 = 5
	// NetworkIDMain is the MainAlbatross consensus network identifier.
	NetworkIDMain uint64 = 24
)

// Network is one supported Nimiq Albatross environment.
type Network struct {
	AuthName      string
	ConsensusName string
	Environment   string
	ID            uint64
}

var (
	testNetwork = Network{
		AuthName:      AuthNetworkTest,
		ConsensusName: ConsensusTest,
		Environment:   EnvironmentTestnet,
		ID:            NetworkIDTest,
	}
	mainNetwork = Network{
		AuthName:      AuthNetworkMain,
		ConsensusName: ConsensusMain,
		Environment:   EnvironmentMainnet,
		ID:            NetworkIDMain,
	}
)

// ParseNetwork maps supported auth and payment spellings onto one Nimiq environment.
func ParseNetwork(value string) (Network, error) {
	switch compactNetworkName(value) {
	case "testalbatross", "testnet":
		return testNetwork, nil
	case "mainalbatross", "mainnet":
		return mainNetwork, nil
	default:
		return Network{}, fmt.Errorf("unsupported Nimiq network")
	}
}

// SameEnvironment reports whether two configured network names refer to one Albatross environment.
func SameEnvironment(left, right string) bool {
	parsedLeft, leftErr := ParseNetwork(left)
	parsedRight, rightErr := ParseNetwork(right)
	return leftErr == nil && rightErr == nil && parsedLeft.ID == parsedRight.ID
}

func (n Network) IsTest() bool {
	return n.ID == NetworkIDTest
}

func (n Network) IsMain() bool {
	return n.ID == NetworkIDMain
}

func compactNetworkName(value string) string {
	return strings.ToLower(strings.NewReplacer("-", "", "_", "", " ", "").Replace(strings.TrimSpace(value)))
}
