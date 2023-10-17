package grpc

import (
	"context"

	"github.com/linyows/dewy/registry"
	dewypb "github.com/linyows/dewy/registry/grpc/proto/gen/dewy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

const (
	Scheme = "grpc"
)

type Client struct {
	cl dewypb.RegistryServiceClient
}

var _ registry.Registry = (*Client)(nil)

// New returns Client.
func New(c Config) (*Client, error) {
	opts := []grpc.DialOption{}
	if c.NoTLS {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	cc, err := grpc.Dial(c.Target, opts...)
	if err != nil {
		return nil, err
	}

	cl := dewypb.NewRegistryServiceClient(cc)
	return &Client{
		cl: cl,
	}, nil
}

// Current returns current artifact.
func (c *Client) Current(ctx context.Context, req *registry.CurrentRequest) (*registry.CurrentResponse, error) {
	var an *string
	if req.ArtifactName != "" {
		an = &req.ArtifactName
	}
	creq := &dewypb.CurrentRequest{
		Arch:        req.Arch,
		Os:          req.OS,
		ArifactName: an,
	}
	cres, err := c.cl.Current(ctx, creq)
	if err != nil {
		return nil, err
	}
	res := &registry.CurrentResponse{
		ID:          cres.Id,
		Tag:         cres.Tag,
		ArtifactURL: cres.ArtifactUrl,
	}
	return res, nil
}

// Report report shipping.
func (c *Client) Report(ctx context.Context, req *registry.ReportRequest) error {
	var perr *string
	if req.Err != nil {
		serr := req.Err.Error()
		perr = &serr
	}
	creq := &dewypb.ReportRequest{
		Id:  req.ID,
		Tag: req.Tag,
		Err: perr,
	}
	if _, err := c.cl.Report(ctx, creq); err != nil {
		return err
	}
	return nil
}
