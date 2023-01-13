/*
Copyright © 2023 NAME HERE <EMAIL ADDRESS>
*/
package cmd

import (
	"context"
	"fmt"
	"log"
	"os"

	"github.com/bufbuild/connect-go"
	"github.com/spf13/cobra"
	ocflv0 "github.com/srerickson/ocfl-index/gen/ocfl/v0"
)

func init() {
	var catCmd = &cobra.Command{
		Use:   "cat",
		Short: "print file content from an OCFL object",
		Long:  `print file content from an OCFL object`,
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			id := args[0]
			p := args[1]
			v := root.commonFlags.vnum
			if err := root.Cat(cmd.Context(), id, v, p); err != nil {
				log.Fatal(err)
			}
		},
	}
	catCmd.Flags().StringVar(&root.commonFlags.vnum, "version", "", "object version")
	root.cmd.AddCommand(catCmd)
}

func (ox *oxCmd) Cat(ctx context.Context, id string, v string, p string) error {
	client := ox.serviceClient()
	req := connect.NewRequest(&ocflv0.GetObjectStateRequest{
		ObjectId: id,
		Version:  v,
		BasePath: p,
		PageSize: 1,
	})
	resp, err := client.GetObjectState(ctx, req)
	if err != nil {
		return fmt.Errorf("during state request: %w", err)
	}
	if resp.Msg.Isdir {
		return fmt.Errorf("path is directory, not a file: '%s'", p)
	}
	conReq := connect.NewRequest(&ocflv0.GetContentRequest{
		Digest: resp.Msg.Digest,
	})
	conResp, err := client.GetContent(ctx, conReq)
	if err != nil {
		return fmt.Errorf("during content request: %w", err)
	}
	for conResp.Receive() {
		if _, err := os.Stdout.Write(conResp.Msg().Data); err != nil {
			return fmt.Errorf("while printint content: %w", err)
		}
	}
	if err := conResp.Err(); err != nil {
		return fmt.Errorf("during content request: %w", err)
	}
	return nil
}
