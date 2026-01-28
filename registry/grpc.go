package registry

import (
	"context"
	"net/url"
	"time"

	pb "github.com/linyows/dewy/registry/gen/dewy"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type GRPC struct {
	Target   string `schema:"-"`
	NoTLS    bool   `schema:"no-tls"`
	Artifact string `schema:"artifact"`
	cl       pb.RegistryServiceClient
}

func NewGRPC(ctx context.Context, u string) (*GRPC, error) {
	ur, err := url.Parse(u)
	if err != nil {
		return nil, err
	}

	var gr GRPC
	if err := decoder.Decode(&gr, ur.Query()); err != nil {
		return nil, err
	}

	if err := gr.Dial(ctx, ur.Host); err != nil {
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
func (c *GRPC) Current(ctx context.Context) (*CurrentResponse, error) {
	var an *string
	if c.Artifact != "" {
		an = &c.Artifact
	}
	creq := &pb.CurrentRequest{
		Arch:        getArch(),
		Os:          getOS(),
		ArifactName: an,
	}
	cres, err := c.cl.Current(ctx, creq)
	if err != nil {
		return nil, err
	}
	var createdAt *time.Time
	if cres.CreatedAt != nil {
		t := cres.CreatedAt.AsTime()
		createdAt = &t
	}

	// Extract slot from build metadata
	var slot string
	if sv := ParseSemVer(cres.Tag); sv != nil {
		slot = sv.BuildMetadata
	}

	res := &CurrentResponse{
		ID:          cres.Id,
		Tag:         cres.Tag,
		ArtifactURL: cres.ArtifactUrl,
		CreatedAt:   createdAt,
		Slot:        slot,
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
		Id:      req.ID,
		Tag:     req.Tag,
		Command: req.Command,
		Err:     perr,
	}
	if _, err := c.cl.Report(ctx, creq); err != nil {
		return err
	}
	return nil
}
