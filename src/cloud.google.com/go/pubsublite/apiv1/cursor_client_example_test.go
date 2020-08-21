// Copyright 2020 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Code generated by protoc-gen-go_gapic. DO NOT EDIT.

package pubsublite_test

import (
	"context"
	"io"

	pubsublite "cloud.google.com/go/pubsublite/apiv1"
	"google.golang.org/api/iterator"
	pubsublitepb "google.golang.org/genproto/googleapis/cloud/pubsublite/v1"
)

func ExampleNewCursorClient() {
	ctx := context.Background()
	c, err := pubsublite.NewCursorClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use client.
	_ = c
}

func ExampleCursorClient_StreamingCommitCursor() {
	// import pubsublitepb "google.golang.org/genproto/googleapis/cloud/pubsublite/v1"

	ctx := context.Background()
	c, err := pubsublite.NewCursorClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}
	stream, err := c.StreamingCommitCursor(ctx)
	if err != nil {
		// TODO: Handle error.
	}
	go func() {
		reqs := []*pubsublitepb.StreamingCommitCursorRequest{
			// TODO: Create requests.
		}
		for _, req := range reqs {
			if err := stream.Send(req); err != nil {
				// TODO: Handle error.
			}
		}
		stream.CloseSend()
	}()
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			// TODO: handle error.
		}
		// TODO: Use resp.
		_ = resp
	}
}

func ExampleCursorClient_CommitCursor() {
	// import pubsublitepb "google.golang.org/genproto/googleapis/cloud/pubsublite/v1"

	ctx := context.Background()
	c, err := pubsublite.NewCursorClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &pubsublitepb.CommitCursorRequest{
		// TODO: Fill request struct fields.
	}
	resp, err := c.CommitCursor(ctx, req)
	if err != nil {
		// TODO: Handle error.
	}
	// TODO: Use resp.
	_ = resp
}

func ExampleCursorClient_ListPartitionCursors() {
	// import pubsublitepb "google.golang.org/genproto/googleapis/cloud/pubsublite/v1"
	// import "google.golang.org/api/iterator"

	ctx := context.Background()
	c, err := pubsublite.NewCursorClient(ctx)
	if err != nil {
		// TODO: Handle error.
	}

	req := &pubsublitepb.ListPartitionCursorsRequest{
		// TODO: Fill request struct fields.
	}
	it := c.ListPartitionCursors(ctx, req)
	for {
		resp, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			// TODO: Handle error.
		}
		// TODO: Use resp.
		_ = resp
	}
}
