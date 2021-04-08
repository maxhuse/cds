package plugin_artifactory_download_artifact

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"

	"github.com/ovh/cds/sdk"
	"github.com/ovh/cds/sdk/grpcplugin/integrationplugin"
)

/*
This plugin have to be used as a download artifact integration plugin

Artifactory download artifact plugin must configured as following:
	name: artifactory-download-artifact-plugin
	type: integration
	author: "Steven Guiheux"
	description: "OVH Artifactory Upload Artifact Plugin"

$ cdsctl admin plugins import artifactory-download-artifact-plugin.yml

Build the present binaries and import in CDS:
	os: linux
	arch: amd64
	cmd: <path-to-binary-file>

$ cdsctl admin plugins binary-add artifactory-download-artifact-plugin artifactory-download-artifact-plugin-bin.yml <path-to-binary-file>

Artifactory integration must configured as following
	name: Artifactory
	default_config:
	  artifactory_url:
		type: string
	  artifactory_token_name:
		type: string
	  artifactory_token:
		type: password
	  artifactory_suffix_snapshot:
		type: string
	  artifactory_suffix_release:
		type: string
	artifact_manager: true
*/

type artifactoryDownloadArtifactPlugin struct {
	integrationplugin.Common
}

func (e *artifactoryDownloadArtifactPlugin) Manifest(_ context.Context, _ *empty.Empty) (*integrationplugin.IntegrationPluginManifest, error) {
	return &integrationplugin.IntegrationPluginManifest{
		Name:        "OVH Artifactory Download Artifact Plugin",
		Author:      "Steven Guiheux",
		Description: "OVH Artifactory Download Artifact Plugin",
		Version:     sdk.VERSION,
	}, nil
}

func (e *artifactoryDownloadArtifactPlugin) Run(_ context.Context, _ *integrationplugin.RunQuery) (*integrationplugin.RunResult, error) {
	return &integrationplugin.RunResult{
		Status: sdk.StatusSuccess,
	}, nil
}

func main() {
	e := artifactoryDownloadArtifactPlugin{}
	if err := integrationplugin.Start(context.Background(), &e); err != nil {
		panic(err)
	}
	return

}

func fail(format string, args ...interface{}) (*integrationplugin.RunResult, error) {
	msg := fmt.Sprintf(format, args...)
	fmt.Println(msg)
	return &integrationplugin.RunResult{
		Details: msg,
		Status:  sdk.StatusFail,
	}, nil
}
