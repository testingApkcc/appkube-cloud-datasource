package pluginhost

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/appkube/cloud-datasource/pkg/infra/httpclient"
	"time"

	"github.com/appkube/cloud-datasource/pkg/infinity"
	"github.com/appkube/cloud-datasource/pkg/models"
	"github.com/appkube/cloud-datasource/pkg/tsdb/cloudwatch"
	"github.com/grafana/grafana-plugin-sdk-go/backend"
	"github.com/grafana/grafana-plugin-sdk-go/data"
)

// QueryData handles multiple queries and returns multiple responses.
func (ds *PluginHost) QueryData(ctx context.Context, req *backend.QueryDataRequest) (*backend.QueryDataResponse, error) {
	response := backend.NewQueryDataResponse()
	//client, err := getInstance(ds.im, req.PluginContext) // to be uncomment when local testing done

	infClient, err := infinity.NewClient(models.InfinitySettings{})
	if err != nil {
		backend.Logger.Error("error getting infinity instance", "error", err.Error())
		return response, fmt.Errorf("error getting infinity instance. %w", err)
	}

	for _, q := range req.Queries {
		//modifications for aws cloudwatch
		query, err := models.LoadQueryToIdentifyType(q)
		if err != nil {
			backend.Logger.Error("error un-marshaling the query", "error", err.Error())
			return response, fmt.Errorf("error un-marshaling the query. %w", err)
		}
		cmdbResp, cmdbStatusCode, _, err := getCmdbData(ctx, *infClient, query, req.Headers)
		if err != nil {
			backend.Logger.Error("error in getting cmdb response", "error", err.Error())
			return response, fmt.Errorf("error in getting cmdb response. %w", err)
		}
		vaultResp, vaultStatusCode, _, err := getAwsCredentials(ctx, *infClient, query, req.Headers)
		if err != nil {
			backend.Logger.Error("error in getting aws credentials", "error", err.Error())
			return response, fmt.Errorf("error in getting aws credentials. %w", err)
		}
		if vaultStatusCode/100 != 2 {
			backend.Logger.Error("vault error", "error", vaultStatusCode)
			return response, fmt.Errorf("vault error. %w", vaultStatusCode)
		}

		vaultString, ok := vaultResp.(string)
		if !ok {
			backend.Logger.Error("vault response error", "error")
			return response, fmt.Errorf("vault response error.")
		}
		awsCreds := &models.AwsCredential{}
		err = json.Unmarshal([]byte(vaultString), &awsCreds)
		if err != nil {
			backend.Logger.Error("error un-marshaling the vault response", "error", err.Error())
			return response, fmt.Errorf("error un-marshaling the vault response. %w", err)
		}

		switch query.Type {
		case models.QueryTypeAppKubeCloudWatch:
			var cloudWatchService = cloudwatch.ProvideService(httpclient.NewProvider(), awsCreds)
			res, err := cloudWatchService.Executor.QueryData(ctx, req)
			if err != nil {
				backend.Logger.Error("error executing cloudwatch query", "error", err.Error())
				return response, fmt.Errorf("error executing cloudwatch query. %w", err)
			}
			response = res
		default:
			res := QueryData(cmdbResp, cmdbStatusCode, ctx, q, *infClient, req.Headers, req.PluginContext)
			response.Responses[q.RefID] = res
		}
		//modifications for cloudwatch datasource
		//res := QueryData(ctx, q, *client.client, req.Headers, req.PluginContext)
		//response.Responses[q.RefID] = res
	}
	return response, nil
}

