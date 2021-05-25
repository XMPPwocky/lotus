package kit

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/filecoin-project/go-address"
	"github.com/filecoin-project/go-state-types/abi"
	lapi "github.com/filecoin-project/lotus/api"
	"github.com/filecoin-project/lotus/build"
	"github.com/filecoin-project/lotus/chain/wallet"
	sealing "github.com/filecoin-project/lotus/extern/storage-sealing"
	"github.com/filecoin-project/lotus/miner"
	libp2pcrypto "github.com/libp2p/go-libp2p-core/crypto"
	"github.com/libp2p/go-libp2p-core/peer"
	"github.com/multiformats/go-multiaddr"
	"github.com/stretchr/testify/require"
)

type TestMiner struct {
	lapi.StorageMiner

	t *testing.T

	// ListenAddr is the address on which an API server is listening, if an
	// API server is created for this Node
	ListenAddr multiaddr.Multiaddr

	ActorAddr address.Address
	OwnerKey  *wallet.Key
	MineOne   func(context.Context, miner.MineReq) error
	Stop      func(context.Context) error

	FullNode   *TestFullNode
	PresealDir string

	Libp2p struct {
		PeerID  peer.ID
		PrivKey libp2pcrypto.PrivKey
	}

	options NodeOpts
}

var MineNext = miner.MineReq{
	InjectNulls: 0,
	Done:        func(bool, abi.ChainEpoch, error) {},
}

func (tm *TestMiner) PledgeSectors(ctx context.Context, n, existing int, blockNotif <-chan struct{}) { //nolint:golint
	for i := 0; i < n; i++ {
		if i%3 == 0 && blockNotif != nil {
			<-blockNotif
			tm.t.Log("WAIT")
		}
		tm.t.Logf("PLEDGING %d", i)
		_, err := tm.PledgeSector(ctx)
		require.NoError(tm.t, err)
	}

	for {
		s, err := tm.SectorsList(ctx) // Note - the test builder doesn't import genesis sectors into FSM
		require.NoError(tm.t, err)
		fmt.Printf("Sectors: %d\n", len(s))
		if len(s) >= n+existing {
			break
		}

		build.Clock.Sleep(100 * time.Millisecond)
	}

	fmt.Printf("All sectors is fsm\n")

	s, err := tm.SectorsList(ctx)
	require.NoError(tm.t, err)

	toCheck := map[abi.SectorNumber]struct{}{}
	for _, number := range s {
		toCheck[number] = struct{}{}
	}

	for len(toCheck) > 0 {
		for n := range toCheck {
			st, err := tm.SectorsStatus(ctx, n, false)
			require.NoError(tm.t, err)
			if st.State == lapi.SectorState(sealing.Proving) {
				delete(toCheck, n)
			}
			require.NotContains(tm.t, string(st.State), "Fail", "sector in a failed state")
		}

		build.Clock.Sleep(100 * time.Millisecond)
		fmt.Printf("WaitSeal: %d\n", len(s))
	}
}