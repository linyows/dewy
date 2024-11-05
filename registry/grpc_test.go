package registry

import (
	"context"
	"errors"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/k1LoW/grpcstub"
	pb "github.com/linyows/dewy/registry/gen/dewy"
	emptypb "google.golang.org/protobuf/types/known/emptypb"
)

func TestCurrent(t *testing.T) {
	ctx := context.Background()
	ts := grpcstub.NewServer(t, "dewy.proto")
	t.Cleanup(func() {
		ts.Close()
	})
	ts.Method("Current").Response(&pb.CurrentResponse{
		Id:          "1234567890",
		Tag:         "v1.0.0",
		ArtifactUrl: "ghr://linyows/dewy",
	})
	g := &GRPC{NoTLS: true}
	if err := g.Dial(ts.Addr()); err != nil {
		t.Fatal(err)
	}
	req := &CurrentRequest{
		Arch: "amd64",
		OS:   "linux",
	}

	t.Run("Response", func(t *testing.T) {
		got, err := g.Current(ctx, req)
		if err != nil {
			t.Fatal(err)
		}
		want := &CurrentResponse{
			ID:          "1234567890",
			Tag:         "v1.0.0",
			ArtifactURL: "ghr://linyows/dewy",
		}
		if diff := cmp.Diff(got, want); diff != "" {
			t.Error(diff)
		}
	})

	t.Run("Request", func(t *testing.T) {
		if want := 1; len(ts.Requests()) != want {
			t.Errorf("got %v, want %v", len(ts.Requests()), want)
		}
		want := grpcstub.Message{
			"arch": "amd64",
			"os":   "linux",
		}
		if diff := cmp.Diff(ts.Requests()[0].Message, want); diff != "" {
			t.Error(diff)
		}
	})
}

func TestReport(t *testing.T) {
	ctx := context.Background()
	ts := grpcstub.NewServer(t, "dewy.proto")
	t.Cleanup(func() {
		ts.Close()
	})
	ts.Method("Report").Response(&emptypb.Empty{})
	g := &GRPC{NoTLS: true}
	if err := g.Dial(ts.Addr()); err != nil {
		t.Fatal(err)
	}
	req := &ReportRequest{
		ID:  "1234567890",
		Tag: "v1.0.0",
		Err: errors.New("something error"),
	}
	if err := g.Report(ctx, req); err != nil {
		t.Fatal(err)
	}

	t.Run("Request", func(t *testing.T) {
		if want := 1; len(ts.Requests()) != want {
			t.Errorf("got %v, want %v", len(ts.Requests()), want)
		}
		want := grpcstub.Message{
			"id":  "1234567890",
			"tag": "v1.0.0",
			"err": "something error",
		}
		if diff := cmp.Diff(ts.Requests()[0].Message, want); diff != "" {
			t.Error(diff)
		}
	})
}
