/*
Copyright (c) 2022 The Regents of the University of California.
*/
package root

import (
	"context"
	"net/http"
	"os"
	"time"

	"github.com/go-logr/logr"
	"github.com/spf13/cobra"
	"github.com/srerickson/ocfl-index/gen/ocfl/v0/ocflv0connect"
)

const (
	envRemote     = "OCFL_INDEX"
	defaultRemote = "http://localhost:8080"
)

// RootCmd is the type of the root command
type Cmd struct {
	cobra.Command
	Log        logr.Logger
	RemoteURL  string
	httpClient *http.Client
	rpcClient  ocflv0connect.IndexServiceClient
}

type OxCmd interface {
	NewCommand(*Cmd) *cobra.Command
	ParseArgs([]string) error
	Run(context.Context, []string) error
}

func (ox *Cmd) Init() {
	ox.RemoteURL = getenvDefault(envRemote, defaultRemote)
}

func (ox *Cmd) AddSub(subs ...OxCmd) {
	for _, sub := range subs {
		sub := sub
		cmd := sub.NewCommand(ox)
		if cmd.RunE == nil {
			cmd.RunE = func(c *cobra.Command, args []string) error {
				if err := sub.ParseArgs(args); err != nil {
					return err
				}
				return sub.Run(c.Context(), args)
			}
		}
		ox.AddCommand(cmd)
	}
}

func (ox *Cmd) HTTPClient() *http.Client {
	if ox.httpClient == nil {
		ox.httpClient = &http.Client{
			Timeout: 20 * time.Second,
		}
	}
	return ox.httpClient
}
func (ox *Cmd) ServiceClient() ocflv0connect.IndexServiceClient {
	if ox.rpcClient == nil {
		ox.rpcClient = ocflv0connect.NewIndexServiceClient(ox.HTTPClient(), ox.RemoteURL)
	}
	return ox.rpcClient
}

func getenvDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
