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
	"errors"
	"log/slog"

	privatev1 "github.com/osac-project/fulfillment-service/internal/api/private/v1"
	"github.com/osac-project/fulfillment-service/internal/auth"
	"github.com/osac-project/fulfillment-service/internal/database"
	"github.com/osac-project/fulfillment-service/internal/database/dao"
)

type PrivateSubnetsServerBuilder struct {
	logger           *slog.Logger
	notifier         *database.Notifier
	attributionLogic auth.AttributionLogic
	tenancyLogic     auth.TenancyLogic
}

var _ privatev1.SubnetsServer = (*PrivateSubnetsServer)(nil)

type PrivateSubnetsServer struct {
	privatev1.UnimplementedSubnetsServer

	logger            *slog.Logger
	generic           *GenericServer[*privatev1.Subnet]
	virtualNetworkDao *dao.GenericDAO[*privatev1.VirtualNetwork]
}

func NewPrivateSubnetsServer() *PrivateSubnetsServerBuilder {
	return &PrivateSubnetsServerBuilder{}
}

func (b *PrivateSubnetsServerBuilder) SetLogger(value *slog.Logger) *PrivateSubnetsServerBuilder {
	b.logger = value
	return b
}

func (b *PrivateSubnetsServerBuilder) SetNotifier(value *database.Notifier) *PrivateSubnetsServerBuilder {
	b.notifier = value
	return b
}

func (b *PrivateSubnetsServerBuilder) SetAttributionLogic(value auth.AttributionLogic) *PrivateSubnetsServerBuilder {
	b.attributionLogic = value
	return b
}

func (b *PrivateSubnetsServerBuilder) SetTenancyLogic(value auth.TenancyLogic) *PrivateSubnetsServerBuilder {
	b.tenancyLogic = value
	return b
}

func (b *PrivateSubnetsServerBuilder) Build() (result *PrivateSubnetsServer, err error) {
	// Check parameters:
	if b.logger == nil {
		err = errors.New("logger is mandatory")
		return
	}
	if b.tenancyLogic == nil {
		err = errors.New("tenancy logic is mandatory")
		return
	}

	// Create the VirtualNetwork DAO for parent validation:
	virtualNetworkDao, err := dao.NewGenericDAO[*privatev1.VirtualNetwork]().
		SetLogger(b.logger).
		SetTable("virtual_networks").
		SetAttributionLogic(b.attributionLogic).
		SetTenancyLogic(b.tenancyLogic).
		Build()
	if err != nil {
		return
	}

	// Create the generic server:
	generic, err := NewGenericServer[*privatev1.Subnet]().
		SetLogger(b.logger).
		SetService(privatev1.Subnets_ServiceDesc.ServiceName).
		SetTable("subnets").
		SetNotifier(b.notifier).
		SetAttributionLogic(b.attributionLogic).
		SetTenancyLogic(b.tenancyLogic).
		Build()
	if err != nil {
		return
	}

	// Create and populate the object:
	result = &PrivateSubnetsServer{
		logger:            b.logger,
		generic:           generic,
		virtualNetworkDao: virtualNetworkDao,
	}
	return
}

func (s *PrivateSubnetsServer) List(ctx context.Context,
	request *privatev1.SubnetsListRequest) (response *privatev1.SubnetsListResponse, err error) {
	err = s.generic.List(ctx, request, &response)
	return
}

func (s *PrivateSubnetsServer) Get(ctx context.Context,
	request *privatev1.SubnetsGetRequest) (response *privatev1.SubnetsGetResponse, err error) {
	err = s.generic.Get(ctx, request, &response)
	return
}

func (s *PrivateSubnetsServer) Create(ctx context.Context,
	request *privatev1.SubnetsCreateRequest) (response *privatev1.SubnetsCreateResponse, err error) {
	err = s.generic.Create(ctx, request, &response)
	return
}

func (s *PrivateSubnetsServer) Update(ctx context.Context,
	request *privatev1.SubnetsUpdateRequest) (response *privatev1.SubnetsUpdateResponse, err error) {
	err = s.generic.Update(ctx, request, &response)
	return
}

func (s *PrivateSubnetsServer) Delete(ctx context.Context,
	request *privatev1.SubnetsDeleteRequest) (response *privatev1.SubnetsDeleteResponse, err error) {
	err = s.generic.Delete(ctx, request, &response)
	return
}

func (s *PrivateSubnetsServer) Signal(ctx context.Context,
	request *privatev1.SubnetsSignalRequest) (response *privatev1.SubnetsSignalResponse, err error) {
	err = s.generic.Signal(ctx, request, &response)
	return
}
