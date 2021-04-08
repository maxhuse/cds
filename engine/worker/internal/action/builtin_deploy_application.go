package action

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"github.com/rockbears/log"

	"github.com/ovh/cds/engine/worker/pkg/workerruntime"
	"github.com/ovh/cds/sdk"
	"github.com/ovh/cds/sdk/grpcplugin/integrationplugin"
)

func RunDeployApplication(ctx context.Context, wk workerruntime.Runtime, _ sdk.Action, _ []sdk.Variable) (sdk.Result, error) {
	jobID, err := workerruntime.JobID(ctx)
	if err != nil {
		return sdk.Result{}, err
	}

	pfName := sdk.ParameterFind(wk.Parameters(), "cds.integration")
	if pfName == nil {
		return sdk.Result{}, errors.New("unable to retrieve deployment integration... Aborting")
	}

	pkey := sdk.ParameterFind(wk.Parameters(), "cds.project")
	pf, err := wk.Client().ProjectIntegrationGet(pkey.Value, pfName.Value, true)
	if err != nil {
		return sdk.Result{}, fmt.Errorf("unable to retrieve deployment integration (%v)... Aborting", err)
	}

	job, err := wk.Client().QueueJobInfo(ctx, jobID)
	if err != nil {
		return sdk.Result{}, err
	}

	//First check OS and Architecture
	binary := job.GetPuginBinary(sdk.GRPCPluginDeploymentIntegration, strings.ToLower(sdk.GOOS), strings.ToLower(sdk.GOARCH))
	if binary == nil {
		return sdk.Result{}, fmt.Errorf("unable to retrieve the plugin for deployment integration %s... Aborting", pf.Model.Name)
	}

	pluginSocket, err := startGRPCPlugin(ctx, binary.PluginName, wk, binary, startGRPCPluginOptions{})
	if err != nil {
		return sdk.Result{}, fmt.Errorf("unable to start GRPCPlugin: %v", err)
	}

	c, err := integrationplugin.Client(context.Background(), pluginSocket.Socket)
	if err != nil {
		return sdk.Result{}, fmt.Errorf("unable to call GRPCPlugin: %v", err)
	}

	pluginSocket.Client = c
	if _, err := c.Manifest(context.Background(), new(empty.Empty)); err != nil {
		return sdk.Result{}, fmt.Errorf("unable to call GRPCPlugin: %v", err)
	}

	pluginClient := pluginSocket.Client
	integrationPluginClient, ok := pluginClient.(integrationplugin.IntegrationPluginClient)
	if !ok {
		return sdk.Result{}, fmt.Errorf("unable to retrieve integration GRPCPlugin: %v", err)
	}

	logCtx, stopLogs := context.WithCancel(ctx)
	done := make(chan struct{})
	go enablePluginLogger(logCtx, done, pluginSocket, wk)

	manifest, err := integrationPluginClient.Manifest(ctx, &empty.Empty{})
	if err != nil {
		integrationPluginClientStop(ctx, integrationPluginClient, done, stopLogs)
		return sdk.Result{}, fmt.Errorf("unable to retrieve retrieve plugin manifest: %v", err)
	}

	wk.SendLog(ctx, workerruntime.LevelInfo, fmt.Sprintf("# Plugin %s v%s is ready", manifest.Name, manifest.Version))

	query := integrationplugin.RunQuery{
		Options: sdk.ParametersToMap(wk.Parameters()),
	}

	res, err := integrationPluginClient.Run(ctx, &query)
	if err != nil {
		integrationPluginClientStop(ctx, integrationPluginClient, done, stopLogs)
		return sdk.Result{}, fmt.Errorf("error deploying application: %v", err)
	}

	wk.SendLog(ctx, workerruntime.LevelInfo, fmt.Sprintf("# Details: %s", res.Details))
	wk.SendLog(ctx, workerruntime.LevelInfo, fmt.Sprintf("# Status: %s", res.Status))

	if strings.ToUpper(res.Status) == strings.ToUpper(sdk.StatusSuccess) {
		integrationPluginClientStop(ctx, integrationPluginClient, done, stopLogs)
		return sdk.Result{
			Status: sdk.StatusSuccess,
		}, nil
	}

	integrationPluginClientStop(ctx, integrationPluginClient, done, stopLogs)

	return sdk.Result{
		Status: sdk.StatusFail,
		Reason: res.Details,
	}, nil
}

func integrationPluginClientStop(ctx context.Context, integrationPluginClient integrationplugin.IntegrationPluginClient, done chan struct{}, stopLogs context.CancelFunc) {
	if _, err := integrationPluginClient.Stop(ctx, new(empty.Empty)); err != nil {
		// Transport is closing is a "normal" error, as we requested plugin to stop
		if !strings.Contains(err.Error(), "transport is closing") {
			log.Error(ctx, "Error on integrationPluginClient.Stop: %s", err)
		}
	}
	stopLogs()
	<-done
}
