/*
Copyright (c) 2026 Red Hat Inc.

Licensed under the Apache License, Version 2.0 (the "License"); you may not use this file except in compliance with the
License. You may obtain a copy of the License at

  http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software distributed under the License is distributed on an
"AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied. See the License for the specific
language governing permissions and limitations under the License.
*/

package servers

import (
	"context"

	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	privatev1 "github.com/osac-project/fulfillment-service/internal/api/private/v1"
	"github.com/osac-project/fulfillment-service/internal/database/dao"
)

// createNetworkClassForTests creates a NetworkClass in READY state for use in tests.
// Returns the NetworkClass object with its ImplementationStrategy set.
func createNetworkClassForTests(ctx context.Context) *privatev1.NetworkClass {
	// Create NetworkClass DAO
	ncDao, err := dao.NewGenericDAO[*privatev1.NetworkClass]().
		SetLogger(logger).
		SetTable("network_classes").
		SetAttributionLogic(attribution).
		SetTenancyLogic(tenancy).
		Build()
	Expect(err).ToNot(HaveOccurred())

	// Create NetworkClass with READY state
	nc := privatev1.NetworkClass_builder{
		ImplementationStrategy: "test-strategy",
		Status: privatev1.NetworkClassStatus_builder{
			State: privatev1.NetworkClassState_NETWORK_CLASS_STATE_READY,
		}.Build(),
	}.Build()

	response, err := ncDao.Create().
		SetObject(nc).
		Do(ctx)
	Expect(err).ToNot(HaveOccurred())

	return response.GetObject()
}

// createVirtualNetworkForTests creates a VirtualNetwork in READY state for use in Subnet tests.
// Automatically creates a parent NetworkClass if needed.
func createVirtualNetworkForTests(ctx context.Context, ipv4Cidr, ipv6Cidr string) *privatev1.VirtualNetwork {
	// Ensure NetworkClass exists
	nc := createNetworkClassForTests(ctx)

	// Create VirtualNetwork DAO
	vnDao, err := dao.NewGenericDAO[*privatev1.VirtualNetwork]().
		SetLogger(logger).
		SetTable("virtual_networks").
		SetAttributionLogic(attribution).
		SetTenancyLogic(tenancy).
		Build()
	Expect(err).ToNot(HaveOccurred())

	builder := privatev1.VirtualNetwork_builder{
		Spec: privatev1.VirtualNetworkSpec_builder{
			NetworkClass: nc.GetImplementationStrategy(),
			Region:       "us-west-1",
		}.Build(),
		Status: privatev1.VirtualNetworkStatus_builder{
			State: privatev1.VirtualNetworkState_VIRTUAL_NETWORK_STATE_READY,
		}.Build(),
	}

	// Add IPv4 CIDR if provided
	if ipv4Cidr != "" {
		builder.Spec.Ipv4Cidr = proto.String(ipv4Cidr)
	}

	// Add IPv6 CIDR if provided
	if ipv6Cidr != "" {
		builder.Spec.Ipv6Cidr = proto.String(ipv6Cidr)
	}

	vn := builder.Build()

	response, err := vnDao.Create().
		SetObject(vn).
		Do(ctx)
	Expect(err).ToNot(HaveOccurred())

	return response.GetObject()
}
