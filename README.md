# go-unixfs-proof

Go implementation of offset-based native UnixFS proofs.

**Note:** this is a side-project and not be considered production-ready. It isn't optimized nor audited in any way. 

## Table of contents
- [About the project](#about-the-project)
- [Does this library assume any particular setup of the UnixFS DAG for the file?](#does-this-library-assume-any-particular-setup-of-the-UnixFS-DAG-for-the-file)
- [Proof format](#proof-format)
- [Use-case analysis and security](#use-case-analysis-and-security)
- [Proof sizes and benchmark](#proof-sizes-and-benchmark)
- [CLI](#cli)
- [Roadmap](#roadmap)
- [Contributing](#contributing)
- [License](#license)
- [Contact](#contact)


## About the project
This library allows generating and verification proofs for UnixFS file DAGs.

The verifier knows the _Cid_ of a UnixFS DAG and the size of the underlying represented file. 
With this information, the verifier asks the prover to generate a proof that it stores the block at a specified offset between _[0, max-file-size]_.

The proof is a sub-DAG which contains all the necessary blocks to assert that:
- The provided block is part of the DAG with the expected Cid root.
- The provided block of data is at the specified offset in the file.

The primary motivation for this kind of library is to provide a way to make challenges at a random-sampled offset of the original file to have a probabilistic guarantee that the prover is storing the data.

Consider the following UnixFS DAG file with a fanout factor of 3:
![image](https://user-images.githubusercontent.com/6136245/139512869-5135649f-dc34-4ef1-9862-5c47860ec581.png)
<!---
(https://excalidraw.com/#json=5662906028916736,qzS2x9JgfY30Vy2tbzWwiA)
-->


Considering a verifier is asking a prover to provide a proof that it contains the corresponding block at the _file level offset_ X, the prover generates the subdag inside the green zone:
- Round nodes are internal DAG nodes that are somewhat small-ish and don't contain file data.
- Square nodes contain chunks of the original file data.
- The indigo-colored nodes are necessary nodes to verify that the target block (red) is at the specified offset.

To understand better more details about this proof, read the _Proof sizes and benchmark_ section.

## Does this library assume any particular setup of the UnixFS DAG for the file?
No, this library works with any DAG layout, so it doesn't have any particular assumptions.
The DAG can have different layouts (e.g., balanced, trickle, etc.), chunking (e.g., fixed size, etc.), or other particular DAG builder configurations.

This minimum level of assumptions allows the challenger to only needed to know the _Cid_ and file size to ask and verify the proof.
There's an inherent tradeoff between assumptions and possible optimizations of the proof. See _Proof size and benchmark_ section.

## Proof format
To avoid inventing any new proof standard or format, the proof is a byte array corresponding to a CAR file format of all the blocks that are part of the proof.
The decision was mainly to avoid friction about defining a new format or standard. 

The order of blocks in the CAR file should be considered undefined despite the current implementation having a BFS order.
Defining a particular order can improve the proof verification, so that's a possible change that can be done.

## Use-case analysis and security
The primary motivation is to support a random-sampling-based challenge system between a prover and a verifier.

Given a file with size _MaxSize_, the verifying can ask the prover to generate proof with the underlying block for a specified _Cid_.

The security of this schema is similar to other random-sampling schemas:
- If the underlying prover doesn't have the block, it won't generate the proof.
- If the offset is random-sampled in the _[0, MaxSize]_ range, it can't be guessed by the prover without storing all the file.

If the bad-prover is storing  only part of the leaves _p_ (e.g., 50%):
- A single challenge makes the prover have a probability `p` (e.g., 50%) of success.
- If the challenger asks for N (e.g., 5) proofs, the probability of generating all correct proofs is `p^N` (e.g., 3%) at the cost of a proof size of ~`SingleProofSize*N`.

Despite the above, if the prover deletes only 1 byte of the data, it would still generate proofs with ~high chance. Still, the file could be considered corrupted since a single byte is usually enough to make the file unavailable.

One possible approach can be inspired from work by Mustafa et al. for data-availability schemas (see [here](https://ethresear.ch/t/simulating-a-fraud-proof-blockchain/5024)).
If an erasure-code schema was applied to the data, this forces the prover to drop a significant amount of data to make the file unrecoverable. For example, if the erasure code has a 2x leverage, the miner should drop at least 50% of the file to make it unrecoverable. As shown before, dropping 50% of the data means it has 3% success if asked for 5 proofs. This means that if the file is in an unrecoverable state, with 5 proofs, we should detect this at least 97% of the time.

Notice that if the prover has missing internal nodes of the UnixFS, then the impact of a missed block is much higher than missing leaves (underlying data) since the probability of hitting an internal node is way bigger than leaves for a random offset. (e.g., if the root Cid block is missing, all challenges will fail). This means that the probability of the prover failing to provide the proofs is lower than the analysis made above for leaves.


## Proof sizes and benchmark
The proof size is directly related to how many assumptions we have about the underlying DAG structure. The current implementation of this library doesn't assume anything about the DAG structure, so it isn't optimized for proof size.
The biggest weight in the proofs comes from leave blocks which are usually heavy (~100s of KB), and depending on where an offest lands on the DAG structure, it could contain multiple data-blocks.

If at least we can bake an assumption about constant size chunks with defined size, we could generate mostly minimal and constant sized proofs since we could probably avoid all leaves and only include the targeted one. Maybe the library can be extended to allow baking assumptions like this and generate smaller proofs in the future.

The cost of generating the proofs should be _O(1)_. Probably soon, I'll add some benchmarks, but realistically speaking is mainly tied to how fast lookups can be done in the `DAGService`, which mainly depends on the source of the data, not the algorithm.

## CLI
A simple CLI `ufsproof` is provided, allowing easy to generate and verify proofs, which can be installed running `make install`.

To generate proofs, run `ufsproof prove [cid] [offset]`, which prints in stdout the proof for block of Cid at the provided offset.
For example:
- `ufsproof prove QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300`: assumes that the Cid is stored in an IPFS API at `/ip4/127.0.0.1/tcp/5001`.
- `ufsproof prove QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300 > proof.car`: stores the proof in a file.
- `ufsproof prove --car-file mydag.car QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300`: uses a CAR file instead of an IPFS API.

To verify proofs, run `ufsproof verify [cid] [offset] [proof-path:(optional, by default stdin)]`.
For example:
- `ufsproof verify QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300 proof.car`


Closing the loop:
```
$ ufsproof prove QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300 | ufsproof verify QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 1300
The proof is valid
$ ufsproof prove QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 10 | ufsproof verify QmUavJLgtkQy6wW2j1J1A5cAP6UQt3XLQjsArsU2ZYmgSo 50000000
The proof is NOT valid
```
Remember that because of (**) mentioned in _Proof sizes and benchmark_ is possible to have a valid proof message on some offsets greater than the proved one.

## Roadmap
Possible ideas in the near future:
- [ ] Allow direct leaf Cid proof (non-offset based); a bit offtopic for this lib and not sure entirely useful.
- [ ] Benchmarks, may be fun but nothing entirely useful for now.
- [ ] Allow strict mode proof validation; maybe it makes sense to fail faster in some cases, nbd.
- [ ] CLI for validation from DealID in Filecoin network; maybe fun, but `Labels` are unverified.
- Baking assumptions for shorter proofs.
- [ ] godocs

This is a side-project made for fun, so a priori is a hand-wavy roadmap.

## Contributing

Contributions make the open source community such an amazing place to learn, inspire, and create. Any contributions you make are **greatly appreciated**.

If you like to donate, please send funds to [this project wallet-address](https://etherscan.io/address/0x2750E75E3771Dfb5041D5014a3dCC6e052fcd575). Any received funds will be directed to other open-source projects or organizations.

## License

Distributed under the MIT License. See `LICENSE` for more information.

## Contact
Ignacio Hagopian - [@jsign](https://github.com/jsign) 
