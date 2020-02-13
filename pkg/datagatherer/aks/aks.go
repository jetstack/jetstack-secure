// Package aks provides a datagatherer for AKS.
package aks

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"

	aks "github.com/Azure/aks-engine/pkg/api/agentPoolOnlyApi/v20180331"
)

// Config is the configuration for an AKS DataGatherer.
type Config struct {
	// ClusterID is the ID of the cluster in AKS.
	ClusterID string
	// ResourceGroup is the resource group the cluster belongs to.
	ResourceGroup string
	// CredentialsPath is the path to the json file containing the credentials to access Azure APIs.
	CredentialsPath string
}

// Validate checks if a Config is valid.
func (c *Config) Validate() error {
	errs := []string{}

	msg := "%s should be a non empty string."
	if c.ClusterID == "" {
		errs = append(errs, fmt.Sprintf(msg, "ClusterID"))
	}
	if c.ResourceGroup == "" {
		errs = append(errs, fmt.Sprintf(msg, "ResourceGroup"))
	}
	if c.CredentialsPath == "" {
		errs = append(errs, fmt.Sprintf(msg, "CredentialsPath"))
	}

	if len(errs) == 0 {
		return nil
	}
	return fmt.Errorf("invalid configuration: %s", strings.Join(errs, ";"))
}

// NewDataGatherer creates a new AKS DataGatherer. It performs a config validation.
func NewDataGatherer(cfg *Config) (*DataGatherer, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	credentials, err := readCredentials(cfg.CredentialsPath)
	if err != nil {
		return nil, err
	}

	return &DataGatherer{
		resourceGroup: cfg.ResourceGroup,
		clusterID:     cfg.ClusterID,
		credentials:   credentials,
	}, nil
}

// AzureCredentials contains credentials needed to authenticate against Azure APIs.
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

// DataGatherer is a data-gatherer for AKS.
type DataGatherer struct {
	resourceGroup string
	clusterID     string
	credentials   *AzureCredentials
}

// Info contains the data retrieved from AKS.
type Info struct {
	// Cluster represents an AKS cluster.
	Cluster *aks.ManagedCluster
}

// Fetch retrieves cluster information from AKS.
func (g *DataGatherer) Fetch() (interface{}, error) {
	client := &http.Client{}

	req, err := http.NewRequest("GET", fmt.Sprintf("https://management.azure.com/subscriptions/%s/resourceGroups/%s/providers/Microsoft.ContainerService/managedClusters/%s?api-version=2019-08-01", g.credentials.Subscription, g.resourceGroup, g.clusterID), nil)
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

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var cluster aks.ManagedCluster
	err = json.Unmarshal(body, &cluster)
	if err != nil {
		return nil, err
	}

	return &Info{
		Cluster: &cluster,
	}, nil
}