func QueryData(cmdbResp any, cmdbStatusCode int, ctx context.Context, backendQuery backend.DataQuery, infClient infinity.Client, requestHeaders map[string]string, pluginContext backend.PluginContext) (response backend.DataResponse) {
	//region Loading Query
	query, err := models.LoadQuery(ctx, backendQuery, pluginContext)
	if err != nil {
		backend.Logger.Error("error un-marshaling the query", "error", err.Error())
		response.Error = fmt.Errorf("error un-marshaling the query. %w", err)
		return response
	}
	//endregion
	//region Frame Builder
	switch query.Type {
	case models.QueryTypeAppKubeAPI:

		//resp, statusCode, _, err := getCmdbData(ctx, infClient, query, requestHeaders)

		//if err != nil {
		//	//fmt.Println("Error in getting cmdb response: ", err.Error())
		//	backend.Logger.Error("Error in getting cmdb response", "error", err.Error())
		//	response.Error = fmt.Errorf("error in getting cmdb response. %w", err)
		//	return response
		//}
		//fmt.Println("CMDB response: ", resp)
		//if statusCode/100 == 2 {
		if cmdbStatusCode/100 == 2 {

			//defer func() {
			//	_ = resp.Body.Close()
			//}()
			//cmdbData, err := io.ReadAll(resp.Body)
			//if err != nil {
			//	//fmt.Println("Error in reading cmdb response: ", err.Error())
			//	backend.Logger.Error("Error in reading cmdb response", "error", err.Error())
			//	response.Error = fmt.Errorf("error in reading cmdb response. %w", err)
			//	return response
			//}
			//fmt.Println(string(cmdbData))

			query.URL = fmt.Sprintf(query.AwsxUrl + query.AccountId)
			query.Type = "json"
			query.Parser = "backend"
			fmt.Println("Appconfig url :" + query.URL)
			frame, err := infinity.GetFrameForURLSources(ctx, query, infClient, requestHeaders)
			if err != nil {
				response.Frames = append(response.Frames, frame)
				response.Error = fmt.Errorf("error getting data frame from cloud elements. %w", err)
				return response
			}
			if frame != nil {
				response.Frames = append(response.Frames, frame)
			}

		}

	case models.QueryTypeGSheets:
		sheetId := query.Spreadsheet
		sheetName := query.SheetName
		sheetRange := query.SheetRange
		if sheetName != "" {
			sheetRange = sheetName + "!" + sheetRange
		}
		if sheetId == "" {
			response.Error = errors.New("invalid or empty sheet ID")
			return response
		}
		query.URL = fmt.Sprintf("https://sheets.googleapis.com/v4/spreadsheets/%s?includeGridData=true&ranges=%s", sheetId, sheetRange)
		frame, err := infinity.GetFrameForURLSources(ctx, query, infClient, requestHeaders)
		if err != nil {
			response.Frames = append(response.Frames, frame)
			response.Error = fmt.Errorf("error getting data frame from google sheets. %w", err)
			return response
		}
		if frame != nil {
			response.Frames = append(response.Frames, frame)
		}
	default:
		query, _ := infinity.UpdateQueryWithReferenceData(ctx, query, infClient.Settings)
		switch query.Source {
		case "url":
			frame, err := infinity.GetFrameForURLSources(ctx, query, infClient, requestHeaders)
			if err != nil {
				frame, _ = infinity.WrapMetaForRemoteQuery(ctx, frame, err, query)
				response.Frames = append(response.Frames, frame)
				response.Error = fmt.Errorf("error getting data frame. %w", err)
				return response
			}
			if frame != nil && infClient.Settings.AuthenticationMethod != models.AuthenticationMethodNone && infClient.Settings.AuthenticationMethod != "" && len(infClient.Settings.AllowedHosts) < 1 {
				frame.AppendNotices(data.Notice{
					Text: "Datasource is missing allowed hosts/URLs. Configure it in the datasource settings page for enhanced security.",
				})
			}
			if frame != nil {
				frame, _ = infinity.WrapMetaForRemoteQuery(ctx, frame, nil, query)
				response.Frames = append(response.Frames, frame)
			}
		case "inline":
			frame, err := infinity.GetFrameForInlineSources(query)
			if err != nil {
				frame, _ := infinity.WrapMetaForInlineQuery(frame, err, query)
				response.Frames = append(response.Frames, frame)
				response.Error = fmt.Errorf("error getting data frame from inline data. %w", err)
				return response
			}
			if frame != nil {
				frame, _ := infinity.WrapMetaForInlineQuery(frame, nil, query)
				response.Frames = append(response.Frames, frame)
			}
		default:
			frame := infinity.GetDummyFrame(query)
			if frame != nil {
				response.Frames = append(response.Frames, frame)
			}
		}
	}
	//endregion
	return response
}

func getCmdbData(ctx context.Context, infClient infinity.Client, query models.Query, requestHeaders map[string]string) (o any, statusCode int, duration time.Duration, err error) {
	fmt.Println("Getting CMDB data")
	//product := query.Product
	environment := query.Environment
	//module := query.Module
	//serviceType := query.ServiceType
	accountId := query.AccountId
	query.URL = query.CmdbUrl + "?" + fmt.Sprintf("associatedEnv=%s&associatedCommonService=FileRepo&associatedLandingZone=%s", environment, accountId)
	return infClient.GetResults(ctx, query, requestHeaders)

}
func getAwsCredentials(ctx context.Context, infClient infinity.Client, query models.Query, requestHeaders map[string]string) (o any, statusCode int, duration time.Duration, err error) {
	fmt.Println("Getting Aws credentials")
	query.URL = query.VaultUrl + "/" + query.AccountId
	return infClient.GetResults(ctx, query, requestHeaders)
}
