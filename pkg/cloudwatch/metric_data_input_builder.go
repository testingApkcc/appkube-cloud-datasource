package cloudwatch

import (
	models2 "github.com/appkube/cloud-datasource/pkg/cloudwatch/models"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/service/cloudwatch"

	"github.com/appkube/cloud-datasource/pkg/infra/log"
)

func (e *cloudWatchExecutor) buildMetricDataInput(logger log.Logger, startTime time.Time, endTime time.Time,
	queries []*models2.CloudWatchQuery) (*cloudwatch.GetMetricDataInput, error) {
	metricDataInput := &cloudwatch.GetMetricDataInput{
		StartTime: aws.Time(startTime),
		EndTime:   aws.Time(endTime),
		ScanBy:    aws.String("TimestampAscending"),
	}
	flagCloudWatchDynamicLabels := false // will come as parameter later
	//shouldSetLabelOptions := e.features.IsEnabled(featuremgmt.FlagCloudWatchDynamicLabels) && len(queries) > 0 && len(queries[0].TimezoneUTCOffset) > 0
	shouldSetLabelOptions := flagCloudWatchDynamicLabels && len(queries) > 0 && len(queries[0].TimezoneUTCOffset) > 0

	if shouldSetLabelOptions {
		metricDataInput.LabelOptions = &cloudwatch.LabelOptions{
			Timezone: aws.String(queries[0].TimezoneUTCOffset),
		}
	}

	for _, query := range queries {
		metricDataQuery, err := e.buildMetricDataQuery(logger, query)
		if err != nil {
			return nil, &models2.QueryError{Err: err, RefID: query.RefId}
		}
		metricDataInput.MetricDataQueries = append(metricDataInput.MetricDataQueries, metricDataQuery)
	}

	return metricDataInput, nil
}
