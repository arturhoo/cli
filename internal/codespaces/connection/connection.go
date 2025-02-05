package connection

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"

	"github.com/cli/cli/v2/internal/codespaces/api"
	"github.com/microsoft/dev-tunnels/go/tunnels"
)

const (
	clientName = "gh"
)

type CodespaceConnection struct {
	tunnelProperties           api.TunnelProperties
	TunnelManager              *tunnels.Manager
	TunnelClient               *tunnels.Client
	Options                    *tunnels.TunnelRequestOptions
	Tunnel                     *tunnels.Tunnel
	AllowedPortPrivacySettings []string
}

// NewCodespaceConnection initializes a connection to a codespace.
// This connections allows for port forwarding which enables the
// use of most features of the codespace command.
func NewCodespaceConnection(ctx context.Context, codespace *api.Codespace, httpClient *http.Client) (connection *CodespaceConnection, err error) {
	// Get the tunnel properties
	tunnelProperties := codespace.Connection.TunnelProperties

	// Create the tunnel manager
	tunnelManager, err := getTunnelManager(tunnelProperties, httpClient)
	if err != nil {
		return nil, fmt.Errorf("error getting tunnel management client: %w", err)
	}

	// Calculate allowed port privacy settings
	allowedPortPrivacySettings := codespace.RuntimeConstraints.AllowedPortPrivacySettings

	// Get the access tokens
	connectToken := tunnelProperties.ConnectAccessToken
	managementToken := tunnelProperties.ManagePortsAccessToken

	// Create the tunnel definition
	tunnel := &tunnels.Tunnel{
		AccessTokens: map[tunnels.TunnelAccessScope]string{tunnels.TunnelAccessScopeConnect: connectToken, tunnels.TunnelAccessScopeManagePorts: managementToken},
		TunnelID:     tunnelProperties.TunnelId,
		ClusterID:    tunnelProperties.ClusterId,
		Domain:       tunnelProperties.Domain,
	}

	// Create options
	options := &tunnels.TunnelRequestOptions{
		IncludePorts: true,
	}

	// Create the tunnel client (not connected yet)
	tunnelClient, err := getTunnelClient(ctx, tunnelManager, tunnel, options)
	if err != nil {
		return nil, fmt.Errorf("error getting tunnel client: %w", err)
	}

	return &CodespaceConnection{
		tunnelProperties:           tunnelProperties,
		TunnelManager:              tunnelManager,
		TunnelClient:               tunnelClient,
		Options:                    options,
		Tunnel:                     tunnel,
		AllowedPortPrivacySettings: allowedPortPrivacySettings,
	}, nil
}

// getTunnelManager creates a tunnel manager for the given codespace.
// The tunnel manager is used to get the tunnel hosted in the codespace that we
// want to connect to and perform operations on ports (add, remove, list, etc.).
func getTunnelManager(tunnelProperties api.TunnelProperties, httpClient *http.Client) (tunnelManager *tunnels.Manager, err error) {
	userAgent := []tunnels.UserAgent{{Name: clientName}}
	url, err := url.Parse(tunnelProperties.ServiceUri)
	if err != nil {
		return nil, fmt.Errorf("error parsing tunnel service uri: %w", err)
	}

	// Create the tunnel manager
	tunnelManager, err = tunnels.NewManager(userAgent, nil, url, httpClient)
	if err != nil {
		return nil, fmt.Errorf("error creating tunnel manager: %w", err)
	}

	return tunnelManager, nil
}

// getTunnelClient creates a tunnel client for the given tunnel.
// The tunnel client is used to connect to the the tunnel and allows
// for ports to be forwarded locally.
func getTunnelClient(ctx context.Context, tunnelManager *tunnels.Manager, tunnel *tunnels.Tunnel, options *tunnels.TunnelRequestOptions) (tunnelClient *tunnels.Client, err error) {
	// Get the tunnel that we want to connect to
	codespaceTunnel, err := tunnelManager.GetTunnel(ctx, tunnel, options)
	if err != nil {
		return nil, fmt.Errorf("error getting tunnel: %w", err)
	}

	// Copy the access tokens from the tunnel definition
	codespaceTunnel.AccessTokens = tunnel.AccessTokens

	// We need to pass false for accept local connections because we don't want to automatically connect to all forwarded ports
	tunnelClient, err = tunnels.NewClient(log.New(io.Discard, "", log.LstdFlags), codespaceTunnel, false)
	if err != nil {
		return nil, fmt.Errorf("error creating tunnel client: %w", err)
	}

	return tunnelClient, nil
}
