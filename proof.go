package unixfsproof

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"

	bsrv "github.com/ipfs/go-blockservice"
	"github.com/ipfs/go-cid"
	"github.com/ipfs/go-datastore"
	dssync "github.com/ipfs/go-datastore/sync"
	blockstore "github.com/ipfs/go-ipfs-blockstore"
	offline "github.com/ipfs/go-ipfs-exchange-offline"
	ipld "github.com/ipfs/go-ipld-format"
	dag "github.com/ipfs/go-merkledag"
	"github.com/ipfs/go-unixfs"
	car "github.com/ipld/go-car"
	"github.com/ipld/go-car/util"
)

// ValidateProof validates a proof for a Cid at a specified offset. If the proof is valid, the return
// parameter is true. If the proof is invalid false is returned. In any other case an error is returned.
func ValidateProof(ctx context.Context, root cid.Cid, offset uint64, proof []byte) (bool, error) {
	r := bufio.NewReader(bytes.NewReader(proof))

	bstore := blockstore.NewBlockstore(dssync.MutexWrap(datastore.NewMapDatastore()))
	cr, err := car.NewCarReader(r)
	if err != nil {
		return false, fmt.Errorf("invalid carv1: %s", err)
	}
	if len(cr.Header.Roots) != 1 {
		return false, fmt.Errorf("root list should have exactly one element")
	}
	if cr.Header.Roots[0] != root {
		return false, fmt.Errorf("the root isn't the expected one")
	}

	// TODO(jsign): if we assume some ordering in the CAR file we could simply have a CAR-serial walker
	//              which would make this much faster and probably simpler in a way avoiding blockstores, etc.
	//              For now, not have those assumptions and do a naive-ish walk.
	for {
		block, err := cr.Next()
		if err == io.EOF {
			break
		}
		if err := bstore.Put(block); err != nil {
			return false, fmt.Errorf("adding block to blockstore: %s", err)
		}
	}
	dserv := dag.NewDAGService(bsrv.New(bstore, offline.Exchange(bstore)))

	// Smart (or lazy?) way to verify the proof. If we assume an ideal full DAGStore, trying to
	// re-create the proof with the proof as the underlying blockstore should fail if something
	// is missing.
	regenProof, err := CreateProof(ctx, root, offset, dserv)
	if err != nil {
		return false, nil
	}

	return bytes.Equal(proof, regenProof), nil
}

// CreateProof creates a proof for a Cid at a specified file offset.
func CreateProof(ctx context.Context, root cid.Cid, offset uint64, dserv ipld.DAGService) ([]byte, error) {
	n, err := dserv.Get(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("get %s from dag service: %s", root, err)
	}
	proofNodes := []ipld.Node{n}

	var currOffset uint64
	for n != nil {
		var next ipld.Node
		for _, child := range n.Links() {
			cn, err := child.GetNode(ctx, dserv)
			if err != nil {
				return nil, fmt.Errorf("get child %s: %s", child.Cid, err)
			}
			proofNodes = append(proofNodes, cn)

			// REMOVE THIS?
			if next == nil {
				fsNode, err := unixfs.ExtractFSNode(n)
				if err != nil {
					return nil, fmt.Errorf("extracting fsnode from merkle-dag: %s", err)
				}
				if currOffset+fsNode.FileSize() >= offset {
					next = cn
					break
				} else {
					currOffset += fsNode.FileSize()
				}
			}
		}
		n = next
	}

	var buf bytes.Buffer
	h := &car.CarHeader{
		Roots:   []cid.Cid{root},
		Version: 1,
	}
	if err := car.WriteHeader(h, &buf); err != nil {
		return nil, fmt.Errorf("writing car header: %s", err)
	}
	seen := cid.NewSet()
	for i := 0; i < len(proofNodes); i++ {
		n := proofNodes[i]
		if !seen.Visit(n.Cid()) {
			continue
		}
		if err := util.LdWrite(&buf, n.Cid().Bytes(), n.RawData()); err != nil {
			return nil, fmt.Errorf("encoding car block: %s", err)
		}
	}

	return buf.Bytes(), nil
}
