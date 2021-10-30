package unixfsproof

import (
	"bytes"
	"context"
	"io"
	"math/rand"
	"testing"

	chunker "github.com/ipfs/go-ipfs-chunker"
	"github.com/ipfs/go-unixfs/importer/balanced"
	h "github.com/ipfs/go-unixfs/importer/helpers"
	testu "github.com/ipfs/go-unixfs/test"
	"github.com/stretchr/testify/require"
)

func TestMakeProof(t *testing.T) {
	dserv := testu.GetDAGServ()

	r := rand.New(rand.NewSource(22))
	data := make([]byte, 100000)
	_, err := io.ReadFull(r, data)
	require.NoError(t, err)

	in := bytes.NewReader(data)
	opts := testu.UseCidV1
	dbp := h.DagBuilderParams{
		Dagserv:    dserv,
		Maxlinks:   3,
		CidBuilder: opts.Prefix,
		RawLeaves:  opts.RawLeavesUsed,
	}

	db, err := dbp.New(chunker.NewSizeSplitter(in, 256))
	require.NoError(t, err)
	node, err := balanced.Layout(db)
	require.NoError(t, err)

	offset := uint64(50000)
	ctx, cls := context.WithCancel(context.Background())
	defer cls()
	proof, err := CreateProof(ctx, node.Cid(), offset, dserv)
	require.NoError(t, err)

	ok, err := ValidateProof(ctx, node.Cid(), offset, proof)
	require.NoError(t, err)
	require.True(t, ok)
}
