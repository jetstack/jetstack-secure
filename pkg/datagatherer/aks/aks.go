// Package aks provides a datagatherer for AKS.
package aks

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"

	aks "github.com/Azure/aks-engine/pkg/api/agentPoolOnlyApi/v20180331"
)

// AKSDataGatherer is a DataGatherer for AKS.
type AKSDataGatherer struct {
	ctx           context.Context
	resourceGroup string
	clusterName   string
	credentials   *AzureCredentials
	dataPath      string
}

// AKSInfo contains the data retrieved from AKS.
type AKSInfo struct {
	Cluster *aks.ManagedCluster
}

type AzureCredentials struct {
	AccessToken  string `json:"accessToken"`
	ExpiresOn    string `json:"expiresOn"`
	Subscription string `json:"subscription"`
	Tenant       string `json:"tenant"`
	TokenType    string `json:"tokenType"`
}

func readCredentials(path string) (*AzureCredentials, error) {
	b, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var creds AzureCredentials
	err = json.Unmarshal(b, &creds)
	if err != nil {
		return nil, err
	}

	if len(creds.Subscription) == 0 {
		return nil, fmt.Errorf("'subscription' must not be empty")
	}
	if creds.TokenType != "Bearer" {
		return nil, fmt.Errorf("'tokenType' %s is not supported", creds.TokenType)
	}

	return &creds, nil
}

// NewAKSDataGatherer creates a new AKSDataGatherer for a cluster.
func NewAKSDataGatherer(ctx context.Context, resourceGroup, clusterName, credentialsPath, dataPath string) (*AKSDataGatherer, error) {
	var credentials *AzureCredentials
	var err error
	if dataPath == "" {
		credentials, err = readCredentials(credentialsPath)
		if err != nil {
			return nil, err
		}
	}

	return &AKSDataGatherer{
		ctx:           ctx,
		resourceGroup: resourceGroup,
		clusterName:   clusterName,
		credentials:   credentials,
		dataPath:      dataPath,
	}, nil
}

// Fetch retrieves cluster information from AKS.
func (g *AKSDataGatherer) Fetch() (interface{}, error) {
	var dataBytes []byte
	var err error

	if g.dataPath != "" {
		dataBytes, err = ioutil.ReadFile(g.dataPath)
		if err != nil {
			return nil, err
		}
	} else {
		client := &http.Client{}

		req, err := http.NewRequest("GET", fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s?api-version=2019-08-01", g.credentials.Subscription, g.resourceGroup, g.clusterName), nil)
		if err != nil {
			return nil, err
		}
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", g.credentials.AccessToken))

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			errorBody, _ := ioutil.ReadAll(resp.Body)
			return nil, fmt.Errorf("error retrieving cluster information (status code %d): %v", resp.StatusCode, string(errorBody))
		}

		dataBytes, err = ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
	}

	var cluster aks.ManagedCluster
	err = json.Unmarshal(dataBytes, &cluster)
	if err != nil {
		return nil, err
	}

	return &AKSInfo{
		Cluster: &cluster,
	}, nil
}
