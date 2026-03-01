/*
Copyright (c) 2025 Red Hat Inc.

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
	"fmt"

	"github.com/jackc/pgx/v5/pgxpool"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/fieldmaskpb"

	ffv1 "github.com/osac-project/fulfillment-service/internal/api/fulfillment/v1"
	privatev1 "github.com/osac-project/fulfillment-service/internal/api/private/v1"
	sharedv1 "github.com/osac-project/fulfillment-service/internal/api/shared/v1"
	"github.com/osac-project/fulfillment-service/internal/database"
	"github.com/osac-project/fulfillment-service/internal/database/dao"
)

var _ = Describe("Virtual networks server", func() {
	var (
		ctx context.Context
		tx  database.Tx
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

		// Create the tables (virtual_networks requires network_classes for validation):
		err = dao.CreateTables(ctx, "network_classes", "virtual_networks")
		Expect(err).ToNot(HaveOccurred())
	})

	Describe("Creation", func() {
		It("Can be built if all the required parameters are set", func() {
			server, err := NewVirtualNetworksServer().
				SetLogger(logger).
				SetAttributionLogic(attribution).
				SetTenancyLogic(tenancy).
				Build()
			Expect(err).ToNot(HaveOccurred())
			Expect(server).ToNot(BeNil())
		})

		It("Fails if logger is not set", func() {
			server, err := NewVirtualNetworksServer().
				SetAttributionLogic(attribution).
				SetTenancyLogic(tenancy).
				Build()
			Expect(err).To(MatchError("logger is mandatory"))
			Expect(server).To(BeNil())
		})

		It("Fails if tenancy logic is not set", func() {
			server, err := NewVirtualNetworksServer().
				SetLogger(logger).
				SetAttributionLogic(attribution).
				Build()
			Expect(err).To(MatchError("tenancy logic is mandatory"))
			Expect(server).To(BeNil())
		})
	})

	Describe("Behaviour", func() {
		var virtualNetworksServer *VirtualNetworksServer

		BeforeEach(func() {
			var err error

			// Create the server:
			virtualNetworksServer, err = NewVirtualNetworksServer().
				SetLogger(logger).
				SetAttributionLogic(attribution).
				SetTenancyLogic(tenancy).
				Build()
			Expect(err).ToNot(HaveOccurred())
		})

		// Helper function to create a NetworkClass in READY state using DAO
		createNetworkClass := func(ctx context.Context, implementationStrategy string) *privatev1.NetworkClass {
			// Create NetworkClass DAO
			networkClassDao, err := dao.NewGenericDAO[*privatev1.NetworkClass]().
				SetLogger(logger).
				SetTable("network_classes").
				SetAttributionLogic(attribution).
				SetTenancyLogic(tenancy).
				Build()
			Expect(err).ToNot(HaveOccurred())

			// Create NetworkClass in READY state
			networkClass := privatev1.NetworkClass_builder{
				ImplementationStrategy: implementationStrategy,
				Capabilities: privatev1.NetworkClassCapabilities_builder{
					SupportsIpv4:      true,
					SupportsIpv6:      true,
					SupportsDualStack: true,
				}.Build(),
				Status: privatev1.NetworkClassStatus_builder{
					State: privatev1.NetworkClassState_NETWORK_CLASS_STATE_READY,
				}.Build(),
			}.Build()

			response, err := networkClassDao.Create().SetObject(networkClass).Do(ctx)
			Expect(err).ToNot(HaveOccurred())
			return response.GetObject()
		}

		// Helper function to create a VirtualNetwork for reuse in tests
		createVirtualNetwork := func(ctx context.Context, server *VirtualNetworksServer, networkClass string) *ffv1.VirtualNetwork {
			response, err := server.Create(ctx, ffv1.VirtualNetworksCreateRequest_builder{
				Object: ffv1.VirtualNetwork_builder{
					Spec: ffv1.VirtualNetworkSpec_builder{
						Region:       "us-east-1",
						NetworkClass: networkClass,
						Ipv4Cidr:     proto.String("10.0.0.0/16"),
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			return response.GetObject()
		}

		It("Creates object", func() {
			// Create NetworkClass first (required by validation)
			nc := createNetworkClass(ctx, "udn-net")

			// Create VirtualNetwork
			response, err := virtualNetworksServer.Create(ctx, ffv1.VirtualNetworksCreateRequest_builder{
				Object: ffv1.VirtualNetwork_builder{
					Spec: ffv1.VirtualNetworkSpec_builder{
						Region:       "us-east-1",
						NetworkClass: nc.GetImplementationStrategy(),
						Ipv4Cidr:     proto.String("10.0.0.0/16"),
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			object := response.GetObject()
			Expect(object).ToNot(BeNil())
			Expect(object.GetId()).ToNot(BeEmpty())
			Expect(object.GetSpec().GetIpv4Cidr()).To(Equal("10.0.0.0/16"))
			Expect(object.GetSpec().GetRegion()).To(Equal("us-east-1"))
			Expect(object.GetSpec().GetNetworkClass()).To(Equal(nc.GetImplementationStrategy()))
		})

		It("Fails to create with invalid NetworkClass reference", func() {
			// Attempt to create VirtualNetwork with non-existent NetworkClass
			_, err := virtualNetworksServer.Create(ctx, ffv1.VirtualNetworksCreateRequest_builder{
				Object: ffv1.VirtualNetwork_builder{
					Spec: ffv1.VirtualNetworkSpec_builder{
						Region:       "us-east-1",
						NetworkClass: "non-existent-class",
						Ipv4Cidr:     proto.String("10.0.0.0/16"),
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("does not exist"))
		})

		It("Get object", func() {
			// Create NetworkClass and VirtualNetwork
			nc := createNetworkClass(ctx, "udn-net")
			createdVN := createVirtualNetwork(ctx, virtualNetworksServer, nc.GetImplementationStrategy())

			// Get the object
			getResponse, err := virtualNetworksServer.Get(ctx, ffv1.VirtualNetworksGetRequest_builder{
				Id: createdVN.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(proto.Equal(createdVN, getResponse.GetObject())).To(BeTrue())
		})

		It("Fails to get non-existent object", func() {
			// Attempt to get non-existent VirtualNetwork
			_, err := virtualNetworksServer.Get(ctx, ffv1.VirtualNetworksGetRequest_builder{
				Id: "non-existent-id",
			}.Build())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("List objects", func() {
			// Create NetworkClass
			nc := createNetworkClass(ctx, "udn-net")

			// Create multiple VirtualNetworks
			const count = 10
			for range count {
				createVirtualNetwork(ctx, virtualNetworksServer, nc.GetImplementationStrategy())
			}

			// List all objects
			response, err := virtualNetworksServer.List(ctx, ffv1.VirtualNetworksListRequest_builder{}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response).ToNot(BeNil())
			items := response.GetItems()
			Expect(items).To(HaveLen(count))
		})

		It("List objects with limit", func() {
			// Create NetworkClass
			nc := createNetworkClass(ctx, "udn-net")

			// Create multiple VirtualNetworks
			const count = 10
			for range count {
				createVirtualNetwork(ctx, virtualNetworksServer, nc.GetImplementationStrategy())
			}

			// List with limit
			response, err := virtualNetworksServer.List(ctx, ffv1.VirtualNetworksListRequest_builder{
				Limit: proto.Int32(5),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response.GetSize()).To(BeNumerically("==", 5))
		})

		It("List objects with offset", func() {
			// Create NetworkClass
			nc := createNetworkClass(ctx, "udn-net")

			// Create multiple VirtualNetworks
			const count = 10
			for range count {
				createVirtualNetwork(ctx, virtualNetworksServer, nc.GetImplementationStrategy())
			}

			// List with offset
			response, err := virtualNetworksServer.List(ctx, ffv1.VirtualNetworksListRequest_builder{
				Offset: proto.Int32(3),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response.GetSize()).To(BeNumerically("==", count-3))
		})

		It("List objects with filter", func() {
			// Create NetworkClass
			nc := createNetworkClass(ctx, "udn-net")

			// Create multiple VirtualNetworks
			const count = 10
			var objects []*ffv1.VirtualNetwork
			for range count {
				vn := createVirtualNetwork(ctx, virtualNetworksServer, nc.GetImplementationStrategy())
				objects = append(objects, vn)
			}

			// List with filter for each specific object
			for _, object := range objects {
				response, err := virtualNetworksServer.List(ctx, ffv1.VirtualNetworksListRequest_builder{
					Filter: proto.String(fmt.Sprintf("this.id == '%s'", object.GetId())),
				}.Build())
				Expect(err).ToNot(HaveOccurred())
				Expect(response.GetSize()).To(BeNumerically("==", 1))
				Expect(response.GetItems()[0].GetId()).To(Equal(object.GetId()))
			}
		})

		It("List objects with ordering", func() {
			// Create NetworkClass
			nc := createNetworkClass(ctx, "udn-net")

			// Create multiple VirtualNetworks
			const count = 5
			for range count {
				createVirtualNetwork(ctx, virtualNetworksServer, nc.GetImplementationStrategy())
			}

			// List with ordering by creation timestamp descending
			response, err := virtualNetworksServer.List(ctx, ffv1.VirtualNetworksListRequest_builder{
				Order: proto.String("this.metadata.creation_timestamp DESC"),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(response.GetSize()).To(BeNumerically("==", count))

			// Verify ordering (newest first)
			items := response.GetItems()
			for i := 0; i < len(items)-1; i++ {
				t1 := items[i].GetMetadata().GetCreationTimestamp().AsTime()
				t2 := items[i+1].GetMetadata().GetCreationTimestamp().AsTime()
				Expect(t1.After(t2) || t1.Equal(t2)).To(BeTrue())
			}
		})

		It("Update object", func() {
			// Create NetworkClass and VirtualNetwork
			nc := createNetworkClass(ctx, "udn-net")
			createdVN := createVirtualNetwork(ctx, virtualNetworksServer, nc.GetImplementationStrategy())

			// Update the object's metadata.name (provide complete object with FieldMask)
			updateResponse, err := virtualNetworksServer.Update(ctx, ffv1.VirtualNetworksUpdateRequest_builder{
				Object: ffv1.VirtualNetwork_builder{
					Id: createdVN.GetId(),
					Metadata: sharedv1.Metadata_builder{
						Name: "updated-name",
					}.Build(),
					Spec: createdVN.GetSpec(), // Keep spec unchanged
				}.Build(),
				UpdateMask: &fieldmaskpb.FieldMask{
					Paths: []string{"metadata.name"},
				},
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(updateResponse.GetObject().GetMetadata().GetName()).To(Equal("updated-name"))

			// Verify spec fields not changed
			Expect(updateResponse.GetObject().GetSpec().GetRegion()).To(Equal(createdVN.GetSpec().GetRegion()))
			Expect(updateResponse.GetObject().GetSpec().GetIpv4Cidr()).To(Equal(createdVN.GetSpec().GetIpv4Cidr()))

			// Get and verify
			getResponse, err := virtualNetworksServer.Get(ctx, ffv1.VirtualNetworksGetRequest_builder{
				Id: createdVN.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			Expect(getResponse.GetObject().GetMetadata().GetName()).To(Equal("updated-name"))
		})

		It("Fails to update non-existent object", func() {
			// Attempt to update non-existent VirtualNetwork
			_, err := virtualNetworksServer.Update(ctx, ffv1.VirtualNetworksUpdateRequest_builder{
				Object: ffv1.VirtualNetwork_builder{
					Id: "non-existent-id",
					Metadata: sharedv1.Metadata_builder{
						Name: "some-name",
					}.Build(),
				}.Build(),
			}.Build())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})

		It("Delete object", func() {
			// Create NetworkClass and VirtualNetwork
			nc := createNetworkClass(ctx, "udn-net")
			createdVN := createVirtualNetwork(ctx, virtualNetworksServer, nc.GetImplementationStrategy())

			// Add a finalizer to prevent immediate archival (allows verification of deletion_timestamp)
			_, err := tx.Exec(
				ctx,
				`update virtual_networks set finalizers = '{"test-finalizer"}' where id = $1`,
				createdVN.GetId(),
			)
			Expect(err).ToNot(HaveOccurred())

			// Delete the object
			_, err = virtualNetworksServer.Delete(ctx, ffv1.VirtualNetworksDeleteRequest_builder{
				Id: createdVN.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())

			// Get and verify deletion_timestamp is set (soft delete)
			getResponse, err := virtualNetworksServer.Get(ctx, ffv1.VirtualNetworksGetRequest_builder{
				Id: createdVN.GetId(),
			}.Build())
			Expect(err).ToNot(HaveOccurred())
			object := getResponse.GetObject()
			Expect(object.GetMetadata().GetDeletionTimestamp()).ToNot(BeNil())
		})

		It("Fails to delete non-existent object", func() {
			// Attempt to delete non-existent VirtualNetwork
			_, err := virtualNetworksServer.Delete(ctx, ffv1.VirtualNetworksDeleteRequest_builder{
				Id: "non-existent-id",
			}.Build())
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("not found"))
		})
	})
})
