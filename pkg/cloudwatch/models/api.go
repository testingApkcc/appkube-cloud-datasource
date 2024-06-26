package models

import (
	resources2 "github.com/appkube/cloud-datasource/pkg/cloudwatch/models/resources"
	"net/url"

	"github.com/aws/aws-sdk-go/service/cloudwatch"
	"github.com/aws/aws-sdk-go/service/cloudwatchlogs"
	"github.com/aws/aws-sdk-go/service/oam"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
)

type RequestContextFactoryFunc func(pluginCtx backend.PluginContext, region string) (reqCtx RequestContext, err error)

type RouteHandlerFunc func(pluginCtx backend.PluginContext, reqContextFactory RequestContextFactoryFunc, parameters url.Values) ([]byte, *HttpError)

type RequestContext struct {
	MetricsClientProvider MetricsClientProvider
	LogsAPIProvider       CloudWatchLogsAPIProvider
	OAMClientProvider     OAMClientProvider
	Settings              CloudWatchSettings
	//Features              featuremgmt.FeatureToggles
}

type ListMetricsProvider interface {
	GetDimensionKeysByDimensionFilter(resources2.DimensionKeysRequest) ([]resources2.ResourceResponse[string], error)
	GetDimensionValuesByDimensionFilter(resources2.DimensionValuesRequest) ([]resources2.ResourceResponse[string], error)
	GetMetricsByNamespace(r resources2.MetricsRequest) ([]resources2.ResourceResponse[resources2.Metric], error)
}

type MetricsClientProvider interface {
	ListMetricsWithPageLimit(params *cloudwatch.ListMetricsInput) ([]resources2.MetricResponse, error)
}

type CloudWatchMetricsAPIProvider interface {
	ListMetricsPages(*cloudwatch.ListMetricsInput, func(*cloudwatch.ListMetricsOutput, bool) bool) error
}

type CloudWatchLogsAPIProvider interface {
	DescribeLogGroups(*cloudwatchlogs.DescribeLogGroupsInput) (*cloudwatchlogs.DescribeLogGroupsOutput, error)
}

type OAMClientProvider interface {
	ListSinks(*oam.ListSinksInput) (*oam.ListSinksOutput, error)
	ListAttachedLinks(*oam.ListAttachedLinksInput) (*oam.ListAttachedLinksOutput, error)
}

type LogGroupsProvider interface {
	GetLogGroups(request resources2.LogGroupsRequest) ([]resources2.ResourceResponse[resources2.LogGroup], error)
}

type AccountsProvider interface {
	GetAccountsForCurrentUserOrRole() ([]resources2.ResourceResponse[resources2.Account], error)
}
