package btree_dao

import (
	"std"
	"strings"
	"testing"
	"time"

	"gno.land/p/demo/btree"
	"gno.land/p/demo/uassert"
	"gno.land/p/demo/urequire"
)

func setupTest() {
	testing.SetOriginCaller(std.Address("g1ej0qca5ptsw9kfr64ey8jvfy9eacga6mpj2z0y"))
	members = btree.New()
}

type TestElement struct {
	value int
}

func (te *TestElement) Less(than btree.Record) bool {
	return te.value < than.(*TestElement).value
}

func TestPlantTree(t *testing.T) {
	setupTest()

	tree := btree.New()
	elements := []int{30, 10, 50, 20, 40}
	for _, val := range elements {
		tree.Insert(&TestElement{value: val})
	}

	err := PlantTree(tree)
	urequire.NoError(t, err)

	found := false
	members.Ascend(func(record btree.Record) bool {
		regDetails := record.(*RegistrationDetails)
		if regDetails.UserBTree == tree {
			found = true
			return false
		}
		return true
	})
	uassert.True(t, found)

	err = PlantTree(tree)
	uassert.Error(t, err)

	emptyTree := btree.New()
	err = PlantTree(emptyTree)
	uassert.Error(t, err)
}

func TestPlantSeed(t *testing.T) {
	setupTest()

	err := PlantSeed("Hello DAO!")
	urequire.NoError(t, err)

	found := false
	members.Ascend(func(record btree.Record) bool {
		regDetails := record.(*RegistrationDetails)
		if regDetails.UserBTree == nil {
			found = true
			uassert.NotEmpty(t, regDetails.NFTID)
			uassert.True(t, strings.Contains(regDetails.NFTID, "seed_"))
			return false
		}
		return true
	})
	uassert.True(t, found)

	err = PlantSeed("")
	uassert.Error(t, err)
}

func TestRegistrationDetailsOrdering(t *testing.T) {
	setupTest()

	rd1 := &RegistrationDetails{
		Address: std.Address("test1"),
		RegTime: time.Now(),
		NFTID:   "0",
	}
	rd2 := &RegistrationDetails{
		Address: std.Address("test2"),
		RegTime: time.Now().Add(time.Hour),
		NFTID:   "1",
	}

	uassert.True(t, rd1.Less(rd2))
	uassert.False(t, rd2.Less(rd1))
}
