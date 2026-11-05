package consul

import (
	"reflect"
	"testing"

	"github.com/hashicorp/consul/api"
)

func TestGRPCResolverAddressesUsesServiceAddressAndFiltersDuplicates(t *testing.T) {
	entries := []*api.ServiceEntry{
		{Node: &api.Node{Address: "10.0.0.1"}, Service: &api.AgentService{Address: "10.0.1.2", Port: 9086}},
		{Node: &api.Node{Address: "10.0.0.1"}, Service: &api.AgentService{Address: "10.0.1.2", Port: 9086}},
		{Node: &api.Node{Address: "10.0.0.3"}, Service: &api.AgentService{Port: 9086}},
		nil,
	}

	addresses := grpcResolverAddresses(entries)
	got := make([]string, len(addresses))
	for i := range addresses {
		got[i] = addresses[i].Addr
	}
	want := []string{"10.0.0.3:9086", "10.0.1.2:9086"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("addresses = %v, want %v", got, want)
	}
}
