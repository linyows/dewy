package registry

import (
	"context"

	pb "github.com/linyows/dewy/registry/gen/dewy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPC struct {
	Target string `schema:"-"`
	NoTLS  bool   `schema:"no-tls"`
	cl     pb.RegistryServiceClient
}

func NewGRPC(ctx context.Context, path string) (*GRPC, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, err
	}

	var gr GRPC
	if err := decoder.Decode(&gr, u.Query()); err != nil {
		return nil, err
	}

	if err := gr.Dial(ctx, u.Host); err != nil {
		return nil, err
	}

	return &gr, nil
}

// Dial returns GRPC.
func (c *GRPC) Dial(ctx context.Context, target string) error {
	c.Target = target
	opts := []grpc.DialOption{}
	if c.NoTLS {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	// cc, err := grpc.NewClient("passthrough://"+c.Target, opts...)
	cc, err := grpc.NewClient(c.Target, opts...)
	if err != nil {
		return err
	}

	c.cl = pb.NewRegistryServiceClient(cc)
	return nil
}

// Current returns current artifact.
func (c *GRPC) Current(ctx context.Context, req *CurrentRequest) (*CurrentResponse, error) {
	var an *string
	if req.ArtifactName != "" {
		an = &req.ArtifactName
	}
	creq := &pb.CurrentRequest{
		Arch:        req.Arch,
		Os:          req.OS,
		ArifactName: an,
	}
	cres, err := c.cl.Current(ctx, creq)
	if err != nil {
		return nil, err
	}
	res := &CurrentResponse{
		ID:          cres.Id,
		Tag:         cres.Tag,
		ArtifactURL: cres.ArtifactUrl,
	}
	return res, nil
}

// Report report shipping.
func (c *GRPC) Report(ctx context.Context, req *ReportRequest) error {
	var perr *string
	if req.Err != nil {
		serr := req.Err.Error()
		perr = &serr
	}
	creq := &pb.ReportRequest{
		Id:  req.ID,
		Tag: req.Tag,
		Err: perr,
	}
	if _, err := c.cl.Report(ctx, creq); err != nil {
		return err
	}
	return nil
}
