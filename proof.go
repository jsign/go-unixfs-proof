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

func ValidateProof(ctx context.Context, root cid.Cid, offset uint64, proof []byte) (bool, error) {
	fmt.Printf("Validating %s\n", root)
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
	regenProof, err := CreateProof(ctx, root, offset, dserv)
	if err != nil {
		return false, fmt.Errorf("regenerating proof to validate: %s", err)
	}
	if !bytes.Equal(proof, regenProof) {
		return false, fmt.Errorf("proof is invalid")
	}

	return true, nil
}

func CreateProof(ctx context.Context, root cid.Cid, offset uint64, dserv ipld.DAGService) ([]byte, error) {
	n, err := dserv.Get(ctx, root)
	if err != nil {
		return nil, fmt.Errorf("get %s from dag service: %s", root, err)
	}
	list := []ipld.Node{n}

	var currOffset, layer uint64
	for n != nil {
		fmt.Printf("In layer %d, numLinks=%d\n", layer, len(n.Links()))
		var next ipld.Node
		for _, child := range n.Links() {
			cn, err := child.GetNode(ctx, dserv)
			if err != nil {
				return nil, fmt.Errorf("get child from layer %d: %s", layer, err)
			}
			list = append(list, cn)

			fmt.Printf("\t%s currOffset=%d\n", child.Cid, currOffset)
			if next == nil {
				fsNode, err := unixfs.ExtractFSNode(n)
				if err != nil {
					return nil, fmt.Errorf("extracting fsnode from merkle-dag: %s", err)
				}
				if currOffset+fsNode.FileSize() >= offset {
					next = cn
				} else {
					currOffset += fsNode.FileSize()
				}
			}
		}
		layer++
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
	for i := 0; i < len(list); i++ {
		n := list[i]
		if !seen.Visit(n.Cid()) {
			continue
		}
		if err := util.LdWrite(&buf, n.Cid().Bytes(), n.RawData()); err != nil {
			return nil, fmt.Errorf("encoding car block: %s", err)
		}
	}

	return buf.Bytes(), nil
}
