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

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"

	ffv1 "github.com/osac-project/fulfillment-service/internal/api/fulfillment/v1"
	"github.com/osac-project/fulfillment-service/internal/database"
	"github.com/osac-project/fulfillment-service/internal/database/dao"
)

var _ = Describe("Subnets server", func() {
	var (
		ctx            context.Context
		tx             database.Tx
		subnetsServer *SubnetsServer
	)

	BeforeEach(func() {
		var err error

		// Create a context:
		ctx = context.Background()

		// Prepare the database pool:
		db := server.MakeDatabase()
		DeferCleanup(db.Close)
		pool, err := pgxpool.New(ctx, db.MakeURL())
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(pool.Close)

		// Create the transaction manager:
		tm, err := database.NewTxManager().
			SetLogger(logger).
			SetPool(pool).
			Build()
		Expect(err).ToNot(HaveOccurred())

		// Start a transaction and add it to the context:
		tx, err = tm.Begin(ctx)
		Expect(err).ToNot(HaveOccurred())
		DeferCleanup(func() {
			err := tm.End(ctx, tx)
			Expect(err).ToNot(HaveOccurred())
		})
		ctx = database.TxIntoContext(ctx, tx)

		// Create the tables:
		err = dao.CreateTables(ctx, "subnets", "virtual_networks", "network_classes")
		Expect(err).ToNot(HaveOccurred())

		// Create the server:
		subnetsServer, err = NewSubnetsServer().
			SetLogger(logger).
			SetAttributionLogic(attribution).
			SetTenancyLogic(tenancy).
			Build()
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Creation", func() {
		It("Can be built if all the required parameters are set", func() {
			server, err := NewSubnetsServer().
				SetLogger(logger).
				SetAttributionLogic(attribution).
				SetTenancyLogic(tenancy).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(server).ToNot(BeNil())
		})

		It("Fails if logger is not set", func() {
			server, err := NewSubnetsServer().
				SetAttributionLogic(attribution).
				SetTenancyLogic(tenancy).
				Build()
			Expect(err).To(MatchError("logger is mandatory"))
			Expect(server).To(BeNil())
		})

		It("Fails if tenancy logic is not set", func() {
			server, err := NewSubnetsServer().
				SetLogger(logger).
				SetAttributionLogic(attribution).
				Build()
			Expect(err).To(MatchError("tenancy logic is mandatory"))
			Expect(server).To(BeNil())
		})
	})

	Describe("Public API CRUD operations", func() {
		It("Creates, gets, lists, and deletes a subnet", func() {
			// Create parent VirtualNetwork in READY state
			vn := createVirtualNetworkForTests(ctx, "10.0.0.0/16", "")

			// Create a subnet
			createRequest := ffv1.SubnetsCreateRequest_builder{
				Object: ffv1.Subnet_builder{
					Spec: ffv1.SubnetSpec_builder{
						Ipv4Cidr:       proto.String("10.0.1.0/24"),
						VirtualNetwork: vn.GetId(),
					}.Build(),
				}.Build(),
			}.Build()

			createResponse, err := subnetsServer.Create(ctx, createRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(createResponse).ToNot(BeNil())
			subnet := createResponse.GetObject()
			Expect(subnet.GetId()).ToNot(BeEmpty())
			Expect(subnet.GetSpec().GetIpv4Cidr()).To(Equal("10.0.1.0/24"))
			Expect(subnet.GetSpec().GetVirtualNetwork()).To(Equal(vn.GetId()))

			// Get the subnet
			getRequest := ffv1.SubnetsGetRequest_builder{
				Id: subnet.GetId(),
			}.Build()

			getResponse, err := subnetsServer.Get(ctx, getRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(getResponse).ToNot(BeNil())
			Expect(getResponse.GetObject().GetId()).To(Equal(subnet.GetId()))

			// List subnets
			listRequest := ffv1.SubnetsListRequest_builder{}.Build()
			listResponse, err := subnetsServer.List(ctx, listRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(listResponse).ToNot(BeNil())
			Expect(listResponse.GetItems()).To(HaveLen(1))
			Expect(listResponse.GetItems()[0].GetId()).To(Equal(subnet.GetId()))

			// Delete the subnet
			deleteRequest := ffv1.SubnetsDeleteRequest_builder{
				Id: subnet.GetId(),
			}.Build()

			_, err = subnetsServer.Delete(ctx, deleteRequest)
			Expect(err).ToNot(HaveOccurred())

			// Verify deletion
			listAfterDelete, err := subnetsServer.List(ctx, listRequest)
			Expect(err).ToNot(HaveOccurred())
			Expect(listAfterDelete.GetItems()).To(BeEmpty())
		})
	})
})
